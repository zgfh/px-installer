package utils

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
)

const (
	opSTART   = "start"
	opSTOP    = "stop"
	opRESTART = "restart"
	opENABLE  = "enable"
	opDISABLE = "disable"
)

// OciServiceControl provides "systemctl"-like controls over the external OCI service
type OciServiceControl struct {
	hostProcMount string
	service       string
}

// NewOciServiceControl creates a new instance of ociServiceControl
func NewOciServiceControl(mountNs, service string) *OciServiceControl {
	return &OciServiceControl{mountNs, service}
}

// RunExternal is a generic runner of external commands
func (o *OciServiceControl) RunExternal(out io.Writer, name string, params ...string) error {
	args := make([]string, 0, 4+len(params))
	args = append(args, "/usr/bin/nsenter", "--mount="+o.hostProcMount, "--", name)
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

func (o *OciServiceControl) do(op string) error {
	logrus.Infof("Doing %s service %s", o.service, op)
	var b bytes.Buffer
	cmd := fmt.Sprintf("systemctl %s %s", op, o.service)
	err := o.RunExternal(&b, "/bin/sh", "-c", cmd)
	logrus.WithError(err).WithField("out", b.String()).Debugf("SVC %sed", op)
	if err != nil {
		err = fmt.Errorf("Could not %s service: %s", op, err)
	}
	return err
}

// Start the service
func (o *OciServiceControl) Start() error {
	return o.do(opSTART)
}

// Stop the service
func (o *OciServiceControl) Stop() error {
	return o.do(opSTOP)
}

// Restart the service
func (o *OciServiceControl) Restart() error {
	return o.do(opRESTART)
}

// Enable the service
func (o *OciServiceControl) Enable() error {
	return o.do(opENABLE)
}

// Disable the service
func (o *OciServiceControl) Disable() error {
	return o.do(opDISABLE)
}

// Reload the service files
func (o *OciServiceControl) Reload() error {
	var b bytes.Buffer
	err := o.RunExternal(&b, "/bin/sh", "-c", `systemctl daemon-reload`)
	logrus.WithError(err).WithField("out", b.String()).Debug("OCI reloaded")
	if err != nil {
		err = fmt.Errorf("Could not reload service: %s", err)
	}
	return err
}

// Remove the service files (called by Main directly)
func (o *OciServiceControl) Remove() error {
	logrus.Info("Removing service bind-mount (if any)")
	err := o.RunExternal(nil, "/bin/sh", "-c",
		fmt.Sprintf(`grep -q ' %[1]s %[1]s ' /proc/self/mountinfo && umount %[1]s`, "/opt/pwx/oci"))
	if err != nil {
		// log and attempt removal
		logrus.WithError(err).Warn("Could not bind-umount Portworx files (continuing)")
	}

	logrus.Info("Removing Portworx files")
	fname := fmt.Sprintf("/etc/systemd/system/%s.service", o.service)
	err = o.RunExternal(nil, "/bin/rm", "-fr", "/opt/pwx", fname)
	if err != nil {
		err = fmt.Errorf("Could not remove all systemd files: %s", err)
	}
	return err
}

// HandleRequest will execute the systemctl -equivalent control command
func (o *OciServiceControl) HandleRequest(op string) (err error) {
	switch op {
	case opSTART, opSTOP, opRESTART, opENABLE, opDISABLE:
		return o.do(op)
	// NOTE: INSTALL and UNINSTALL (REMOVE) is being handling via main()
	default:
		return fmt.Errorf("Unsupported service request: %s", op)
	}
	return err
}
