/*
Copyright 2025 Jayson Wang.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package networking

import (
	"context"
	stderrors "errors"
	"iter"
	"slices"
	"time"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	networkingv1alpha1 "github.com/wjiec/kertical/api/networking/v1alpha1"
	portforwardingutils "github.com/wjiec/kertical/internal/controller/networking/portforwarding"
	"github.com/wjiec/kertical/internal/events"
	"github.com/wjiec/kertical/internal/kertical"
	"github.com/wjiec/kertical/internal/portforwarding"
	"github.com/wjiec/kertical/internal/verbosity"
)

// SetupPortForwarding sets up the PortForwarding controller with the manager
func SetupPortForwarding(mgr ctrl.Manager) (func() error, error) {
	r := newPortForwardingReconciler(mgr.GetClient(), mgr.GetScheme())
	return r.CleanUp, r.SetupWithManager(mgr)
}

// PortForwardingReconciler reconciles a PortForwarding object
type PortForwardingReconciler struct {
	client.Client
	scheme *runtime.Scheme

	forwarder     portforwarding.PortForwarding
	statusUpdater portforwardingutils.StatusUpdater
}

// newPortForwardingReconciler creates a new instance of PortForwardingReconciler.
func newPortForwardingReconciler(c client.Client, scheme *runtime.Scheme) *PortForwardingReconciler {
	return &PortForwardingReconciler{
		Client: c,
		scheme: scheme,

		statusUpdater: portforwardingutils.NewStatusUpdater(c),
	}
}

// We don't need to declare the rbac permissions needed for the current control
// here, as this controller will be deployed separately and is not part of the
// controller-manager.

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.4/pkg/reconcile
func (r *PortForwardingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	defer func(start time.Time) {
		logger.V(verbosity.Verbose).Info("Finished reconcile", "duration", time.Since(start))
	}(time.Now())

	var instance networkingv1alpha1.PortForwarding
	if err := r.Get(ctx, req.NamespacedName, &instance); err != nil {
		if errors.IsNotFound(err) {
			logger.V(verbosity.Verbose).Info("resource has been deleted")
		}

		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	logger.V(verbosity.VeryVerbose).Info("Start reconciling")

	// examine DeletionTimestamp to determine if object is under deletion
	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		if err := portforwardingutils.AddFinalizer(ctx, r.Client, &instance); err != nil {
			return ctrl.Result{}, err
		}
	} else {
		// The object is being deleted
		if portforwardingutils.ContainsFinalizer(&instance) {
			// our finalizer is present, so let's remove the port-forwarding rules first
			if err := r.removePortForwarding(ctx, &instance); err != nil {
				// if fail to remove the port forwarding here, return with error
				// so that it can be retried.
				return ctrl.Result{}, err
			}

			// remove our finalizer from the list and update it.
			if requeue, err := portforwardingutils.RemoveFinalizer(ctx, r.Client, &instance); err != nil || requeue {
				return ctrl.Result{Requeue: requeue}, err
			}
		}

		// Stop reconciliation as the item is being deleted
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, r.syncPortForwarding(ctx, &instance)
}

// syncPortForwarding reconciles the desired port forwarding configuration with the current state
func (r *PortForwardingReconciler) syncPortForwarding(ctx context.Context, instance *networkingv1alpha1.PortForwarding) error {
	var service corev1.Service
	if err := r.Get(ctx, portforwardingutils.ServiceObjectKey(instance), &service); err != nil {
		return err
	}

	endpointSlices, err := r.listOwnedEndpointSlices(ctx, &service)
	if err != nil {
		return err
	}

	// Calculate what ports need to be added, deleted, or left unchanged
	additions, deletions, unchanged, errs := portforwardingutils.SyncForwardingPorts(instance, &service, endpointSlices)
	log.FromContext(ctx).V(verbosity.Verbose).Info("calc portforwarding diff", "additions", additions,
		"deletions", deletions, "unchanged", unchanged, "errs", errs)

	newForwardedPorts := slices.Clone(errs)

	// we still need to ensure they're configured correctly
	for _, elem := range unchanged {
		if sourcePort := uint16(elem.SourcePort.IntValue()); sourcePort > 0 {
			err = r.forwarder.AddForwarding(elem.Protocol, sourcePort, elem.TargetHosts, uint16(elem.TargetPort), service.Name)
			if err != nil {
				if !stderrors.Is(err, portforwarding.ErrPortAlreadyForwarded) {
					log.FromContext(ctx).Error(err, "failed to syncing unchanged forwarding")
					elem.State = networkingv1alpha1.PortForwardingFailed
				}
			} else {
				elem.State = networkingv1alpha1.PortForwardingReady
			}
		} else {
			elem.State = networkingv1alpha1.PortForwardingUnknown
		}

		newForwardedPorts = append(newForwardedPorts, elem)
	}

	// Process port forwarding rules that need to be removed
	for _, elem := range deletions {
		if sourcePort := uint16(elem.SourcePort.IntValue()); sourcePort > 0 {
			err = r.forwarder.RemoveForwarding(elem.Protocol, uint16(elem.SourcePort.IntVal), elem.TargetHosts, uint16(elem.TargetPort))
			if err != nil && !stderrors.Is(err, portforwarding.ErrPortNotForwarded) {
				log.FromContext(ctx).Error(err, "failed to remove forwarding")
				// If deletion fails, we need to maintain the error status
				// and wait for subsequent removal attempts.
				elem.State = networkingv1alpha1.PortForwardingResidual
				newForwardedPorts = append(newForwardedPorts, elem)
			}
		}
	}

	// Process port forwarding rules that need to be added
	for _, elem := range additions {
		err = r.forwarder.AddForwarding(elem.Protocol, uint16(elem.SourcePort.IntValue()), elem.TargetHosts, uint16(elem.TargetPort), service.Name)
		if err != nil {
			log.FromContext(ctx).Error(err, "failed to adding additions forwarding")
			if stderrors.Is(err, portforwarding.ErrPortAlreadyForwarded) {
				elem.State = networkingv1alpha1.PortForwardingConflict
			} else if stderrors.Is(err, portforwarding.ErrPortAlreadyInuse) {
				elem.State = networkingv1alpha1.PortForwardingRejected
			} else {
				elem.State = networkingv1alpha1.PortForwardingFailed
			}
		}
		newForwardedPorts = append(newForwardedPorts, elem)
	}

	// Update the status with the new forwarded ports list
	slices.SortFunc(newForwardedPorts, func(a, b networkingv1alpha1.ForwardedPort) int {
		return a.SourcePort.IntValue() - b.SourcePort.IntValue()
	})
	return r.statusUpdater.PatchNodeForwardedStatus(ctx, instance, kertical.NodeName(), newForwardedPorts)
}

// removePortForwarding cleans up all active port forwarding for a [networkingv1alpha1.PortForwarding] resource.
func (r *PortForwardingReconciler) removePortForwarding(ctx context.Context, instance *networkingv1alpha1.PortForwarding) error {
	// If the current port forwarding doesn't have any configured ports, we
	// have nothing to clean up and the resource can be deleted directly.
	for _, nodeStatus := range instance.Status.NodePortForwardingStatus {
		if nodeStatus.NodeName == kertical.NodeName() && len(nodeStatus.ForwardedPorts) > 0 {
			for _, elem := range nodeStatus.ForwardedPorts {
				if sourcePort := uint16(elem.SourcePort.IntValue()); sourcePort > 0 {
					err := r.forwarder.RemoveForwarding(elem.Protocol, sourcePort, elem.TargetHosts, uint16(elem.TargetPort))
					if err != nil && !stderrors.Is(err, portforwarding.ErrPortNotForwarded) {
						return err
					}
				}
			}

			// After removing all forwarded ports on this node, update the node status
			return r.statusUpdater.PatchNodeForwardedStatus(ctx, instance, kertical.NodeName(), portforwardingutils.ForwardedPorts{})
		}
	}

	return nil
}

// listOwnedEndpointSlices retrieves all [*discoveryv1.EndpointSlice] owned by a Service.
func (r *PortForwardingReconciler) listOwnedEndpointSlices(ctx context.Context, svc *corev1.Service) ([]*discoveryv1.EndpointSlice, error) {
	var endpointSliceList discoveryv1.EndpointSliceList
	err := r.List(ctx, &endpointSliceList,
		client.UnsafeDisableDeepCopy,
		client.InNamespace(svc.Namespace),
		client.MatchingFields{portforwardingutils.IndexServiceOwnerReference: svc.Name},
	)
	if err != nil {
		return nil, err
	}

	var endpointSlices []*discoveryv1.EndpointSlice
	for i := range endpointSliceList.Items {
		endpointSlice := &endpointSliceList.Items[i]
		if endpointSlice.DeletionTimestamp == nil {
			endpointSlices = append(endpointSlices, endpointSlice)
		}
	}

	return endpointSlices, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PortForwardingReconciler) SetupWithManager(mgr ctrl.Manager) (err error) {
	if r.forwarder, err = portforwarding.New(kertical.ControllerName()); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named("networking-portforwarding").
		For(&networkingv1alpha1.PortForwarding{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&corev1.Service{}, events.Referenced[*corev1.Service](r.resolveReferencedPortForwarding)).
		Watches(&discoveryv1.EndpointSlice{}, events.Referenced[*discoveryv1.EndpointSlice](r.resolveIndirectPortForwarding)).
		WithOptions(controller.Options{}).
		Complete(r)
}

// CleanUp removes all port forwarding rule created by the port forwarding controller.
func (r *PortForwardingReconciler) CleanUp() error {
	return r.forwarder.Close()
}

// resolveIndirectPortForwarding maps an [*discoveryv1.EndpointSlice] to the PortForwarding resources
// that need reconciliation when the [*discoveryv1.EndpointSlice] changes.
func (r *PortForwardingReconciler) resolveIndirectPortForwarding(ctx context.Context, es *discoveryv1.EndpointSlice) iter.Seq[ctrl.Request] {
	if serviceName, found := es.Labels[discoveryv1.LabelServiceName]; found {
		return r.resolveReferencedPortForwarding(ctx, &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: es.Namespace,
			},
		})
	}
	return func(yield func(ctrl.Request) bool) {}
}

// resolveReferencedPortForwarding finds all PortForwarding resources in the same namespace
// that reference the given Service and returns them as a sequence of reconciliation requests.
func (r *PortForwardingReconciler) resolveReferencedPortForwarding(ctx context.Context, svc *corev1.Service) iter.Seq[ctrl.Request] {
	var portForwardingList networkingv1alpha1.PortForwardingList
	if err := r.List(ctx, &portForwardingList, client.InNamespace(svc.Namespace)); err != nil {
		log.FromContext(ctx).Error(err, "failed to list PortForwarding in namespace")
		return nil
	}

	return func(yield func(ctrl.Request) bool) {
		for _, portForwarding := range portForwardingList.Items {
			if portForwarding.Spec.ServiceRef.Name == svc.Name {
				var namespacedName = types.NamespacedName{
					Namespace: svc.Namespace,
					Name:      portForwarding.Name,
				}

				if !yield(ctrl.Request{NamespacedName: namespacedName}) {
					return
				}
			}
		}
	}
}
