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
	"slices"
	"time"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	networkingv1alpha1 "github.com/wjiec/kertical/api/networking/v1alpha1"
	"github.com/wjiec/kertical/internal/controller/networking/externalproxy"
	"github.com/wjiec/kertical/internal/events"
	"github.com/wjiec/kertical/internal/expectations"
	"github.com/wjiec/kertical/internal/fieldindex"
	"github.com/wjiec/kertical/internal/kertical"
	"github.com/wjiec/kertical/internal/verbosity"
)

// SetupExternalProxy sets up the ExternalProxy controller with the manager
func SetupExternalProxy(mgr ctrl.Manager) error {
	return newExternalProxyReconciler(mgr.GetClient(), mgr.GetScheme()).
		SetupWithManager(mgr)
}

// ExternalProxyReconciler handles reconciliation of the ExternalProxy resources
type ExternalProxyReconciler struct {
	client.Client
	scheme *runtime.Scheme

	statusUpdater externalproxy.StatusUpdater

	serviceExpectation   expectations.ControllerExpectations
	endpointsExpectation expectations.ControllerExpectations
	ingressExpectation   expectations.ControllerExpectations
}

// newExternalProxyReconciler creates a new instance of ExternalProxyReconciler
func newExternalProxyReconciler(c client.Client, scheme *runtime.Scheme) *ExternalProxyReconciler {
	r := &ExternalProxyReconciler{
		Client:        c,
		scheme:        scheme,
		statusUpdater: externalproxy.NewStatusUpdater(c),
	}

	r.serviceExpectation = expectations.NewControllerExpectations()
	r.endpointsExpectation = expectations.NewControllerExpectations()
	r.ingressExpectation = expectations.NewControllerExpectations()

	return r
}

// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.k8s.io,resources=endpointslices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.kertical.com,resources=externalproxies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.kertical.com,resources=externalproxies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.kertical.com,resources=externalproxies/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.1/pkg/reconcile
func (r *ExternalProxyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	ctx = expectations.WithControllerKey(ctx, req.String())

	logger := log.FromContext(ctx)
	defer func(start time.Time) {
		logger.V(verbosity.Verbose).Info("Finished reconcile", "duration", time.Since(start))
	}(time.Now())

	var instance networkingv1alpha1.ExternalProxy
	if err := r.Get(ctx, req.NamespacedName, &instance); err != nil {
		if errors.IsNotFound(err) {
			logger.V(verbosity.Verbose).Info("resource has been deleted")
			r.serviceExpectation.DeleteExpectations(req.String())
		}

		// we'll ignore not-found errors, since they can't be fixed by an immediate
		// requeue (we'll need to wait for a new notification), and we can get them
		// on deleted requests.
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	logger.V(verbosity.VeryVerbose).Info("Start reconciling")

	// if Service expectations have not satisfied yet, just skip this reconcile.
	if satisfied, unsatisfiedDuration := r.serviceExpectation.SatisfiedExpectations(req.String()); !satisfied {
		if unsatisfiedDuration >= expectations.ExpectationTimeout {
			logger.Info("expectation unsatisfied overtime", "overtime", unsatisfiedDuration)
			return reconcile.Result{}, nil
		}

		// requeue the request to be processed after the remaining timeout.
		return reconcile.Result{RequeueAfter: expectations.ExpectationTimeout - unsatisfiedDuration}, nil
	}

	// ensure services related to the ExternalProxy are reconciled
	if err := r.SyncService(ctx, &instance); err != nil {
		return ctrl.Result{}, err
	}

	newStatus := &networkingv1alpha1.ExternalProxyStatus{ServiceName: instance.Spec.Service.Name}
	if err := r.statusUpdater.UpdateStatus(ctx, &instance, newStatus); err != nil {
		return ctrl.Result{}, err
	}

	// if EndpointSlice expectation have not satisfied yet, just skip this reconcile.
	if satisfied, unsatisfiedDuration := r.endpointsExpectation.SatisfiedExpectations(req.String()); !satisfied {
		if unsatisfiedDuration >= expectations.ExpectationTimeout {
			logger.Info("expectation unsatisfied overtime", "overtime", unsatisfiedDuration)
			return reconcile.Result{}, nil
		}

		// requeue the request to be processed after the remaining timeout.
		return reconcile.Result{RequeueAfter: expectations.ExpectationTimeout - unsatisfiedDuration}, nil
	}

	// ensure EndpointSlices related to the ExternalProxy are reconciled
	if err := r.SyncEndpointSlices(ctx, &instance); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// if Ingress expectations have not satisfied yet, just skip this reconcile.
	if satisfied, unsatisfiedDuration := r.ingressExpectation.SatisfiedExpectations(req.String()); !satisfied {
		if unsatisfiedDuration >= expectations.ExpectationTimeout {
			logger.Info("expectation unsatisfied overtime", "overtime", unsatisfiedDuration)
			return reconcile.Result{}, nil
		}

		// requeue the request to be processed after the remaining timeout.
		return reconcile.Result{RequeueAfter: expectations.ExpectationTimeout - unsatisfiedDuration}, nil
	}

	// ensure Ingress related to the ExternalProxy are reconciled
	if err := r.SyncIngress(ctx, &instance); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	newStatus = &networkingv1alpha1.ExternalProxyStatus{
		Ready:              true,
		ServiceName:        instance.Spec.Service.Name,
		ObservedGeneration: instance.Generation,
	}
	return ctrl.Result{}, r.statusUpdater.UpdateStatus(ctx, &instance, newStatus)
}

// SyncService ensures that the Services associated with an ExternalProxy instance are correctly
// configured and reconciled.
func (r *ExternalProxyReconciler) SyncService(ctx context.Context, instance *networkingv1alpha1.ExternalProxy) error {
	// Retrieve Services associated with the controller.
	ownedServices, err := r.listOwnedServices(ctx, instance)
	if err != nil {
		return err
	}

	var activatedService *corev1.Service
	// Since the name of an existing Service can't be modified, we prefer to find a matching service
	// and update it. Otherwise, we delete the mismatched service and create a new one.
	for _, ownedService := range ownedServices {
		if ownedService.Name == instance.Spec.Service.Name {
			activatedService = ownedService
		} else {
			r.serviceExpectation.Expect(expectations.ControllerKeyFromCtx(ctx), expectations.ActionDeletions, ownedService.Name)
			if err = r.Delete(ctx, ownedService); err != nil {
				r.serviceExpectation.Observe(expectations.ControllerKeyFromCtx(ctx), expectations.ActionDeletions, ownedService.Name)
				return err
			}
		}
	}

	newService := externalproxy.NewService(instance)
	// If no service is currently activated or owned, proceed to create a new one
	// for the ExternalProxy instance.
	if len(ownedServices) == 0 || activatedService == nil {
		if err = ctrl.SetControllerReference(instance, newService, r.scheme); err != nil {
			return err
		}

		r.serviceExpectation.Expect(expectations.ControllerKeyFromCtx(ctx), expectations.ActionCreations, newService.Name)
		if err = r.Create(ctx, newService); err != nil {
			r.serviceExpectation.Observe(expectations.ControllerKeyFromCtx(ctx), expectations.ActionCreations, newService.Name)
			return err
		}

		// Return successfully upon service creation completion
		return nil
	}

	// Check if the currently activated service needs reconciling based on its revision status.
	if !externalproxy.ShouldReconcileResource(instance, activatedService) {
		return nil
	}

	// Update the existing activated service to match the ExternalProxy's desired state.
	copyService := activatedService.DeepCopy()
	copyService.Labels = newService.Labels
	copyService.Annotations = newService.Annotations
	copyService.Spec.Type = instance.Spec.Service.Type
	copyService.Spec.Ports = slices.Clone(instance.Spec.Service.Ports)
	return r.Update(ctx, copyService)
}

// listOwnedServices retrieves a list of Services that are owned by a given ExternalProxy instance.
//
// It only includes services that are not marked for deletion.
func (r *ExternalProxyReconciler) listOwnedServices(ctx context.Context, instance *networkingv1alpha1.ExternalProxy) ([]*corev1.Service, error) {
	var serviceList corev1.ServiceList
	if err := r.listOwnedResources(ctx, &serviceList, instance); err != nil {
		return nil, err
	}

	services := make([]*corev1.Service, 0, len(serviceList.Items))
	for i := range serviceList.Items {
		service := &serviceList.Items[i]
		if service.DeletionTimestamp == nil {
			services = append(services, service)
		}
	}

	slices.SortStableFunc(services, func(a, b *corev1.Service) int {
		return a.CreationTimestamp.Compare(b.CreationTimestamp.Time)
	})

	return services, nil
}

// SyncEndpointSlices ensures that the EndpointSlices associated with an ExternalProxy instance are properly reconciled.
func (r *ExternalProxyReconciler) SyncEndpointSlices(ctx context.Context, instance *networkingv1alpha1.ExternalProxy) error {
	var ownerService corev1.Service
	if err := r.Get(ctx, externalproxy.ServiceKey(instance), &ownerService); err != nil {
		return err
	}

	// Retrieve EndpointSlices associated with the services.
	ownerEndpointSlices, err := r.listOwnedEndpointSlices(ctx, &ownerService)
	if err != nil {
		return err
	}

	endpointSliceIndex := make(map[string]int)
	for idx, elem := range ownerEndpointSlices {
		endpointSliceIndex[elem.Name] = idx
	}

	for _, desiredEndpointSlice := range externalproxy.NewEndpointSlices(instance) {
		if idx, found := endpointSliceIndex[desiredEndpointSlice.Name]; found {
			delete(endpointSliceIndex, desiredEndpointSlice.Name)
			// Update the EndpointSlice only if the existing EndpointSlice differs from the desired state
			if externalproxy.ShouldReconcileResource(instance, ownerEndpointSlices[idx]) {
				copyEndpointSlice := ownerEndpointSlices[idx].DeepCopy()
				copyEndpointSlice.Labels = desiredEndpointSlice.Labels
				copyEndpointSlice.Annotations = desiredEndpointSlice.Annotations
				copyEndpointSlice.AddressType = desiredEndpointSlice.AddressType
				copyEndpointSlice.Endpoints = desiredEndpointSlice.Endpoints
				copyEndpointSlice.Ports = desiredEndpointSlice.Ports

				if err = r.Update(ctx, copyEndpointSlice); err != nil {
					return err
				}
			}
		} else {
			// We should create a new EndpointSlice and set the service as its owner.
			//	see https://kubernetes.io/docs/concepts/services-networking/endpoint-slices/#ownership for more details
			if err = ctrl.SetControllerReference(&ownerService, desiredEndpointSlice, r.scheme); err != nil {
				return err
			}

			r.endpointsExpectation.Expect(expectations.ControllerKeyFromCtx(ctx), expectations.ActionCreations, desiredEndpointSlice.Name)
			if err = r.Create(ctx, desiredEndpointSlice); err != nil {
				r.endpointsExpectation.Observe(expectations.ControllerKeyFromCtx(ctx), expectations.ActionCreations, desiredEndpointSlice.Name)
				return err
			}
		}
	}

	// Delete EndpointSlices that are no longer needed
	for _, idx := range endpointSliceIndex {
		endpointSlice := ownerEndpointSlices[idx]
		r.endpointsExpectation.Expect(expectations.ControllerKeyFromCtx(ctx), expectations.ActionDeletions, endpointSlice.Name)
		if err = r.Delete(ctx, endpointSlice); err != nil {
			r.endpointsExpectation.Observe(expectations.ControllerKeyFromCtx(ctx), expectations.ActionDeletions, endpointSlice.Name)
			return err
		}
	}

	return nil
}

// listOwnedEndpointSlices retrieves a list of EndpointSlices owned by a given Service.
func (r *ExternalProxyReconciler) listOwnedEndpointSlices(ctx context.Context, service *corev1.Service) ([]*discoveryv1.EndpointSlice, error) {
	var endpointSliceList discoveryv1.EndpointSliceList
	if err := r.listOwnedResources(ctx, &endpointSliceList, service); err != nil {
		return nil, err
	}

	endpointSlices := make([]*discoveryv1.EndpointSlice, 0, len(endpointSliceList.Items))
	for i := range endpointSliceList.Items {
		endpointSlice := &endpointSliceList.Items[i]
		if endpointSlice.DeletionTimestamp == nil {
			endpointSlices = append(endpointSlices, endpointSlice)
		}
	}

	slices.SortStableFunc(endpointSlices, func(a, b *discoveryv1.EndpointSlice) int {
		return a.CreationTimestamp.Compare(b.CreationTimestamp.Time)
	})

	return endpointSlices, nil
}

// SyncIngress ensures that the Ingresses associated with an ExternalProxy instance are properly reconciled.
func (r *ExternalProxyReconciler) SyncIngress(ctx context.Context, instance *networkingv1alpha1.ExternalProxy) error {
	ingresses, err := r.listOwnedIngresses(ctx, instance)
	if err != nil {
		return err
	}

	// If the Ingress configuration is removed from the ExternalProxy, delete all existing ingresses.
	if instance.Spec.Ingress == nil {
		for _, ingress := range ingresses {
			r.ingressExpectation.Expect(expectations.ControllerKeyFromCtx(ctx), expectations.ActionDeletions, ingress.Name)
			if err = r.Delete(ctx, ingress); err != nil {
				r.ingressExpectation.Observe(expectations.ControllerKeyFromCtx(ctx), expectations.ActionDeletions, ingress.Name)
				return err
			}
		}

		return nil
	}

	// Find the activated one and make others to deletion.
	var activatedIngress *networkingv1.Ingress
	for _, ingress := range ingresses {
		if ingress.Name == instance.Spec.Ingress.Name {
			activatedIngress = ingress
		} else {
			r.ingressExpectation.Expect(expectations.ControllerKeyFromCtx(ctx), expectations.ActionDeletions, ingress.Name)
			if err = r.Delete(ctx, ingress); err != nil {
				r.ingressExpectation.Observe(expectations.ControllerKeyFromCtx(ctx), expectations.ActionDeletions, ingress.Name)
				return err
			}
		}
	}

	newIngress := externalproxy.NewIngress(instance)
	// If no ingress is currently activated or owned, proceed to create a new one
	// for the ExternalProxy instance.
	if len(ingresses) == 0 || activatedIngress == nil {
		if err = ctrl.SetControllerReference(instance, newIngress, r.scheme); err != nil {
			return err
		}

		r.ingressExpectation.Expect(expectations.ControllerKeyFromCtx(ctx), expectations.ActionCreations, newIngress.Name)
		if err = r.Create(ctx, newIngress); err != nil {
			r.ingressExpectation.Observe(expectations.ControllerKeyFromCtx(ctx), expectations.ActionCreations, newIngress.Name)
			return err
		}

		// Return successfully upon service creation completion
		return nil
	}

	// Check if the currently activated service needs reconciling based on its revision status.
	if !externalproxy.ShouldReconcileResource(instance, activatedIngress) {
		return nil
	}

	// Update the existing activated ingress to match the ExternalProxy's desired state.
	copyIngress := activatedIngress.DeepCopy()
	copyIngress.Labels = newIngress.Labels
	copyIngress.Annotations = newIngress.Annotations
	copyIngress.Spec = newIngress.Spec
	return r.Update(ctx, copyIngress)
}

// listOwnedIngresses retrieves a list of Ingresses owned by a given ExternalProxy instance.
func (r *ExternalProxyReconciler) listOwnedIngresses(ctx context.Context, instance *networkingv1alpha1.ExternalProxy) ([]*networkingv1.Ingress, error) {
	var ingressList networkingv1.IngressList
	if err := r.listOwnedResources(ctx, &ingressList, instance); err != nil {
		return nil, err
	}

	ingresses := make([]*networkingv1.Ingress, 0, len(ingressList.Items))
	for i := range ingressList.Items {
		ingress := &ingressList.Items[i]
		if ingress.DeletionTimestamp == nil {
			ingresses = append(ingresses, ingress)
		}
	}

	slices.SortStableFunc(ingresses, func(a, b *networkingv1.Ingress) int {
		return a.CreationTimestamp.Compare(b.CreationTimestamp.Time)
	})

	return ingresses, nil
}

// listOwnedResources retrieves owned resources for a given ExternalProxy instance.
func (r *ExternalProxyReconciler) listOwnedResources(ctx context.Context, list client.ObjectList, instance client.Object) error {
	return r.List(ctx, list,
		client.UnsafeDisableDeepCopy,
		client.InNamespace(instance.GetNamespace()),
		client.MatchingFields{fieldindex.IndexOwnerReferenceUID: string(instance.GetUID())},
	)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ExternalProxyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	serviceControllerRefResolver := events.GvkResolver[*corev1.Service](apiVersion, externalproxy.Kind)
	ingressControllerRefResolver := events.GvkResolver[*networkingv1.Ingress](apiVersion, externalproxy.Kind)

	return ctrl.NewControllerManagedBy(mgr).
		Named("networking-externalproxy").
		For(&networkingv1alpha1.ExternalProxy{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&corev1.Service{}, events.OwnedWithExpectation(r.serviceExpectation, serviceControllerRefResolver)).
		Watches(&discoveryv1.EndpointSlice{}, events.OwnedWithExpectation(r.endpointsExpectation, r.endpointSliceControllerRefResolver(serviceControllerRefResolver))).
		Watches(&networkingv1.Ingress{}, events.OwnedWithExpectation(r.ingressExpectation, ingressControllerRefResolver)).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: concurrentReconciles,
		}).
		Complete(r)
}

// endpointSliceControllerRefResolver creates a resolver function that resolves controller references
// for EndpointSlices based on their associated Services.
func (r *ExternalProxyReconciler) endpointSliceControllerRefResolver(svcResolver events.ControllerRefResolver[*corev1.Service]) events.ControllerRefResolver[*discoveryv1.EndpointSlice] {
	return func(ctx context.Context, object *discoveryv1.EndpointSlice) *ctrl.Request {
		if object.Labels[discoveryv1.LabelManagedBy] != kertical.ControllerName() {
			return nil
		}

		if serviceName, found := object.Labels[discoveryv1.LabelServiceName]; found {
			serviceKey := types.NamespacedName{Namespace: object.Namespace, Name: serviceName}

			var service corev1.Service
			if err := r.Get(ctx, serviceKey, &service); err != nil {
				log.FromContext(ctx).Error(err, "failed to get service for endpoint slice")
				return nil
			}

			return svcResolver(ctx, &service)
		}

		return nil
	}
}
