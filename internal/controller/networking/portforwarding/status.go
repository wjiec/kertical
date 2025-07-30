package portforwarding

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	networkingv1alpha1 "github.com/wjiec/kertical/api/networking/v1alpha1"
	"github.com/wjiec/kertical/internal/verbosity"
)

// StatusUpdater defines an interface for updating the status of PortForwarding objects.
type StatusUpdater interface {
	// PatchNodeForwardedStatus updates the forwarded port status for a specific node
	// in a [*networkingv1alpha1.PortForwarding] resource.
	PatchNodeForwardedStatus(context.Context, *networkingv1alpha1.PortForwarding, string, ForwardedPorts) error

	// UpdateCondition updates a condition in the PortForwarding resource's status.
	UpdateCondition(ctx context.Context, pf *networkingv1alpha1.PortForwarding, condition metav1.Condition) error
}

// realStatusUpdater is a concrete implementation of the StatusUpdater interface.
type realStatusUpdater struct {
	c client.Client
}

// NewStatusUpdater creates a new instance of StatusUpdater with the provided client.
func NewStatusUpdater(c client.Client) StatusUpdater {
	return &realStatusUpdater{c: c}
}

// PatchNodeForwardedStatus updates the forwarded port status for a specific node
// in a [*networkingv1alpha1.PortForwarding] resource.
func (su *realStatusUpdater) PatchNodeForwardedStatus(ctx context.Context, pf *networkingv1alpha1.PortForwarding, nodeName string, ports ForwardedPorts) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var newObject networkingv1alpha1.PortForwarding
		// Retrieve the latest version of the port forwarding from the API server.
		if err := su.c.Get(ctx, client.ObjectKeyFromObject(pf), &newObject); err != nil {
			return err
		}

		var nodeStatusIndex = -1
		for i, nodeStatus := range newObject.Status.NodePortForwardingStatus {
			if nodeStatus.NodeName == nodeName {
				nodeStatusIndex = i
				break
			}
		}

		if nodeStatusIndex >= 0 {
			newObject.Status.NodePortForwardingStatus[nodeStatusIndex].ForwardedPorts = ports
		} else {
			nodeStatus := networkingv1alpha1.NodePortForwardingStatus{
				NodeName:       nodeName,
				ForwardedPorts: ports,
			}
			newObject.Status.NodePortForwardingStatus = append(newObject.Status.NodePortForwardingStatus, nodeStatus)
		}

		if err := su.c.Status().Update(ctx, &newObject); err != nil {
			return err
		}

		log.FromContext(ctx).V(verbosity.Verbose).Info("patched node status", "nodeName", nodeName, "forwardedPorts", ports)
		return nil
	})
}

// UpdateCondition updates a condition in the PortForwarding resource's status.
//
// This function handles optimistic concurrency by using RetryOnConflict to
// retry the operation if the resource has been modified by another operation.
func (su *realStatusUpdater) UpdateCondition(ctx context.Context, pf *networkingv1alpha1.PortForwarding, condition metav1.Condition) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var current networkingv1alpha1.PortForwarding
		if err := su.c.Get(ctx, client.ObjectKeyFromObject(pf), &current); err != nil {
			return err
		}

		newObject := current.DeepCopy()
		if meta.SetStatusCondition(&newObject.Status.Conditions, condition) {
			patch := client.MergeFrom(&current)
			return su.c.Status().Patch(ctx, newObject, patch)
		}
		return nil
	})
}
