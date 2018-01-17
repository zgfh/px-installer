package utils

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
)

const clientAPIDefaultVersion = "1.23"

var (
	dockerDownloadMsg = []byte(`"Pulling fs layer"`)
	pxNeedsReboot = []byte("WARNING: May require reboot to complete the Portworx upgrade")
)

// DockerInstaller is a Docker client specialized for Container installation
type DockerInstaller struct {
	auth string
	ctx  context.Context
	cli  *client.Client
}

// NewDockerInstaller creates an instance of the DockerInstaller
func NewDockerInstaller(user, pass string) (*DockerInstaller, error) {
	auth, ctx := "", context.Background()

	cliVer := os.Getenv("DOCKER_API_VERSION")
	if cliVer == "" {
		cliVer = clientAPIDefaultVersion
	}

	// NOTE: see https://docs.docker.com/engine/api/v1.26/#section/Versioning
	cli, err := client.NewClient("unix:///var/run/docker.sock", cliVer, nil, nil)
	if err != nil {
		return nil, err
	}

	cli.NegotiateAPIVersion(ctx)

	if user != "" {
		js := fmt.Sprintf(`{"username":%q,"password":%q}`, user, pass)
		auth = base64.StdEncoding.EncodeToString([]byte(js))
	}
	return &DockerInstaller{
		auth: auth,
		ctx:  ctx,
		cli:  cli,
	}, nil
}

// PullImage pulls the image of a given name
func (di *DockerInstaller) PullImage(name string) error {
	opts := types.ImagePullOptions{RegistryAuth: di.auth}
	out, err := di.cli.ImagePull(di.ctx, name, opts)
	if err != nil {
		return err
	}
	_, err = io.Copy(os.Stdout, out)
	return err
}

// DownloadNotifyCbFunc is used in conjunction with PullImageCb, to provide callback when "image pull" is downloading
// the content (as opposed to {"status":"Status: Image is up to date for portworx/px-base:338f20e"})
type DownloadNotifyCbFunc func() error

// PullImageCb pulls the image of a given name. The CallBack function is called if image does not exist, and is being downloaded.
func (di *DockerInstaller) PullImageCb(name string, cb DownloadNotifyCbFunc) error {
	opts := types.ImagePullOptions{RegistryAuth: di.auth}
	out, err := di.cli.ImagePull(di.ctx, name, opts)
	if err != nil {
		return err
	}

	buf := make([]byte, 512)
	if n, err := io.ReadFull(out, buf); err != nil {
		if err == io.ErrUnexpectedEOF {
			// this is an OK condition (incomplete read), copy the bytes we've got and exit
			_, err = os.Stdout.Write(buf[0:n])
			return err
		}
		return err
	}
	// based on initial read, let's determine if we started downloading layers
	if cb != nil && bytes.Contains(buf, dockerDownloadMsg) {
		if err := cb(); err != nil {
			return err
		}
	}
	// flush initial content, and continue...
	os.Stdout.Write(buf)

	// Keep on copying w/ 512b buffer, so we'll see some output (dfl. is 32k buffer)
	_, err = io.CopyBuffer(os.Stdout, out, buf)
	return err
}

// GetImageID inspects the image of a given name, and returns the image ID
func (di *DockerInstaller) GetImageID(name string) (string, error) {
	out, _, err := di.cli.ImageInspectWithRaw(di.ctx, name)
	if err != nil {
		return "", err
	}
	return out.ID, nil
}

// dockerLogsReader is a simplified version of stdcopy.StdCopy docker logs stream de-multiplexer.
// See also: https://stackoverflow.com/questions/46428721/how-to-stream-docker-container-logs-via-the-go-sdk
func dockerLogReader(dockLog io.ReadCloser, writers ...io.Writer) error {
	if len(writers) <= 0 {
		return fmt.Errorf("No writers specified")
	}

	hdrLen := 8
	hdr, buf := make([]byte, hdrLen), make([]byte, 32*1024+2)

	for {
		// get header
		rc, err := dockLog.Read(hdr)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("Could not read docker log header: %s", err)
		} else if rc != 8 {
			return fmt.Errorf("Short write (hdr)")
		}

		// get frame length, and increase buffer (if required)
		frameLenAndOffs := binary.BigEndian.Uint32(hdr[4:]) + 2
		if uint32(len(buf)) < frameLenAndOffs {
			logrus.Debugf("dockerLogsReader - reallocating bufsiz %d to %d", len(buf), frameLenAndOffs)
			buf = make([]byte, frameLenAndOffs)
		}

		switch hdr[0] {
		case 1: // os.Stdout, prefix w/ "> "
			buf[0] = '>'
		default: // os.Stderr, prefix w/ "E "
			buf[0] = 'E'
		}
		buf[1] = ' '
		rc, err = dockLog.Read(buf[2:frameLenAndOffs])
		if err != nil {
			return fmt.Errorf("Could not read docker log data: %s", err)
		}

		for _, w := range writers {
			w.Write(buf[:rc+2])
		}
	}
}

// RunOnce will create container, run it, wait until it's finished, and finally remove it.
func (di *DockerInstaller) RunOnce(name, cntr string, binds, entrypoint, args []string) error {
	contConf := container.Config{
		Image:        name,
		Cmd:          args,
		AttachStdin:  false,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          false,
	}

	if len(entrypoint) > 0 {
		logrus.Infof("Overriding entrypoint with %v", entrypoint)
		contConf.Entrypoint = entrypoint
	}

	hostConf := container.HostConfig{
		Binds:      binds,
		AutoRemove: false,
		Privileged: true,
	}

	logrus.Infof("Removing old container %s (if any)", cntr)
	err := di.cli.ContainerRemove(di.ctx, cntr, types.ContainerRemoveOptions{
		RemoveVolumes: true,
		Force:         true,
	})
	logrus.WithError(err).Debug("Old container removed")

	logrus.Info("Creating container from image ", name)
	logrus.Debugf("> CONF: %+v  /  HOST: %+v", contConf, hostConf)
	resp, err := di.cli.ContainerCreate(di.ctx, &contConf, &hostConf, nil, cntr)
	if err != nil {
		return fmt.Errorf("Could not create container %s: %s", name, err)
	}

	logrus.Infof("Starting container %s [%s]", resp.ID, name)
	if err := di.cli.ContainerStart(di.ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("Could not start container %s [%s]: %s", resp.ID, name, err)
	}

	// after this point, we want to always dump the logs, and remove container only in all OK
	var retError error

	// REFS:
	// - https://docs.docker.com/engine/api/v1.24/#get-container-logs
	// - curl --unix-socket /var/run/docker.sock 'http:/containers/9908c502895c/logs?stdout=1&stderr=true&tail=true&timestamps=true'
	logrus.Infof("Logs for container %s [%s]", resp.ID, name)
	out, err := di.cli.ContainerLogs(di.ctx, resp.ID, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: true,
		Follow:     true,
	})
	if err != nil {
		// let's make it non-fatal (will check ContainerWait() instead)
		logrus.WithError(err).Errorf("Could not get logs for container %s [%s]: %s", resp.ID, name, err)
	} else {
		var b bytes.Buffer
		// note, can cancel goroutine by cancelling the context for ContainerLogs() (closes reader)
		err = dockerLogReader(out, &b, os.Stdout)
		if err != nil {
			logrus.WithError(err).Warnf("Could not get complete px-oci-installer log")
		} else if bytes.Contains(b.Bytes(), pxNeedsReboot) {
			logrus.Warn("TODO: Cordon the node's containers")
		} else {
			logrus.Info("PX module OK")
		}
	}

	logrus.Infof("Waiting for container %s [%s]", resp.ID, name)
	resultC, errC := di.cli.ContainerWait(di.ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case result := <-resultC:
		if result.StatusCode != 0 {
			retError = fmt.Errorf("Expected status code '0', got %d", result.StatusCode)
		}
	case err := <-errC:
		retError = fmt.Errorf("Error while running container %s [%s]: %s", resp.ID, name, err)
	}

	// CHECKME: Not removing the container, not to provoke the fsync, also to keep the PX-image
	// > di.cli.ContainerRemove(di.ctx, resp.ID, types.ContainerRemoveOptions{RemoveVolumes: true, Force: true })

	logrus.Warnf("NOTE: Not removing the %s container [%s]", resp.ID, name)

	return retError
}

// ExtractConfig extracts the containers configuration
func (di *DockerInstaller) ExtractConfig(id string) (*SimpleContainerConfig, error) {
	scc := SimpleContainerConfig{}

	cconf, err := di.cli.ContainerInspect(di.ctx, id)
	if err != nil {
		return nil, fmt.Errorf("Error inspecting container '%s': %s", id, err)
	}
	logrus.Debugf("CONFIG:%+v", cconf)

	// Copy arguments
	scc.Args = make([]string, len(cconf.Args))
	copy(scc.Args, cconf.Args)

	// Copy mounts
	scc.Mounts = formatMounts(cconf)

	// Copy ENV
	scc.Env = make([]string, len(cconf.Config.Env))
	copy(scc.Env, cconf.Config.Env)

	// Copy LABELS
	scc.Labels = make(map[string]string)
	for k, v := range cconf.Config.Labels {
		scc.Labels[k] = v
	}

	return &scc, nil
}
