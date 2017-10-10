package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/portworx/px-installer/px-oci-mon/utils"
)

const (
	ociInstallerImage  = "portworx/px-enterprise:1.2.11-rc5" // TODO: "portworx/px-base-enterprise-oci:latest"
	ociInstallerName   = "px-oci-installer"
	mntFileName        = "/host_proc/1/ns/mnt"
	dockerFileSockName = "/var/run/docker.sock"
	baseDir            = "/opt/pwx/oci"
	baseServiceName    = "portworx"
	baseServiceFileFmt = "/etc/systemd/system/%s.service"
	pxConfigFile       = "/etc/pwx/config.json"
)

var (
	// rsyncRegex extracts '# files transferred' from rsync's stat/stat2 report
	rsyncRegex = regexp.MustCompilePOSIX(`Number of regular files transferred: ([0-9,]+)`)
	// xtractKubeletRegex extracts /var/kubelet -override from running kubelet daemon
	xtractKubeletRegex = regexp.MustCompile(`\s+--root-dir=(\S+)`)
	debugsOn = false
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

func runExternalWithOutput(out io.Writer, name string, params ...string) error {
	args := make([]string, 0, 4+len(params))
	args = append(args, "/usr/bin/nsenter", "--mount="+mntFileName, "--", name)
	args = append(args, params...)

	logrus.Info("> run: ", strings.Join(args[3:], " "))
	logrus.Debugf(">>> %+v", args)
	cmd := exec.Command(args[0], args[1:]...)
	if out == nil {
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	} else {
		// note: exec.CombinedOutput() assigns to bytes.buffer, like we do
		cmd.Stdout, cmd.Stderr = out, out
	}
	return cmd.Run()
}

func runExternal(name string, params ...string) error {
	return runExternalWithOutput(nil, name, params...)
}

// Output filters --

type regexFilter struct {
	re  *regexp.Regexp
	val []byte
	sync.Mutex
}

func (r *regexFilter) Write(b []byte) (int, error) {
	if m := r.re.FindAllSubmatch(b, -1); len(m) > 0 && len(m[0]) > 1 {
		ll := len(m[0][1])
		r.Lock()
		r.val = make([]byte, ll)
		copy(r.val, m[0][1])
		r.Unlock()
	}
	return os.Stderr.Write(b)
}

type cachingOutput struct {
	bb bytes.Buffer
}

func (c *cachingOutput) Write(b []byte) (int, error) {
	c.bb.Write(b)
	return os.Stderr.Write(b)
}

func (c *cachingOutput) String() string {
	return c.bb.String()
}

// -- Output Filters

func installPxFromOciImage(di *utils.DockerInstaller, imageName string, cfg *utils.SimpleContainerConfig) (bool, error) {
	logrus.Info("Downloading Portworx...")

	if err := di.PullImage(imageName); err != nil {
		logrus.WithError(err).Error("Could not pull ", imageName)
		usage("Could not pull " + imageName +
			" - have you specified REGISTRY_USER/REGISTRY_PASS env. variables?")
	}

	// NOTE: This step is required, if px-runcds does not mount pwx-dirs
	if err := runExternal("/bin/mkdir", "-p", "/opt/pwx", "/etc/pwx"); err != nil {
		logrus.WithError(err).Warn("Unable to create pwx directories directly -- retry via shell")
		err = runExternal("/bin/sh", "-c", "mkdir -p /opt/pwx /etc/pwx")
		if err != nil {
			return true, fmt.Errorf("Unable to create pwx directories: %s", err)
		}
	}

	reFilter := &regexFilter{re: rsyncRegex}
	args := []string{"--upgrade-inplace", "--info=stats2"}
	if debugsOn {
		// do verbose rsync if debug is turned on
		args = append(args, "-v")
	}
	err := di.RunOnce(imageName, ociInstallerName, []string{"/opt/pwx:/opt/pwx", "/etc/pwx:/etc/pwx"},
		[]string{"/runc-entry-point.sh", "--debug"}, args, reFilter)
	if err != nil {
		logrus.WithError(err).Error("Could not install ", imageName)
		usage("Could not install " + imageName +
			" - please inspect docker's log, and contact Portworx support.")
	}

	numTransferredFiles := -1
	if len(reFilter.val) > 0 {
		// get rid of ,'s: 25,455 -> 25455
		matched := strings.Replace(string(reFilter.val), ",", "", -1)
		numTransferredFiles, err = strconv.Atoi(matched)
		if err != nil {
			logrus.Warn("Could not find number of transferred files")
			numTransferredFiles = -1 // reset, just in case
		} else {
			logrus.Infof("Number of transferred files: %d", numTransferredFiles)
		}
	}

	logrus.Info("Installing Portworx...")

	// Compose startup-line for PX-RunC
	args = make([]string, 0, 1+len(cfg.Args)+len(cfg.Env)*2+len(cfg.Mounts)*2)
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

	var out cachingOutput
	if err = runExternalWithOutput(&out, args[0], args[1:]...); err != nil {
		logrus.WithError(err).Error("Could not install PX-RunC")
		return true, err
	}
	installOutput := out.String()

	// figure out if update required
	needsUpdate := false
	if numTransferredFiles > 3 {
		logrus.Infof("Portworx service restart required as %d files updated.", numTransferredFiles)
		needsUpdate = true
	} else if numTransferredFiles < 0 {
		logrus.Info("Portworx service restart required as unknown number of files updated.")
		needsUpdate = true
	}
	if isRestartRequired(installOutput) {
		logrus.Info("Portworx service restart required due to configuration update.")
		needsUpdate = true
	}
	if _, err := os.Stat(pxConfigFile); err != nil {
		logrus.WithError(err).Debug("Error statting ", pxConfigFile)
		logrus.Info("Portworx service restart required due to missing/invalid ", pxConfigFile)
	}
	return needsUpdate, nil
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
			return fmt.Errorf("File/Directory %s not mounted - please mount via 'run -v ...' option", m)
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
		logrus.WithError(err).Error("Could not talk to Docker")
		usage("Could not talk to Docker" +
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
	isRestartRequired, err := installPxFromOciImage(di, pxImage, opts)
	if err != nil {
		logrus.WithError(err).Error("Could not install Portworx service")
		os.Exit(-1)
	}

	if isRestartRequired {
		logrus.Warn("Restarting portworx service")
		logrus.Info("Reloading services")
		err = runExternal("/bin/sh", "-c", "systemctl daemon-reload")
		logrus.Info("Stopping Portworx service (if any)")
		err = runExternal("/bin/sh", "-c", "systemctl stop portworx")
		logrus.WithError(err).Debugf("Stopping done")

		logrus.Info("Enabling and Starting Portworx service")
		err = runExternal("/bin/sh", "-c",
			`systemctl enable portworx && systemctl start portworx`)
		if err != nil {
			logrus.WithError(err).Error("Could not start Portworx service")
			os.Exit(-1)
		}
	} else {
		logrus.Info("Portworx service restart not required.")
	}
}

func doUninstall() {
	logrus.Info("Stopping Portworx service")
	var b bytes.Buffer
	err := runExternalWithOutput(&b, "/bin/sh", "-c",
		`systemctl stop portworx`)
	if err != nil {
		strerr := b.String()
		if strings.Contains(strerr, " not loaded") {
			logrus.Info(strerr)
		} else {
			logrus.WithError(err).Error("Could not stop Portworx service")
			os.Exit(-1) // NOTE: CRITICAL failure !!
		}
	}

	logrus.Info("Disabling Portworx service")
	b.Reset()
	err = runExternalWithOutput(&b, "/bin/sh", "-c",
		`systemctl disable portworx`)
	if err != nil {
		strerr := b.String()
		if strings.Contains(strerr, "No such file or directory") {
			logrus.Info("Portworx service already disabled")
		} else {
			logrus.WithError(err).Error("Could not disable Portworx service")
			os.Exit(-1) // NOTE: CRITICAL failure !!
		}
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

// getKubernetesRootDir scans the external kubelet service for "--root-dir=XX" override, or returns a default kubelet dir
func getKubernetesRootDir() (string, error) {
	logrus.Info("Locating kubelet's local state directory")
	var out cachingOutput
	args := strings.Fields(`/bin/ps --no-headers -o cmd -C kubelet`)
	if err := runExternalWithOutput(&out, args[0], args[1:]...); err != nil {
		err = fmt.Errorf("Could not find kubelet service: %s", err)
		return "", err
	}
	m := xtractKubeletRegex.FindAllStringSubmatch(out.String(), -1)
	if len(m) > 0 && len(m[0]) > 1 {
		return m[0][1], nil
	}
	// return default value
	return "/var/lib/kubelet", nil
}

// used for unit-tests
var getKubernetesRootDirFn = getKubernetesRootDir

// isRestartRequired returns FALSE if no updates detected, or if only POD-mounts -specific updates are present.
// It returns TRUE if spec is new, if input not parseable, or non POD-mounts -specific updates present.
func isRestartRequired(in string) bool {
	if strings.Index(in, "SPEC UNCHANGED ") > 0 {
		return false
	} else if strings.Index(in, "SPEC CREATED ") > 0 {
		return true
	}

	// find proper location for "/var/lib/kubelet/pods/"
	kubeletPodsDir, err := getKubernetesRootDirFn()
	if err != nil {
		logrus.WithError(err).Error("Error scanning kubelet process")
		return true
	}
	kubeletPodsDir += "/pods/"

	search4 := " Updated mounts: add{"
	addStartIdx := strings.Index(in, search4)
	if addStartIdx > 0 {
		addStartIdx += len(search4)
		if endIdx := strings.Index(in[addStartIdx:], "}"); endIdx > 0 {
			endIdx += addStartIdx
			for _, p := range strings.Fields(in[addStartIdx:endIdx]) {
				logrus.Debugf("add/%s/", p)
				if !strings.HasPrefix(p, kubeletPodsDir) {
					return true
				}
			}
		} else {
			logrus.Error("INTERNAL ERROR - found mounts add{ with no matching }")
			return true
		}
	} else {
		// reset -- we never found added mounts
		addStartIdx = 0
	}

	search4 = " rm{"
	if rmStartIdx := strings.Index(in[addStartIdx:], search4); rmStartIdx > 0 {
		rmStartIdx += addStartIdx + len(search4)
		if endIdx := strings.Index(in[rmStartIdx:], "}"); endIdx > 0 {
			endIdx += rmStartIdx
			for _, p := range strings.Fields(in[rmStartIdx:endIdx]) {
				logrus.Debugf("rm/%s/", p)
				if !strings.HasPrefix(p, kubeletPodsDir) {
					return true
				}
			}
		} else {
			logrus.Error("INTERNAL ERROR - found mounts rm{ with no matching }")
			return true
		}
	}
	return false
}

func main() {
	if len(os.Args) < 2 || (os.Args[1] != "install" && os.Args[1] != "uninstall") {
		usage("First argument must be <install|uninstall>")
	}

	if debugsOn = os.Getenv("DEBUG") != ""; !debugsOn {
		for _, v := range os.Args[1:] {
			if v == "--debug" {
				debugsOn = true
				break
			}
		}
	}
	if debugsOn {
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

	// NOTE: we are DaemonSet entrypoint, so we should not exit
	logrus.Infof("%s done - going to sleep", strings.Title(os.Args[1]))
	err := syscall.Pause()
	logrus.WithError(err).Error("Could not pause")
}
