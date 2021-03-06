package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/portworx/px-installer/px-oci-mon/utils"
	"github.com/portworx/sched-ops/k8s"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
)

const (
	ociInstallerName   = "px-oci-installer"
	hostProcMount      = "/host_proc/1/ns/mnt"
	baseDir            = "/opt/pwx/oci"
	baseServiceName    = "portworx"
	baseServiceFileFmt = "/etc/systemd/system/%s.service"
	pxConfigFile       = "/etc/pwx/config.json"
	pxImageKey         = "PX_IMAGE"
	pxImageIDKey       = "PX_IMAGE_ID"
	instK8sDir         = "/opt/pwx/oci/inst-k8s"
	instScratchDir     = "/opt/pwx/oci/inst-scratchDir"
	sha1verBegin       = 7
	sha1verEnd         = 19
	// pxImagePrefix will be combined w/ PXTAG to create the linked docker-image
	pxImagePrefix = "portworx/px-enterprise"
	defaultPXTAG  = "1.2.12.1"
)

var (
	// xtractKubeletRegex extracts /var/kubelet -override from running kubelet daemon
	xtractKubeletRegex = regexp.MustCompile(`\s+--root-dir=(\S+)`)
	debugsOn           = false
	lastPxDisabled     = false
	lastServiceCmd     = ""
	ociService         *utils.OciServiceControl
	ociRestServer      *utils.OciRESTServlet
	ociPrivateMounts   = map[string]bool{
		"/etc/pwx:/etc/pwx":                         true,
		"/opt/pwx:/opt/pwx":                         true,
		"/etc/systemd/system:/etc/systemd/system":   true,
		"/proc/1/ns:/host_proc/1/ns":                true,
		"/var/run/docker.sock:/var/run/docker.sock": true,
	}
	kubernetesArgs    = []string{"-x", "kubernetes"}
	optPreSync        = false
	optDrainAllPods = false
	optRestEndpoint   = ""
	meNode            *v1.Node
	// PXTAG is externally defined image tag (can use `go build -ldflags "-X main.PXTAG=1.2.3" ... `
	// to set portworx/px-enterprise:1.2.3)
	PXTAG string
)

// usage borrowed from ../../porx/cmd/px-runc/px-runc.go -- TODO: Consider refactoring !!
func usage(args ...interface{}) {
	if len(args) > 0 {
		logrus.Error(args...)
		fmt.Fprintln(os.Stderr)
	}

	fmt.Printf(`Usage: %[1]s [options]

options:
   --endpoint <ip:port>  Start REST service at specific endpoint
   --sync                Will issue sync operation before stopping/restarting the PX-OCI service
   --drain-all           Will drain ALL PX-dependent pods before upgrade (dfl. only managed nodes get drained)
   --log <file>          Will use logfile instead of Docker-log
   --debug               Increase logs-verbosity to debug-level
   *                     Any additional options will be passed on to px-runc

NOTE that any options not explicitly listed above, will be passed directly to px-runc.
For details please see http://docs.portworx.com/runc

`, os.Args[0])
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

type installStatus struct {
	needRestart bool
	needInstall bool
	needCordon  bool
}

// installPxFromOciImage downloads the Docker image, and (if required) runs the install/upgrade to the alternate location.
func installPxFromOciImage(di *utils.DockerInstaller, imageName string, cfg *utils.SimpleContainerConfig) (installStatus, error) {
	logrus.Info("Downloading Portworx image...")

	retSt := installStatus{false, false, false} // assume no install/restart/cordon needed

	downloadCbFn := func() error {
		logrus.Info("Docker image download detected - assuming upgrade and setting OCI-mon to unhealthy")
		ociRestServer.SetStateInstalling()
		return nil
	}

	err := di.PullImageCb(imageName, downloadCbFn)
	if err != nil {
		logrus.WithError(err).Error("Could not pull ", imageName)
		usage("Could not pull " + imageName +
			" - have you specified REGISTRY_USER/REGISTRY_PASS env. variables?")
	}

	retSt.needInstall = true // assume yes, fix later
	if pulledID, err := di.GetImageID(imageName); err == nil && len(pulledID) > sha1verEnd {
		logrus.Info("Pulled PX image ID ", pulledID)
		cfg.Env = append(cfg.Env, pxImageIDKey+"="+pulledID)

		// compare w/ installed image
		ociConfigFile := path.Join(baseDir, "config.json")
		installedID, err := utils.ExtractEnvFromOciConfig(ociConfigFile, pxImageIDKey)
		if err == nil && len(installedID) > sha1verEnd {
			if pulledID == installedID {
				logrus.Infof("Installed image ID %s same as pulled image ID %s",
					installedID[sha1verBegin:sha1verEnd], pulledID[sha1verBegin:sha1verEnd])
				retSt.needInstall = false
				ociRestServer.SetStateInstallFinished()
			} else {
				logrus.Infof("Installed image ID %s _DIFFERENT_ than pulled image ID %s",
					installedID[sha1verBegin:sha1verEnd], pulledID[sha1verBegin:sha1verEnd])
			}
		} else {
			logrus.WithError(err).Warnf("Could not retrieve installed OCI image ID (is this initial install?)")
		}
	} else {
		logrus.WithError(err).Error("Could not retrieve PX image ID")
	}

	if retSt.needInstall {
		logrus.Info("Installing/Upgrading Portworx OCI files (restart pending)")

		args := []string{"--upgrade"}
		if debugsOn {
			// do verbose rsync if debug is turned on
			args = append(args, "--debug")
		}

		// set up log-parser to determine if cordoning/draining will be required
		logProcCb := func(log []byte, err error) {
			if err != nil {
				// log incomplete, require cordoning/draining
				logrus.WithError(err).Warnf("Could not get complete px-oci-installer log")
				retSt.needCordon = true
			} else if bytes.Contains(log, []byte(" require reboot ")) {
				logrus.Warn("Will require Cordoning/Draining the node's containers")
				retSt.needCordon = true
			} else {
				logrus.Info("PX module OK (no cordon/pod-draining required)")
				retSt.needCordon = false
			}
		}
		err := di.RunOnce(imageName, ociInstallerName, []string{instK8sDir + ":/opt/pwx", "/etc/pwx:/etc/pwx"},
			[]string{"/runc-entry-point.sh"}, args, logProcCb)
		if err != nil {
			logrus.WithError(err).Error("Could not install ", imageName)
			usage("Could not install " + imageName +
				" - please inspect docker's log, and contact Portworx support.")
		}
	}

	logrus.Info("Installing Portworx OCI service...")

	// Compose startup-line for PX-RunC
	args := make([]string, 0, 6+len(cfg.Args)+len(cfg.Env)*2+len(cfg.Mounts)*2)
	var pxUnitFile string
	var oldUnitFileModTime time.Time
	if retSt.needInstall {
		// NOTE: we dumped the OCI into a separate directory!
		// now we need a tweaked install-- example /opt/pwx/k8s/bin/px-runc install -oci /opt/pwx/k8s/oci -sysd /dev/null -c zox-dbg-mk126 -m enp0s8 -d enp0s8 -s /dev/sdc
		args = append(args, path.Join(instK8sDir, "bin/px-runc"), "install", "-oci",
			path.Join(instK8sDir, "oci"), "-sysd", "/dev/null")
		pxUnitFile = ""
	} else {
		args = append(args, "/opt/pwx/bin/px-runc", "install")

		pxUnitFile = fmt.Sprintf(baseServiceFileFmt, baseServiceName)
		if st, err := os.Stat(pxUnitFile); err != nil {
			logrus.WithError(err).Warn("Could not find service-file (is this initial install?)")
		} else {
			oldUnitFileModTime = st.ModTime()
		}
	}

	if strings.HasSuffix(strings.ToLower(os.Args[1]), "install") {
		// skip INSTALL/UNINSTALL arg...
		args = append(args, os.Args[2:]...)
	} else {
		args = append(args, os.Args[1:]...)
	}

	// Add Mounts
	for _, vol := range cfg.Mounts {
		// skip local mounts, pass the others
		if _, has := ociPrivateMounts[vol]; has {
			logrus.Debugf("Skipping mount %s", vol)
			continue
		} else if len(vol) < 4 || strings.HasPrefix(vol, "/var/run/docker.sock:") {
			// Additional checks - skip anything under `len(a:/b)`, also under no circumstances
			// should we pass docker.sock directly
			logrus.Debugf("Also skipping mount %s", vol)
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
		retSt.needRestart = true
		return retSt, err
	}

	/*
	 * figure out if update required due to config change or other reasons
	 */

	// 1. check status of the unit-file (if valid)
	if pxUnitFile != "" {
		if oldUnitFileModTime.IsZero() {
			logrus.Info("Portworx service restart required due to initial config.")
			retSt.needRestart = true
			// let's also do reload + enable of the service
			if err = ociService.Reload(); err != nil {
				logrus.WithError(err).Error("Could not reload service.")
			}
			if err = ociService.Enable(); err != nil {
				logrus.WithError(err).Error("Could not enable service.")
			}
		} else if newUnitSt, err := os.Stat(pxUnitFile); err != nil {
			err2 := fmt.Errorf("Could not stat %s: %s", pxUnitFile, err)
			retSt.needRestart = true
			return retSt, err2
		} else if newUnitSt.ModTime().Sub(oldUnitFileModTime) > 0 {
			logrus.Info("Portworx service restart required due to updated ", pxUnitFile)
			retSt.needRestart = true
		}
	} else if retSt.needInstall {
		logrus.Info("Portworx service restart required due to OCI upgrade/install")
		retSt.needRestart = true
	}

	// 2. check output of "px-runc install"
	if isRestartRequired(installOutput.String()) {
		logrus.Info("Portworx service restart required due to configuration update.")
		retSt.needRestart = true
	}

	// 3. check for missing /etc/pwx/config.json
	if _, err := os.Stat(pxConfigFile); err != nil {
		logrus.WithError(err).Debug("Error stat ", pxConfigFile)
		logrus.Info("Portworx service restart required due to missing/invalid ", pxConfigFile)
		retSt.needRestart = true
	}
	return retSt, nil
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
			logrus.WithError(err).Errorf("Directory/File %s not found - please add as mount", m)
			errMounts = append(errMounts, m)
		} else if st0.Dev == st1.Dev {
			logrus.Errorf("Directory/File %s not mounted - please add as mount", m)
			errMounts = append(errMounts, m)
		}
	}

	if len(errMounts) > 0 {
		err = fmt.Errorf("Following dirs/files must be mounted into this continer: %s",
			strings.Join(errMounts, ", "))
	}

	return err
}

// isExist returns TRUE only if path exists
func isExist(parts ...string) bool {
	path := path.Join(parts...)
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// moveFileOrDir moves a file or a directory from one location to another
func moveFileOrDir(src, dest string) error {
	// os.Rename(src, dest) -- cannot use in case of moves across mountpoints
	cmd := exec.Command("/bin/mv", src, dest)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()

	if err != nil {
		logrus.WithError(err).WithField("out", out.String()).Errorf("Could not move %s to %s", src, dest)
		err = fmt.Errorf("Could not move %s to %s: %s", src, dest, err)
	}
	return err
}

// switchOciInstall moves the new OCI-rootfs at {optK8sDir}, to the original /opt/pwx
func switchOciInstall() error {
	logrus.Infof("Finalizing OCI install -- Moving temp OCI image from %s/ to /opt/pwx/", instK8sDir)

	success := false

	if isExist(instScratchDir) {
		logrus.Warnf("Directory %s not empty (purging now)", instScratchDir)
		os.RemoveAll(instScratchDir)
	}
	err := os.MkdirAll(path.Join(instScratchDir, "oci"), 0700)
	if err != nil {
		return fmt.Errorf("Could not create %s: %s", instScratchDir, err)
	}

	ociParts := strings.Fields("bin oci/rootfs oci/config.json")

	// schedule rollback (if required) and cleanup
	defer func() {
		if !success {
			logrus.Warnf("ROLLBACK: Rolling back %s/{bin,oci/*} to /opt/pwx/", instScratchDir)
			for _, p := range ociParts {
				if isExist(instScratchDir, p) {
					org, scr := path.Join("/opt/pwx", p), path.Join(instScratchDir, p)
					if err = os.RemoveAll(org); err != nil {
						logrus.WithError(err).Warn("Could not remove ", org)
					}
					logrus.Warn("ROLLBACK: Removed ", org)

					if err = moveFileOrDir(scr, org); err != nil {
						logrus.WithError(err).Warnf("Rollback %s FAILED", p)
					} else {
						logrus.Warnf("ROLLBACK: Moved %s to %s", scr, org)
					}
				}
			}
			logrus.Warn("ROLLBACK: Rollback completed.")
		}
		// fire off general async cleanup
		go func() {
			toRm := []string{instK8sDir, instScratchDir}
			logrus.Info("ASYNC: Launched deletion of ", toRm)
			for _, dir := range toRm {
				if err = os.RemoveAll(dir); err != nil {
					logrus.WithError(err).Warn("Could not remove ", dir)
				}
			}
			logrus.Info("ASYNC: Deletion completed.")
		}()
	}()

	if err = ociService.Stop(); err != nil {
		logrus.WithError(err).Warn("Error stopping service (cont)")
		// let's still continue, and attempt upgrade w/ the service "live"
	}

	logrus.Infof("Moving old /opt/pwx/{bin,oci/*} to %s; moving %s/{bin,oci/*} to /opt/pwx/", instScratchDir, instK8sDir)
	for _, p := range ociParts {
		// mv <orig> to <scratch> ...
		org, neo, scr := path.Join("/opt/pwx", p), path.Join(instK8sDir, p), path.Join(instScratchDir, p)
		if isExist(org) {
			if err = moveFileOrDir(org, scr); err != nil {
				return err
			}
			logrus.Infof("> mv %s %s -OK.", org, scr)
		}

		// mv <new> to <orig> ...
		if err = moveFileOrDir(neo, org); err != nil {
			return err
		}
		logrus.Infof("> mv %s %s -OK.", neo, org)
	}
	// re-running the install
	logrus.Info("OCI bits moved - reinstalling the PX-RunC")
	if err = ociService.RunExternal(nil, "/opt/pwx/bin/px-runc", "install"); err != nil {
		return fmt.Errorf("Could not run `px-runc install`: %s", err)
	}
	success = true
	return nil
}

func finalizePxOciInstall(status installStatus) error {
	initialInstall := !isExist(fmt.Sprintf(baseServiceFileFmt, baseServiceName))

	if optPreSync {
		logrus.Info("Running sync() before PX-OCI install/upgrade")
		syscall.Sync()
	}

	if status.needInstall {
		if status.needCordon {
			err := utils.DrainPxVolumeConsumerPods(meNode, optDrainAllPods)
			if err != nil {
				logrus.WithError(err).Error("Error draining PX-dependent pods")
			} else {
				logrus.Info("PX-dependent pods successfully drained.")
				defer func() {
					if err = utils.UncordonNode(meNode); err != nil {
						logrus.WithError(err).Error("Error Uncordoning node")
					} else {
						logrus.Info("Node successfully uncordoned")
					}
				}()
			}
		}
		if err := switchOciInstall(); err != nil {
			return err
		}
	}

	logrus.Warn("Reloading + Restarting portworx service")

	if err := ociService.Reload(); err != nil {
		logrus.WithError(err).Warn("Error reloading service (cont)")
	}

	if initialInstall {
		logrus.Warn("Initial install detected - enabling the Portworx service")
		if err := ociService.Enable(); err != nil {
			logrus.WithError(err).Warn("Error enabling service (cont)")
		}
	}

	// Additional services we'd need to enable: portworx-reboot
	addtlSvcName := "portworx-reboot"
	if isExist(fmt.Sprintf(baseServiceFileFmt, addtlSvcName)) {
		svc := utils.NewOciServiceControl(hostProcMount, addtlSvcName)
		if err := svc.Enable(); err != nil {
			logrus.WithError(err).Error("Could not enable ", addtlSvcName)
		}
	} else {
		logrus.Debugf("%s.service does not exist - skipping enablement", addtlSvcName)
	}

	return ociService.Restart()
}

func doInstall() error {
	pxImage := os.Getenv(pxImageKey)
	if pxImage == "" {
		pxImage = pxImagePrefix + ":" + PXTAG
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
	logrus.Debugf("OPTIONS:: %#v", opts)
	instSt, err := installPxFromOciImage(di, pxImage, opts)
	if err != nil {
		return fmt.Errorf("Could not install Portworx service: %s", err)
	}

	if instSt.needRestart || instSt.needInstall || instSt.needCordon {
		if err = finalizePxOciInstall(instSt); err != nil {
			return fmt.Errorf("Could not finalize OCI install: %s", err)
		}
	} else {
		logrus.Info("Portworx service restart not required.")
	}
	ociRestServer.SetStateInstallFinished()
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
	err := ociService.Remove()

	// Uninstall additional services (portworx-reboot)
	if err == nil {
		addtlSvcName := "portworx-reboot"
		addtlUnitFile := fmt.Sprintf(baseServiceFileFmt, addtlSvcName)
		if isExist(addtlUnitFile) {
			svc := utils.NewOciServiceControl(hostProcMount, addtlSvcName)
			if err := svc.Stop(); err != nil {
				logrus.WithError(err).Error("Could not stop ", addtlSvcName)
			}
			if err := svc.Disable(); err != nil {
				logrus.WithError(err).Error("Could not disable ", addtlSvcName)
			}
			if err := svc.RunExternal(nil, "/bin/rm", "-f", addtlUnitFile); err != nil {
				logrus.WithError(err).Error("Could not remove ", addtlUnitFile)
			}
		} else {
			logrus.Debugf("%s.service does not exist - skipping removal", addtlSvcName)
		}
	}

	// returning error from OCI-service removal
	return err
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

		if err := ociService.HandleRequest(req); err != nil {
			logrus.Error(err)
			// note: in case of errors, we will _not_ reset the `lastServiceCmd`, so this request will be repeated
			// on the next watch (note that watch() triggers every few seconds, on every Node{}-update ).
		} else if req == "restart" {
			// successful restart - remove "restart" label (will keep others)
			utils.RemoveServiceLabel(node)
			lastServiceCmd = ""
		} else {
			// command was successful, persist it
			lastServiceCmd = req
		}
	}
	return nil
}

func setLogfile(fname string) error {
	logrus.Infof("Redirecting all output to %s", fname)
	f, err := os.OpenFile(fname, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	fmt.Fprintln(f, "------------------------------------------------------------------------------")
	os.Stdout, os.Stderr = f, f
	logrus.SetOutput(f)
	logrus.Info("Started logging into ", fname)
	return nil
}

func main() {
	logrus.Infof("Input arguments: %v", os.Args)
	args := make([]string, 0, len(os.Args))
	var scheduler *string
	ensureExtraArgFn := func(i int, opt string) {
		if (i+1) >= len(os.Args) {
			usage("ERROR: Argument ", opt, " requires extra option!  Please correct your configuration.")
		}
	}
	for i := 0; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "":
			logrus.Infof("NOTE -- skippng empty arg #%d", i)
			i++ // skip empty args
		case "--sync":
			optPreSync = true // local option
		case "--drain-all":
			optDrainAllPods = true // local option
		case "--endpoint":
			ensureExtraArgFn(i, os.Args[i])
			i++
			optRestEndpoint = os.Args[i] // local option
		case "--log":
			ensureExtraArgFn(i, os.Args[i])
			i++
			if err := setLogfile(os.Args[i]); err != nil {
				logrus.Errorf("Could not set up logging to %s: %s", os.Args[i], err)
				os.Exit(1)
			}
		case "-x":
			ensureExtraArgFn(i, os.Args[i])
			i++
			if os.Args[i] != "kubernetes" {
				logrus.Errorf("Invalid option '-x %s' provided."+
					"  Please correct your configuration.", os.Args[i])
				os.Exit(1)
			}
			args = append(args, kubernetesArgs...)
			scheduler = &os.Args[i]
		case "--help", "-h":
			usage()
		case "--debug":
			debugsOn = true
			fallthrough
		default:
			args = append(args, os.Args[i])
		}
	}
	if scheduler == nil {
		logrus.Warnf("Scheduler not specified - adding `-x kubernetes` to the parameters")
		args = append(args, kubernetesArgs...)
	}
	logrus.Infof("Updated arguments: %v", args)
	os.Args = args // reset to [potentially] trimmed down version

	if debugsOn || os.Getenv("DEBUG") != "" { // Debugs on?
		logrus.SetLevel(logrus.DebugLevel)
	}

	if PXTAG == "" {
		PXTAG = defaultPXTAG
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

	var err error
	meNode, err = utils.FindMyNode()
	if err != nil || meNode == nil {
		logrus.Errorf("Could not find my node in Kubernetes cluster: %s", err)
		os.Exit(1)
	}

	ociService = utils.NewOciServiceControl(hostProcMount, baseServiceName)
	ociRestServer = utils.NewRESTServlet(ociService, meNode)

	logrus.Info("Activating REST server")
	ociRestServer.Start(optRestEndpoint)

	lastOp := "Install"
	if utils.IsPxDisabled(meNode) {
		lastPxDisabled = false // force state change
		err = k8s.Instance().WatchNode(meNode, watchNodeLabels)
		lastOp = "Uninstall"
	} else {
		err = doInstall()
	}
	if err != nil {
		// note: CRITICAL FAILURE if install | uninstall failed
		logrus.Error(err)
		os.Exit(-1)
	}
	ociRestServer.SetStateInstallFinished()

	logrus.Info("Activating node-watcher")
	k8s.Instance().WatchNode(meNode, watchNodeLabels)

	// NOTE: exiting the main() goroutine, the daemonSet is still maintained "alive" via Watcher
	logrus.Info(lastOp, " done - MAIN exiting")
	runtime.Goexit()
	// normally unreachable
	logrus.Error("Could not exit MAIN !!")
}
