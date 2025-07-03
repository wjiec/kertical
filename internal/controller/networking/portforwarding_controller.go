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
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	networkingv1alpha1 "github.com/wjiec/kertical/api/networking/v1alpha1"
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
}

// newPortForwardingReconciler creates a new instance of PortForwardingReconciler.
func newPortForwardingReconciler(c client.Client, scheme *runtime.Scheme) *PortForwardingReconciler {
	return &PortForwardingReconciler{
		Client: c,
		scheme: scheme,
	}
}

// +kubebuilder:rbac:groups=networking.kertical.com,resources=portforwardings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.kertical.com,resources=portforwardings/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.kertical.com,resources=portforwardings/finalizers,verbs=update

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

// SetupWithManager sets up the controller with the Manager.
func (r *PortForwardingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("networking-portforwarding").
		For(&networkingv1alpha1.PortForwarding{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&corev1.Service{}, nil).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: concurrentReconciles,
		}).
		Complete(r)
}
