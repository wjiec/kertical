package portforwarding

import (
	"context"

	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	networkingv1alpha1 "github.com/wjiec/kertical/api/networking/v1alpha1"
)

const (
	// FinalizerName is the name of the finalizer added to PortForwarding resources
	// to ensure proper cleanup before deletion
	FinalizerName = "networking.kertical.com/portforwarding"
)

// ContainsFinalizer checks if the given PortForwarding resource has our finalizer.
func ContainsFinalizer(pf *networkingv1alpha1.PortForwarding) bool {
	return controllerutil.ContainsFinalizer(pf, FinalizerName)
}

// AddFinalizer adds the PortForwarding finalizer to the resource if it doesn't already exist.
//
// This ensures that cleanup operations can be performed before the resource is deleted.
func AddFinalizer(ctx context.Context, c client.Client, pf *networkingv1alpha1.PortForwarding) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var newObject networkingv1alpha1.PortForwarding
		if err := c.Get(ctx, client.ObjectKeyFromObject(pf), &newObject); err != nil {
			return client.IgnoreNotFound(err)
		}

		// examine DeletionTimestamp to determine if object is under deletion
		if newObject.ObjectMeta.DeletionTimestamp.IsZero() {
			// The object is not being deleted, so if it does not have our finalizer,
			// then let's add the finalizer and update the object. This is equivalent
			// to registering our finalizer.
			if !controllerutil.ContainsFinalizer(&newObject, FinalizerName) {
				controllerutil.AddFinalizer(&newObject, FinalizerName)
				return c.Update(ctx, &newObject)
			}
		}

		return nil
	})
}

// RemoveFinalizer removes the PortForwarding finalizer from the resource if all forwarded ports
// have been cleaned up. This allows the resource to be deleted by the Kubernetes API.
func RemoveFinalizer(ctx context.Context, c client.Client, pf *networkingv1alpha1.PortForwarding) (bool, error) {
	var requeue bool
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var newObject networkingv1alpha1.PortForwarding
		if err := c.Get(ctx, client.ObjectKeyFromObject(pf), &newObject); err != nil {
			return client.IgnoreNotFound(err)
		}

		// The object is being deleted
		if !newObject.ObjectMeta.DeletionTimestamp.IsZero() {
			requeue = false // We need to reset the flag before checking the resource status.

			if controllerutil.ContainsFinalizer(&newObject, FinalizerName) {
				// Only remove the finalizer if all forwarded ports are cleaned up
				// by checking if any node still has active forwarded ports
				for _, nodeStatus := range pf.Status.NodePortForwardingStatus {
					if len(nodeStatus.ForwardedPorts) != 0 {
						requeue = true
						log.FromContext(ctx).Info("There are still resources that have not been cleaned up")
						return nil
					}
				}

				controllerutil.RemoveFinalizer(&newObject, FinalizerName)
				return c.Update(ctx, &newObject)
			}
		}
		return nil
	})
	return requeue, err
}
