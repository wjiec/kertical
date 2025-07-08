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
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	networkingv1alpha1 "github.com/wjiec/kertical/api/networking/v1alpha1"
	"github.com/wjiec/kertical/internal/events"
	"github.com/wjiec/kertical/internal/kertical"
	"github.com/wjiec/kertical/internal/portforwarding"
	"github.com/wjiec/kertical/internal/verbosity"
)

// SetupPortForwarding sets up the PortForwarding controller with the manager
func SetupPortForwarding(mgr ctrl.Manager) error {
	return newPortForwardingReconciler(mgr.GetClient(), mgr.GetScheme()).
		SetupWithManager(mgr)
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

	return ctrl.Result{}, nil
}

func (r *PortForwardingReconciler) syncPortForwarding(ctx context.Context, instance *networkingv1alpha1.PortForwarding) error {
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PortForwardingReconciler) SetupWithManager(mgr ctrl.Manager) (err error) {
	if r.forwarder, err = portforwarding.Factory(kertical.ControllerName()); err != nil {
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
