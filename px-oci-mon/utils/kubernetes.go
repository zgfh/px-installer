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
	enablementKey            = "px/enabled"
	serviceKey               = "px/service"
	pxStorageProvisionerName = "kubernetes.io/portworx-volume"
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
// PARAMS: K8s Node (self) where to run the command, and bool-flag specifying if all PX-dependent nodes should be drained,
// or only the managed ones (note only managed pods are guaranteed to restart elsewhere).
// NOTE: after successful call of this function, must call uncordonNode to undo the effects
func DrainPxVolumeConsumerPods(n *v1.Node, drainAllPxDepPods bool) error {
	k8si := k8s.Instance()
	pods, err := k8si.GetPodsUsingVolumePluginByNodeName(n.GetName(), pxStorageProvisionerName)
	if err != nil {
		return fmt.Errorf("Failed to get PX consumer pods: %s", err)
	}

	// should we filter out only managed pods?
	podNames := podsListToString(pods)
	if !drainAllPxDepPods && len(pods) > 0 {
		newPods := make([]v1.Pod, 0, len(pods))
		for _, p := range pods {
			if k8si.IsPodBeingManaged(p) {
				newPods = append(newPods, p)
			}
		}
		if len(pods) != len(newPods) {
			oldPodNames := podNames
			pods = newPods
			podNames = podsListToString(pods)
			logrus.Infof("Reduced list of PX consumer pods from '%s' to '%s'", oldPodNames, podNames)
		}
	}

	if len(pods) <= 0 {
		logrus.Info("No PX consumer pods found.")
		return nil
	}
	// ELSE len(pods) > 0 ... we have extra work to do

	podNames = podsListToString(pods)
	err = k8si.DrainPodsFromNode(n.GetName(), pods)
	if err != nil {
		logrus.WithError(err).WithField("pods", podNames).Warnf("Failed to drain PX volume consumer pods")
		err = fmt.Errorf("Failed to drain pods: %s", err)
	} else {
		logrus.WithField("pods", podNames).Warnf("PX consumer pods drained successfully" +
			" - node cordon in effect.")
	}
	return err
}

// CordonNode sets up the "PODs ban", so NO new PODs can be scheduled on this node.
func CordonNode(n *v1.Node) error {
	return k8s.Instance().CordonNode(n.GetName())
}

// UncordonNode removes the "PODs ban", so new PODs can be scheduled on this node.
func UncordonNode(n *v1.Node) error {
	return k8s.Instance().UnCordonNode(n.GetName())
}
