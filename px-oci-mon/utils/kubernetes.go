package utils

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/portworx/sched-ops/k8s"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
)

const (
	enablementKey = "px/enabled"
	serviceKey    = "px/service"
	pxVolsName    = "kubernetes.io/portworx-volume"
)

var (
	disabledLabels = []string{
		"false", // please keep first, keyword used w/ k8s uninstall
		"uninstall",
		"remove",
		"rm",
	}
)

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
	return k8s.Instance().FindMyNode()
}

func podsListToString(plist []v1.Pod) string {
	b, sep := bytes.Buffer{}, ""
	for _, p := range plist {
		b.WriteString(sep)
		b.WriteString(p.GetName())
		sep = ", "
	}
	return string(b.Bytes())
}

// DrainPxVolumeConsumerPods will cordon the node (prevent new PODs), and "kick out" all current PODs that use PX volumes.
// NOTE: after successful call of this function, must call uncordonNode to undo the effects
func DrainPxVolumeConsumerPods(n *v1.Node) error {
	pods, err := k8s.Instance().GetPodsUsingVolumePluginByNodeName(n.GetName(), pxVolsName)
	if err != nil {
		return fmt.Errorf("Failed to get PX volume consumer pods: %s", err)
	}

	err = k8s.Instance().CordonNode(n.GetName())
	if err != nil {
		return fmt.Errorf("Failed to cordon node: %s", err)
	}

	// schedule cleanup (if failures)
	podNames, success := "(none)", false
	defer func() {
		if !success {
			logrus.WithError(err).WithField("pods", podNames).Warnf("Failed to drain PX volume consumer pods" +
				" (rolling back changes)")
			if err2 := k8s.Instance().UnCordonNode(n.GetName()); err2 != nil {
				logrus.WithError(err).Errorf("Failed to uncordon node")
			}
		}
	}()

	if len(pods) <= 0 {
		logrus.Info("No PX volume consumer pods found.")
		success = true
		return nil
	}
	// ELSE len(pods) > 0 ...

	podNames = podsListToString(pods)
	err = k8s.Instance().DrainPodsFromNode(n.GetName(), pods)
	if err != nil {
		return fmt.Errorf("Failed to drain pods: %s", err)
	}
	success = true
	return nil
}

// CordonNode sets up the "PODs ban", so NO new PODs can be scheduled on this node.
func CordonNode(n *v1.Node) error {
	return k8s.Instance().CordonNode(n.GetName())
}

// UncordonNode removes the "PODs ban", so new PODs can be scheduled on this node.
func UncordonNode(n *v1.Node) error {
	return k8s.Instance().UnCordonNode(n.GetName())
}
