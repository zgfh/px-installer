package utils

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/portworx/sched-ops/k8s"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
)

const (
	enablementKey = "px/enabled"
	serviceKey    = "px/service"
)

var (
	disabledLabels = []string{
		"false", // please keep first, keyword used w/ k8s uninstall
		"uninstall",
		"remove",
		"rm",
	}
)

// GetLocalIPList returns the list of local IP addresses, and optionally includes local hostname.
func GetLocalIPList(includeHostname bool) ([]string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	ipList := make([]string, 0, len(ifaces))
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			return ipList, fmt.Errorf("Error listing addresses for %s: %s", i.Name, err)
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			// process IP address
			if ip != nil && !ip.IsLoopback() && !ip.IsUnspecified() {
				ipList = append(ipList, ip.String())
			}
		}
	}

	if includeHostname {
		hn, err := os.Hostname()
		if err == nil && hn != "" && !strings.HasPrefix(hn, "localhost") {
			ipList = append(ipList, hn)
		}
	}

	return ipList, nil
}

func inArray(needle string, stack ...string) (has bool) {
	for i := range stack {
		if has = needle == stack[i]; has {
			break
		}
	}
	return
}

// IsPxDisabled reports if PX is disabled on this node.
func IsPxDisabled(n *v1.Node) bool {
	if lb, has := n.GetLabels()[enablementKey]; has {
		lb = strings.ToLower(lb)
		return inArray(lb, disabledLabels...)
	}
	logrus.Debugf("No px-enabled label found on node %s - assuming 'enabled'", n.GetName())
	return false
}

// IsUninstallRequested reports if PX should uninstall on this node.
func IsUninstallRequested(n *v1.Node) bool {
	if lb, has := n.GetLabels()[enablementKey]; has {
		lb = strings.ToLower(lb)
		return inArray(lb, disabledLabels[1:]...)
	}
	return false
}

// DisablePx will replace force-set label to "false", thus triggering the K8s uninstall
func DisablePx(n *v1.Node) error {
	lb, _ := n.GetLabels()[enablementKey]
	lb = strings.ToLower(lb)
	logrus.Warnf("Resetting k8s label '%s=%s' to '%s' -- expect cleanup by k8s",
		enablementKey, lb, disabledLabels[0])
	return k8s.Instance().AddLabelOnNode(n.GetName(), enablementKey, disabledLabels[0])
}

// GetServiceRequest returns the state of the "px/service" label
func GetServiceRequest(n *v1.Node) string {
	if lb, has := n.GetLabels()[serviceKey]; has {
		return strings.ToLower(lb)
	}
	logrus.Debugf("No service request on node %s", n.GetName())
	return ""
}

// RemoveServiceLabel deletes the operations label off the node
func RemoveServiceLabel(n *v1.Node) error {
	logrus.Infof("Removing k8s label %s=%s", serviceKey, GetServiceRequest(n))
	return k8s.Instance().RemoveLabelOnNode(n.GetName(), serviceKey)
}

// FindMyNode finds LOCAL Node from Kubernetes env.
func FindMyNode() (*v1.Node, error) {
	ipList, err := GetLocalIPList(true)
	if err != nil {
		return nil, fmt.Errorf("Could not find my IPs/Hostname: %s", err)
	}
	return k8s.Instance().SearchNodeByAddresses(ipList)
}
