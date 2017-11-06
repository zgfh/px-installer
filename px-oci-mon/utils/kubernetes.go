package utils

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/portworx/sched-ops/k8s"
	"github.com/sirupsen/logrus"
	k8s_types "k8s.io/client-go/pkg/api/v1"
)

const (
	enablementKey = "px/enabled"
	serviceKey    = "px/service"
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

// IsPxEnabled reports if PX is enabled on this node.
func IsPxEnabled(n *k8s_types.Node) bool {
	if lb, has := n.GetLabels()[enablementKey]; has {
		lb = strings.ToLower(lb)
		return lb == "true" || lb == "yes" || lb == "1" || lb == "enabled"
	}
	logrus.Debugf("No px-enabled label found on node %s - assuming 'enabled'", n.GetName())
	return true
}

// GetServiceRequest returns the state of the "px/service" label
func GetServiceRequest(n *k8s_types.Node) string {
	if lb, has := n.GetLabels()[serviceKey]; has {
		return strings.ToLower(lb)
	}
	logrus.Debugf("No operation requested on node %s", n.GetName())
	return ""
}

// RemoveServiceLabel deletes the operations label off the node
func RemoveServiceLabel(n *k8s_types.Node) error {
	return k8s.Instance().RemoveLabelOnNode(n.GetName(), serviceKey)
}

// FindMyNode finds LOCAL Node from Kubernetes env.
func FindMyNode() (*k8s_types.Node, error) {
	ipList, err := GetLocalIPList(true)
	if err != nil {
		return nil, fmt.Errorf("Could not find my IPs/Hostname: %s", err)
	}
	return k8s.Instance().SearchNodeByAddresses(ipList)
}
