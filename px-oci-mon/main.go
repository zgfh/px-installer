package main

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"regexp"
	"runtime"
	"strings"
	"syscall"

	"github.com/portworx/px-installer/px-oci-mon/utils"
	"github.com/portworx/sched-ops/k8s"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/pkg/api/v1"
)

const (
	ociInstallerImage  = "portworx/px-enterprise:1.2.11"
	ociInstallerName   = "px-oci-installer"
	hostProcMount      = "/host_proc/1/ns/mnt"
	baseDir            = "/opt/pwx/oci"
	baseServiceName    = "portworx"
	baseServiceFileFmt = "/etc/systemd/system/%s.service"
	pxConfigFile       = "/etc/pwx/config.json"
	pxImageKey         = "PX_IMAGE"
	pxImageIDKey       = "PX_IMAGE_ID"
)

var (
	// xtractKubeletRegex extracts /var/kubelet -override from running kubelet daemon
	xtractKubeletRegex = regexp.MustCompile(`\s+--root-dir=(\S+)`)
	debugsOn           = false
	lastPxDisabled     = false
	lastServiceCmd     = ""
	ociService         *utils.OciServiceControl
	ociPrivateMounts   = map[string]bool{
		"/etc/pwx:/etc/pwx":                         true,
		"/opt/pwx:/opt/pwx":                         true,
		"/etc/systemd/system:/etc/systemd/system":   true,
		"/proc/1/ns:/host_proc/1/ns":                true,
		"/var/run/docker.sock:/var/run/docker.sock": true,
	}
)

// usage borrowed from ../../porx/cmd/px-runc/px-runc.go -- TODO: Consider refactoring !!
func usage(args ...interface{}) {
	if len(args) > 0 {
		logrus.Error(args...)
		fmt.Fprintln(os.Stderr)
	}

	fmt.Printf(`Usage: %[1]s [options]

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
   -t <token>                Portworx lighthouse token for cluster

kvdb-options:
   -userpwd <user:passwd>    Username and password for ETCD authentication
   -ca <file>                Specify location of CA file for ETCD authentication
   -cert <file>              Specify locationof certificate for ETCD authentication
   -key <file>               Specify location of certificate key for ETCD authentication
   -acltoken <token>         ACL token value used for Consul authentication

examples:
   %[1]s -k etcd://70.0.1.65:2379 -c MY_CLUSTER_ID -s /dev/sdc -d enp0s8 -m enp0s8

`, os.Args[0], baseDir, baseServiceName, fmt.Sprintf(baseServiceFileFmt, baseServiceName))
	os.Exit(1)
}

// Output filters --

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
	logrus.Info("Downloading Portworx image...")

	pxNeedsRestart := false

	downloadCbFn := func() error {
		logrus.Info("Docker image download detected - assuming upgrade and shutting down OCI (restart pending)")
		pxNeedsRestart = true
		return ociService.Stop()
	}

	err := di.PullImageCb(imageName, downloadCbFn)
	if err != nil {
		logrus.WithError(err).Error("Could not pull ", imageName)
		usage("Could not pull " + imageName +
			" - have you specified REGISTRY_USER/REGISTRY_PASS env. variables?")
	}

	pxNeedsInstall := true
	if pulledID, err := di.GetImageID(imageName); err == nil && len(pulledID) > 19 {
		logrus.Info("Pulled PX image ID ", pulledID)
		cfg.Env = append(cfg.Env, pxImageIDKey+"="+pulledID)

		// compare w/ installed image
		ociConfigFile := path.Join(baseDir, "config.json")
		installedID, err := utils.ExtractEnvFromOciConfig(ociConfigFile, pxImageIDKey)
		if err == nil && len(installedID) > 19 {
			if pulledID == installedID {
				logrus.Infof("Installed image ID %s same as pulled image ID %s",
					installedID[7:19], pulledID[7:19])
				pxNeedsInstall = false
			} else {
				logrus.Infof("Installed image ID %s _DIFFERENT_ than pulled image ID %s",
					installedID[7:19], pulledID[7:19])
			}
		} else {
			logrus.WithError(err).Warnf("Could not retrieve installed OCI image ID (is this initial install?)")
		}
	} else {
		logrus.WithError(err).Error("Could not retrieve PX image ID")
	}

	if pxNeedsInstall {
		logrus.Info("Installing/Upgrading Portworx OCI files (restart pending)")
		pxNeedsRestart = true
		if err := ociService.Stop(); err != nil {
			return true, err
		}

		args := []string{"--upgrade"}
		if debugsOn {
			// do verbose rsync if debug is turned on
			args = append(args, "--debug")
		}
		err := di.RunOnce(imageName, ociInstallerName, []string{"/opt/pwx:/opt/pwx", "/etc/pwx:/etc/pwx"},
			[]string{"/runc-entry-point.sh"}, args)
		if err != nil {
			logrus.WithError(err).Error("Could not install ", imageName)
			usage("Could not install " + imageName +
				" - please inspect docker's log, and contact Portworx support.")
		}
	}

	logrus.Info("Installing Portworx OCI service...")

	pxUnitFile := fmt.Sprintf(baseServiceFileFmt, baseServiceName)
	oldUnitSt, err := os.Stat(pxUnitFile)
	if err != nil {
		logrus.WithError(err).Warn("Could not find service-file (is this initial install?)")
	}

	// Compose startup-line for PX-RunC
	args := make([]string, 0, 1+len(cfg.Args)+len(cfg.Env)*2+len(cfg.Mounts)*2)
	args = append(args, "/opt/pwx/bin/px-runc", "install")
	if strings.HasSuffix(strings.ToLower(cfg.Args[0]), "install") {
		// skip INSTALL/UNINSTALL arg...
		args = append(args, cfg.Args[1:]...)
	} else {
		args = append(args, cfg.Args...)
	}

	// Add Mounts
	for _, vol := range cfg.Mounts {
		// skip local mounts, pass the others
		if _, has := ociPrivateMounts[vol]; has {
			continue
		}
		args = append(args, "-v", vol)
	}

	// Add Environment
	for _, env := range cfg.Env {
		args = append(args, "-e", env)
	}

	// TODO: Add Labels?

	var installOutput cachingOutput
	if err = ociService.RunExternal(&installOutput, args[0], args[1:]...); err != nil {
		logrus.WithError(err).Error("Could not install PX-RunC")
		return true, err
	}

	// figure out if update required due to config change or other reasons

	// 1. check status of the unit-file
	if oldUnitSt == nil {
		logrus.Info("Portworx service restart required due to initial config.")
		pxNeedsRestart = true
		// let's also do reload + enable of the service
		ociService.Reload()
		if err = ociService.Enable(); err != nil {
			logrus.WithError(err).Error("Could not enable service.")
		}
	} else if newUnitSt, err := os.Stat(pxUnitFile); err != nil {
		return true, fmt.Errorf("Could not stat %s: %s", pxUnitFile, err)
	} else if newUnitSt.ModTime().Sub(oldUnitSt.ModTime()) > 0 {
		logrus.Info("Portworx service restart required due to updated ", pxUnitFile)
		pxNeedsRestart = true
	}

	// 2. check output of "px-runc install"
	if isRestartRequired(installOutput.String()) {
		logrus.Info("Portworx service restart required due to configuration update.")
		pxNeedsRestart = true
	}

	// 3. check for missing /etc/pwx/config.json
	if _, err := os.Stat(pxConfigFile); err != nil {
		logrus.WithError(err).Debug("Error stat ", pxConfigFile)
		logrus.Info("Portworx service restart required due to missing/invalid ", pxConfigFile)
		pxNeedsRestart = true
	}
	return pxNeedsRestart, nil
}

func validateMounted(mounts ...string) error {
	var st0, st1 syscall.Stat_t

	err := syscall.Lstat("/", &st0)
	if err != nil {
		// improbable, but let's handle it
		return fmt.Errorf("INTERNAL ERROR - could not stat '/': %s", err)
	}

	errMounts := make([]string, 0, 3)
	for _, m := range mounts {
		err = syscall.Lstat(m, &st1)
		if err != nil {
			logrus.WithError(err).Errorf("File/Directory %s not found - please mount via 'run -v ...' option", m)
			errMounts = append(errMounts, m)
		} else if st0.Dev == st1.Dev {
			logrus.Errorf("File/Directory %s not mounted - please mount via 'run -v ...' option", m)
			errMounts = append(errMounts, m)
		}
	}

	if len(errMounts) > 0 {
		err = fmt.Errorf("Following dirs/files must be mounted into this continer: %s",
			strings.Join(errMounts, ", "))
	}

	return err
}

func doInstall() error {
	pxImage := os.Getenv(pxImageKey)
	if pxImage == "" {
		pxImage = ociInstallerImage
	}

	id, err := utils.GetMyContainerID()
	if err != nil {
		return fmt.Errorf("Could not determine my container ID: %s", err)
	}

	di, err := utils.NewDockerInstaller(os.Getenv("REGISTRY_USER"), os.Getenv("REGISTRY_PASS"))
	if err != nil {
		logrus.WithError(err).Error("Could not talk to Docker")
		usage("Could not talk to Docker" +
			" - please restart using '-v /var/run/docker.sock:/var/run/docker.sock' option")
	}

	opts, err := di.ExtractConfig(id)
	if err != nil {
		return fmt.Errorf("Could not extract my container's configuration: %s", err)
	}
	if _, has := os.LookupEnv(pxImageKey); !has { // add PX_IMAGE env if missing
		opts.Env = append(opts.Env, pxImageKey+"="+pxImage)
	}
	// Filter out undesired ENV entries
	envListFilt := make([]string, 0, len(opts.Env))
	for _, v := range opts.Env {
		if strings.HasPrefix(v, "PATH=") {
			logrus.Debugf("Removing %q entry from ENV", v)
			continue
		}
		envListFilt = append(envListFilt, v)
	}
	opts.Env = envListFilt

	// TODO: Sanity checks for options
	logrus.Debugf("OPTIONS:: %+v\n", opts)
	isRestartRequired, err := installPxFromOciImage(di, pxImage, opts)
	if err != nil {
		return fmt.Errorf("Could not install Portworx service: %s", err)
	}

	if isRestartRequired {
		logrus.Warn("Reloading + Restarting portworx service")
		err = ociService.Reload()
		if err != nil {
			logrus.WithError(err).Warn("Error reloading service (cont)")
		}
		err = ociService.Restart()
		if err != nil {
			return err
		}
	} else {
		logrus.Info("Portworx service restart not required.")
	}
	return nil
}

func doUninstall() error {
	logrus.Info("Stopping Portworx service")
	if err := ociService.Stop(); err != nil {
		return err
	}

	logrus.Info("Disabling Portworx service")
	if err := ociService.Disable(); err != nil {
		return err
	}

	logrus.Info("Removing Portworx service bind-mount (if any) and uninstall")
	if err := ociService.Remove(); err != nil {
		return err
	}
	return nil
}

// getKubernetesRootDir scans the external kubelet service for "--root-dir=XX" override, or returns a default kubelet dir
func getKubernetesRootDir() (string, error) {
	logrus.Info("Locating kubelet's local state directory")
	var out cachingOutput
	args := strings.Fields(`/bin/ps --no-headers -o cmd -C kubelet`)
	if err := ociService.RunExternal(&out, args[0], args[1:]...); err != nil {
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

// watchNodeLabels monitors the label changes on the Node
// NOTE: see https://kubernetes.io/docs/concepts/workloads/pods/pod/#termination-of-pods
func watchNodeLabels(node *v1.Node) error {
	logrus.Debugf("WATCH labels: %+v", node.GetLabels())

	isPxDisabled := utils.IsPxDisabled(node)
	defer func() { lastPxDisabled = isPxDisabled }()
	if !isPxDisabled && lastPxDisabled {
		logrus.Info("Requested PX-enablement via labels")
		doInstall()
	} else if isPxDisabled && !lastPxDisabled {
		logrus.Info("Requested PX-disablement via labels")
		if utils.IsUninstallRequested(node) {
			doUninstall()
			utils.DisablePx(node)
		} else {
			logrus.Warn("Label 'px/enable=false' set directly, not removing the OCI install" +
					" (use px/enable=remove to uninstall)")
		}
	} else if req := utils.GetServiceRequest(node); req != "" {
		if req == lastServiceCmd {
			logrus.Debug("Ignoring service-request for ", req)
			return nil
		}
		lastServiceCmd = req
		if err := ociService.HandleRequest(req); err != nil {
			logrus.Error(err)
		} else if req == "restart" {
			// we're removing "restart" label, and but keeping the others
			utils.RemoveServiceLabel(node)
			lastServiceCmd = ""
		}
	}
	return nil
}

func main() {
	// Debugs on?
	if debugsOn = os.Getenv("DEBUG") != ""; !debugsOn {
		for _, v := range os.Args[1:] { // skim thorough the debug-opts
			if v == "--debug" {
				debugsOn = true
				break
			}
		}
	}
	if debugsOn {
		logrus.SetLevel(logrus.DebugLevel)
	}

	// Validate required OCI mounts are all valid and accounted for
	if len(ociPrivateMounts) > 0 {
		dirs, i := make([]string, len(ociPrivateMounts)), 0
		for k := range ociPrivateMounts {
			dirs[i] = strings.Split(k, ":")[1]
			i++
		}
		if err := validateMounted(dirs...); err != nil {
			logrus.Error(err)
			os.Exit(-1)
		}
	}

	ociService = utils.NewOciServiceControl(hostProcMount, baseServiceName)

	meNode, err := utils.FindMyNode()
	if err != nil || meNode == nil {
		logrus.Errorf("Could not find my node in Kubernetes cluster: %s", err)
		os.Exit(1)
	}

	lastOp := "Install"
	if lastPxDisabled = utils.IsPxDisabled(meNode); lastPxDisabled {
		err = doUninstall()
		lastOp = "Uninstall"
	} else {
		err = doInstall()
	}
	if err != nil {
		// note: CRITICAL FAILURE if install | uninstall failed
		logrus.Error(err)
		os.Exit(-1)
	}

	logrus.Info("Activating node-watcher")
	k8s.Instance().WatchNode(meNode, watchNodeLabels)

	// NOTE: exiting the main() goroutine, the daemonSet is still maintained "alive" via Watcher
	logrus.Info(lastOp, " done - MAIN exiting")
	runtime.Goexit()
	// normally unreachable
	logrus.Error("Could not exit MAIN !!")
}
