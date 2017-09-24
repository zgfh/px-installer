package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/portworx/px-installer/px-oci-mon/utils"
)

const (
	ociInstallerImage  = "zoxpx/px-enterprise-oci:1.2.11-rc3" // TODO: "portworx/px-base-enterprise-oci:latest"
	ociInstallerName   = "px-oci-installer"
	mntFileName        = "/host_proc/1/ns/mnt"
	dockerFileSockName = "/var/run/docker.sock"
	baseDir            = "/opt/pwx/oci"
	baseServiceName    = "portworx"
	baseServiceFileFmt = "/etc/systemd/system/%s.service"
)

// usage borrowed from ../../porx/cmd/px-runc/px-runc.go -- TODO: Consider refactoring !!
func usage(args ...interface{}) {
	if len(args) > 0 {
		logrus.Error(args...)
		fmt.Fprintln(os.Stderr)
	}

	fmt.Printf(`Usage: %[1]s <install|uninstall> [options]

options:
   -oci <dir>                Specify OCI directory (dfl: %[2]s)
   -name <name>              Specify container/service name (dfl: %[3]s)
   -sysd <file>              Specify SystemD service file (dfl: %[4]s)
   -v <dir:dir[:shared,ro]>  Specify extra mounts
   -c                        [REQUIRED] Specifies the cluster ID that this PX instance is to join
   -k                        [REQUIRED] Points to your key value database, such as an etcd cluster or a consul cluster
   -s                        [OPTIONAL if -a is used] Specifies the various drives that PX should use for storing the data
   -d <ethX>                 Specify the data network interface
   -m <ethX>                 Specify the management network interface
   -z                        Instructs PX to run in zero storage mode
   -f                        Instructs PX to use an unmounted drive even if it has a filesystem on it
   -a                        Instructs PX to use any available, unused and unmounted drives
   -A                        Instructs PX to use any available, unused and unmounted drives or partitions
   -x <swarm|kubernetes>     Specify scheduler being used in the environment
   -token <token>            Portworx lighthouse token for cluster

kvdb-options:
   -userpwd <user:passwd>    Username and password for ETCD authentication
   -ca <file>                Specify location of CA file for ETCD authentication
   -cert <file>              Specify locationof certificate for ETCD authentication
   -key <file>               Specify location of certificate key for ETCD authentication
   -acltoken <token>         ACL token value used for Consul authentication

examples:
   %[1]s install -k etcd://70.0.1.65:2379 -c MY_CLUSTER_ID -s /dev/sdc -d enp0s8 -m enp0s8

`, os.Args[0], baseDir, baseServiceName, fmt.Sprintf(baseServiceFileFmt, baseServiceName))
	os.Exit(1)
}

func runExternal(name string, params ...string) error {
	args := make([]string, 0, 4+len(params))
	args = append(args, "/usr/bin/nsenter", "--mount="+mntFileName, "--", name)
	args = append(args, params...)

	logrus.Info("> run: ", strings.Join(args[3:], " "))
	logrus.Debugf(">>> %+v", args)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

func installPxFromOciImage(di *utils.DockerInstaller, imageName string, cfg *utils.SimpleContainerConfig) error {
	logrus.Info("Downloading Portworx...")

	err := di.PullImage(imageName)
	if err != nil {
		logrus.WithError(err).Error("Could not pull ", imageName)
		usage("Could not pull " + imageName +
			" - have you specified REGISTRY_USER/REGISTRY_PASS env. variables?")
	}

	// NOTE: This step is required, if px-runcds does not mount pwx-dirs
	err = runExternal("/bin/mkdir", "-p", "/opt/pwx", "/etc/pwx")
	if err != nil {
		logrus.WithError(err).Warn("Unable to create pwx directories directly -- retry via shell")
		err = runExternal("/bin/sh", "-c", "mkdir -p /opt/pwx /etc/pwx")
		if err != nil {
			return fmt.Errorf("Unable to create pwx directories: %s", err)
		}
	}

	err = di.RunOnce(imageName, ociInstallerName,
		[]string{"--upgrade-inplace"}, []string{"/opt/pwx", "/etc/pwx"})
	if err != nil {
		logrus.WithError(err).Error("Could not install ", imageName)
		usage("Could not install " + imageName +
			" - please inspect docker's log, and contact Portworx support.")
	}

	logrus.Info("Installing Portworx...")

	// Compose startup-line for PX-RunC
	args := make([]string, 0, 1+len(cfg.Args)+len(cfg.Env)*2+len(cfg.Mounts)*2)
	args = append(args, "/opt/pwx/bin/px-runc", "install")
	args = append(args, cfg.Args[1:]...)

	// Add Mounts
	for _, vol := range cfg.Mounts {
		// skip 2 internal mounts, pass the others
		if strings.Contains(vol, ":/host_proc/1/ns") || strings.HasSuffix(vol, dockerFileSockName) {
			continue
		}
		args = append(args, "-v", vol)
	}

	// Add Environment
	for _, env := range cfg.Env {
		args = append(args, "-e", env)
	}

	// TODO: Add Labels?

	err = runExternal(args[0], args[1:]...)
	if err != nil {
		logrus.WithError(err).Error("Could not install PX-RunC")
	}

	return err
}

func validateMounts(mounts ...string) error {
	var st0, st1 syscall.Stat_t

	if err := syscall.Lstat("/", &st0); err != nil {
		// improbable, but let's handle it
		return fmt.Errorf("INTERNAL ERROR - could not stat '/': %s", err)
	}
	for _, m := range mounts {
		err := syscall.Lstat(m, &st1)
		if err != nil {
			return fmt.Errorf("File/Directory %s not found (%s) - please mount via 'run -v ...' option", m, err)
		} else if st0.Dev == st1.Dev {
			return fmt.Errorf("File/Directory %s not mounted - please mount via 'run -v ...' option", m, err)
		}
	}
	return nil
}

func doInstall() {
	pxImage := os.Getenv("PX_IMAGE")
	if pxImage == "" {
		pxImage = ociInstallerImage
	}

	err := validateMounts(mntFileName, dockerFileSockName)
	if err != nil {
		usage(err)
	}

	id, err := utils.GetMyContainerID()
	if err != nil {
		logrus.WithError(err).Error("Could not determine my container ID" +
			" - are you running me inside Docker?")
		os.Exit(-1)
	}

	di, err := utils.NewDockerInstaller(os.Getenv("REGISTRY_USER"), os.Getenv("REGISTRY_PASS"))
	if err != nil {
		logrus.WithError(err).Error("Could not 'talk' to Docker")
		usage("Could not 'talk' to Docker" +
				" - please restart using '-v /var/run/docker.sock:/var/run/docker.sock' option")
	}

	opts, err := di.ExtractConfig(id)
	if err != nil {
		logrus.WithError(err).Error("Could not extract my container's configuration" +
			" - are you running me inside Docker?")
		os.Exit(-1)
	}

	// TODO: Sanity checks for options
	logrus.Debugf("OPTIONS:: %+v\n", opts)

	err = installPxFromOciImage(di, pxImage, opts)
	if err != nil {
		logrus.WithError(err).Error("Could not install Portworx service")
		os.Exit(-1)
	}

	logrus.Info("Stopping Portworx service (if any)")
	err = runExternal("/bin/sh", "-c", "systemctl stop portworx")
	logrus.WithError(err).Debugf("Stopping done")

	logrus.Info("Starting Portworx service")
	err = runExternal("/bin/sh", "-c",
		`systemctl daemon-reload && systemctl enable portworx && systemctl start portworx`)
	if err != nil {
		logrus.WithError(err).Error("Could not start Portworx service")
		os.Exit(-1)
	}
}

func doUninstall() {
	logrus.Info("Stopping Portworx service")
	err := runExternal("/bin/sh", "-c",
		`systemctl stop portworx`)
	if err != nil {
		logrus.WithError(err).Error("Could not stop Portworx service")
		os.Exit(-1) // NOTE: CRITICAL failure !!
	}

	logrus.Info("Disabling Portworx service")
	err = runExternal("/bin/sh", "-c",
		`systemctl disable portworx`)
	if err != nil {
		logrus.WithError(err).Warn("Could not disable Portworx service (continuing)")
	}

	logrus.Info("Removing Portworx service bind-mount (if any)")
	err = runExternal("/bin/sh", "-c",
		fmt.Sprintf(`grep -q ' %[1]s %[1]s ' /proc/self/mountinfo && umount %[1]s`, "/opt/pwx/oci"))
	if err != nil {
		logrus.WithError(err).Warn("Could not bind-umount Portworx files (continuing)")
	}

	logrus.Info("Removing Portworx files")
	err = runExternal("/bin/rm", "-fr", "/opt/pwx", "/etc/systemd/system/portworx.service")
	if err != nil {
		logrus.WithError(err).Warn("Could not remove all Portworx files")
	}
}

func main() {
	if len(os.Args) < 2 || (os.Args[1] != "install" && os.Args[1] != "uninstall") {
		usage("First argument must be <install|uninstall>")
	}

	if (len(os.Args) > 2 && os.Args[2] == "--debug") || os.Getenv("DEBUG") != "" {
		logrus.SetLevel(logrus.DebugLevel)
	}

	switch os.Args[1] {
	case "install":
		doInstall()
	case "uninstall":
		doUninstall()
	default:
		usage("Command " + os.Args[1] + " not supported")
	}

	// CHECKME: Should we always go to sleep, or can we also exit
	logrus.Infof("%s done - going to sleep", strings.Title(os.Args[1]))
	err := syscall.Pause()
	logrus.WithError(err).Error("Could not pause")
}
