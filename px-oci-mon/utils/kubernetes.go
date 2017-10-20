package utils

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/Sirupsen/logrus"
	"k8s.io/client-go/1.5/kubernetes"
	"k8s.io/client-go/1.5/pkg/api"
	k8s_types "k8s.io/client-go/1.5/pkg/api/v1"
	"k8s.io/client-go/1.5/pkg/labels"
	"k8s.io/client-go/1.5/pkg/selection"
	"k8s.io/client-go/1.5/pkg/util/sets"
	"k8s.io/client-go/1.5/rest"
)

const (
	hostnameKey   = "kubernetes.io/hostname"
	enablementKey = "px/enabled"
	configKey     = "px/config"
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

// K8sClient is a simple wrapper around the Kubernetes Client API, with some extended functionality
type K8sClient struct {
	*kubernetes.Clientset
}

// K8sNode is a simple wrapper around the Kubernetes Node struct, with some extended functionality
type K8sNode struct {
	*k8s_types.Node
}

// IsPxEnabled reports if PX is enabled on this node.
func (n *K8sNode) IsPxEnabled() bool {
	if lb, has := n.GetLabels()[enablementKey]; has {
		lb = strings.ToLower(lb)
		return lb == "true" || lb == "yes" || lb == "1" || lb == "enabled"
	}
	logrus.Debugf("No px-enabled label found on node %s - assuming 'enabled'", n.GetName())
	return true
}

// GetExtraConfig extracts configuration customization on this node.
func (n *K8sNode) GetExtraConfig() []string {
	if lb, has := n.GetLabels()[configKey]; has {
		return strings.Fields(lb)
	}
	return []string{}
}

// NewK8sClient creates an instance of a new Kubernetes client
func NewK8sClient() (*K8sClient, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	kc, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	client := &K8sClient{kc}

	if vi, err := client.ServerVersion(); err == nil {
		logrus.Infof("Connected Kubernetes Server version: %+v", vi)
	} else {
		return nil, err
	}

	return client, nil
}

// FindNode finds a Node from Kubernetes env, based on a given list of IPs (and Hostname).
func (c *K8sClient) FindNode(ipList []string) (*K8sNode, error) {
	// first we list the Nodes
	nodes, err := c.Core().Nodes().List(api.ListOptions{})
	if err != nil {
		return nil, err
	}

	logrus.Debug("FindNode - locating based on IP address")
	for _, n := range nodes.Items {
		for _, addr := range n.Status.Addresses {
			switch addr.Type {
			case k8s_types.NodeExternalIP:
				fallthrough
			case k8s_types.NodeInternalIP:
				for _, ip := range ipList {
					if addr.Address == ip {
						return &K8sNode{&n}, nil
					}
				}
			}
		}
	}

	logrus.Debug("FindNode - locating based on Hostname")
	for _, n := range nodes.Items {
		for _, addr := range n.Status.Addresses {
			switch addr.Type {
			case k8s_types.NodeHostName:
				for _, ip := range ipList {
					if addr.Address == ip {
						return &K8sNode{&n}, nil
					}
				}
			}
		}
	}

	logrus.Debug("FindNode - locating based on labels")
	for _, n := range nodes.Items {
		if hn, has := n.GetLabels()[hostnameKey]; has {
			for _, ip := range ipList {
				if hn == ip {
					return &K8sNode{&n}, nil
				}
			}
		}
	}
	return nil, fmt.Errorf("Could not find requested node in Kubernetes")
}

// FindMyNode finds LOCAL Node from Kubernetes env.
func (c *K8sClient) FindMyNode() (*K8sNode, error) {
	ipList, err := GetLocalIPList(true)
	if err != nil {
		return nil, fmt.Errorf("Could not find my IPs/Hostname: %s", err)
	}
	return c.FindNode(ipList)
}

// NodeWatchFunc is a callback provided to the WatchNode function
// which is invoked when the k8s node object is changed.
type NodeWatchFunc func(node *K8sNode) error

// WatchNode sets up a watcher that listens for the changes on Node's labels.
func (c *K8sClient) WatchNode(node *K8sNode, fn NodeWatchFunc) error {
	if node == nil {
		return fmt.Errorf("No node given to watch")
	}
	hn, has := node.GetLabels()[hostnameKey]
	if !has || hn == "" {
		return fmt.Errorf("Can't find kubernetes hostname label")
	}

	return c.WatchNodeByHostname(hn, fn)
}

// WatchNodeByHostname sets up a watcher that listens for the changes on Node's labels.
func (c *K8sClient) WatchNodeByHostname(
	nodeHostname string,
	watchNodeFn NodeWatchFunc,
) error {
	// Add a selector so that we only watch for updates
	// on the provided node.
	requirement, err := labels.NewRequirement(
		hostnameKey,
		selection.DoubleEquals,
		sets.NewString(nodeHostname),
	)
	if err != nil {
		return fmt.Errorf("Failed to create a requirement for watch "+
			"with nodeID (%v): ", err)
	}

	selector := labels.NewSelector()
	selector = selector.Add(*requirement)
	listOptions := api.ListOptions{
		Watch:         true,
		LabelSelector: selector,
	}
	watchInterface, err := c.Core().Nodes().Watch(listOptions)
	if err != nil {
		return err
	}

	// fire off watch function
	go func() {
		for {
			select {
			case event, more := <-watchInterface.ResultChan():
				if !more {
					logrus.Warn("Kubernetes node watch channel closed")
					return
				}
				if k8sNode, ok := event.Object.(*k8s_types.Node); ok {
					// CHECKME: handle errors?
					watchNodeFn(&K8sNode{k8sNode})
				}
			}
		}
	}()
	return nil
}
