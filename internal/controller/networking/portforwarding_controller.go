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
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	networkingv1alpha1 "github.com/wjiec/kertical/api/networking/v1alpha1"
	portforwardingutils "github.com/wjiec/kertical/internal/controller/networking/portforwarding"
	"github.com/wjiec/kertical/internal/events"
	"github.com/wjiec/kertical/internal/kertical"
	"github.com/wjiec/kertical/internal/portforwarding"
	"github.com/wjiec/kertical/internal/verbosity"
)

const (
	PortForwardingFinalizerName = "networking.kertical.com/finalizer"
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

	forwarder portforwarding.PortForwarding
}

// newPortForwardingReconciler creates a new instance of PortForwardingReconciler.
func newPortForwardingReconciler(c client.Client, scheme *runtime.Scheme) *PortForwardingReconciler {
	return &PortForwardingReconciler{
		Client: c,
		scheme: scheme,
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
		// The object is not being deleted, so if it does not have our finalizer,
		// then let's add the finalizer and update the object. This is equivalent
		// to registering our finalizer.
		if !controllerutil.ContainsFinalizer(&instance, PortForwardingFinalizerName) {
			controllerutil.AddFinalizer(&instance, PortForwardingFinalizerName)
			if err := r.Update(ctx, &instance); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		// The object is being deleted
		if controllerutil.ContainsFinalizer(&instance, PortForwardingFinalizerName) {
			// our finalizer is present, so let's remove the port-forwarding rules first
			if err := r.removePortForwarding(ctx, &instance); err != nil {
				// if fail to remove the port forwarding here, return with error
				// so that it can be retried.
				return ctrl.Result{}, err
			}

			// remove our finalizer from the list and update it.
			controllerutil.RemoveFinalizer(&instance, PortForwardingFinalizerName)
			if err := r.Update(ctx, &instance); err != nil {
				return ctrl.Result{}, err
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

	// Calculate what ports need to be added, deleted, or left unchanged
	additions, deletions, unchanged, err := portforwardingutils.SyncForwardingPorts(instance, &service)
	if err != nil {
		return err
	}

	var newForwardedPorts []networkingv1alpha1.ForwardedPort
	newForwardedPorts = append(newForwardedPorts, unchanged...)

	// Process port forwarding rules that need to be removed
	for _, elem := range deletions {
		err = r.forwarder.RemoveForwarding(elem.Protocol, uint16(elem.SourcePort), []string{elem.TargetHost}, uint16(elem.TargetPort))
		if err != nil {
			// 删除失败的情况下, 我们需要保持错误的状态等待后续再次进行移除
			elem.State = networkingv1alpha1.PortForwardingResidual
			newForwardedPorts = append(newForwardedPorts, elem)
		}
	}

	// Process port forwarding rules that need to be added
	for _, elem := range additions {
		err = r.forwarder.AddForwarding(elem.Protocol, uint16(elem.SourcePort), []string{elem.TargetHost}, uint16(elem.TargetPort), service.Name)
		if err != nil {
			log.FromContext(ctx).Error(err, "failed to add forwarding")
			if stderrors.Is(err, portforwarding.ErrPortAlreadyInuse) {
				elem.State = networkingv1alpha1.PortForwardingConflict
			} else {
				elem.State = networkingv1alpha1.PortForwardingFailed
			}
		}
		newForwardedPorts = append(newForwardedPorts, elem)
	}

	// Update the status with the new forwarded ports list
	newPf := instance.DeepCopy()
	newPf.Status.ForwardedPorts = newForwardedPorts
	return r.Status().Update(ctx, newPf)
}

// removePortForwarding cleans up all active port forwarding for a [networkingv1alpha1.PortForwarding] resource.
func (r *PortForwardingReconciler) removePortForwarding(_ context.Context, instance *networkingv1alpha1.PortForwarding) error {
	// If the current port forwarding doesn't have any configured ports, we
	// have nothing to clean up and the resource can be deleted directly.
	for _, elem := range instance.Status.ForwardedPorts {
		err := r.forwarder.RemoveForwarding(elem.Protocol, uint16(elem.SourcePort), []string{elem.TargetHost}, uint16(elem.TargetPort))
		if err != nil {
			return err
		}
	}
	return nil
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
		WithOptions(controller.Options{
			MaxConcurrentReconciles: concurrentReconciles,
		}).
		Complete(r)
}

// CleanUp removes all port forwarding rule created by the port forwarding controller.
func (r *PortForwardingReconciler) CleanUp() error {
	return r.forwarder.Close()
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
