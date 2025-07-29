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

package v1alpha1

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	networkingv1alpha1 "github.com/wjiec/kertical/api/networking/v1alpha1"
	"github.com/wjiec/kertical/internal/verbosity"
)

// SetupPortForwardingWebhookWithManager registers the webhook for PortForwarding in the manager.
func SetupPortForwardingWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&networkingv1alpha1.PortForwarding{}).
		WithDefaulter(NewPortForwardingCustomDefaulter(mgr.GetScheme())).
		WithValidator(NewPortForwardingCustomValidator()).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-networking-kertical-com-v1alpha1-portforwarding,mutating=true,failurePolicy=fail,sideEffects=None,groups=networking.kertical.com,resources=portforwardings,verbs=create;update,versions=v1alpha1,name=mportforwarding-v1alpha1.kb.io,admissionReviewVersions=v1

// PortForwardingCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind PortForwarding when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type PortForwardingCustomDefaulter struct {
	scheme  *runtime.Scheme
	decoder admission.Decoder
}

// Ensure that PortForwardingCustomDefaulter implements the CustomDefaulter interface
var _ webhook.CustomDefaulter = &PortForwardingCustomDefaulter{}

// NewPortForwardingCustomDefaulter creates and returns a new instance of PortForwardingCustomDefaulter
func NewPortForwardingCustomDefaulter(scheme *runtime.Scheme) *PortForwardingCustomDefaulter {
	return &PortForwardingCustomDefaulter{
		scheme:  scheme,
		decoder: admission.NewDecoder(scheme),
	}
}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind PortForwarding.
func (d *PortForwardingCustomDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	portforwarding, ok := obj.(*networkingv1alpha1.PortForwarding)
	if !ok {
		return fmt.Errorf("expected an PortForwarding object but got %T", obj)
	}

	logger := logf.FromContext(ctx).WithName("networking-portforwarding-defaulter")
	logger.Info("defaulting for PortForwarding", "object", klog.KObj(portforwarding))
	defer func(start time.Time) {
		logger.V(verbosity.Verbose).Info("Finished defaulting for PortForwarding",
			"object", klog.KObj(portforwarding), "duration", time.Since(start))
	}(time.Now())

	return d.applyDefaults(portforwarding)
}

// applyDefaults applies default values to PortForwarding fields.
func (d *PortForwardingCustomDefaulter) applyDefaults(portforwarding *networkingv1alpha1.PortForwarding) error {
	return nil
}

// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-networking-kertical-com-v1alpha1-portforwarding,mutating=false,failurePolicy=fail,sideEffects=None,groups=networking.kertical.com,resources=portforwardings,verbs=create;update,versions=v1alpha1,name=vportforwarding-v1alpha1.kb.io,admissionReviewVersions=v1

// PortForwardingCustomValidator struct is responsible for validating the PortForwarding resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type PortForwardingCustomValidator struct{}

// Ensure that PortForwardingCustomValidator implements the CustomValidator interface
var _ webhook.CustomValidator = &PortForwardingCustomValidator{}

// NewPortForwardingCustomValidator creates and returns a new instance of PortForwardingCustomValidator
func NewPortForwardingCustomValidator() *PortForwardingCustomValidator {
	return &PortForwardingCustomValidator{}
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type PortForwarding.
func (v *PortForwardingCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	portforwarding, ok := obj.(*networkingv1alpha1.PortForwarding)
	if !ok {
		return nil, fmt.Errorf("expected an PortForwarding object but got %T", obj)
	}

	logger := logf.FromContext(ctx).WithName("networking-portforwarding-validator")
	logger.Info("validating for PortForwarding upon creation", "object", klog.KObj(portforwarding))
	defer func(start time.Time) {
		logger.V(verbosity.Verbose).Info("Finished validating for PortForwarding upon creation",
			"object", klog.KObj(portforwarding), "duration", time.Since(start))
	}(time.Now())

	return nil, v.validatePortForwarding(portforwarding)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type PortForwarding.
func (v *PortForwardingCustomValidator) ValidateUpdate(ctx context.Context, _, newObj runtime.Object) (admission.Warnings, error) {
	portforwarding, ok := newObj.(*networkingv1alpha1.PortForwarding)
	if !ok {
		return nil, fmt.Errorf("expected an PortForwarding object but got %T", newObj)
	}

	logger := logf.FromContext(ctx).WithName("networking-portforwarding-validator")
	logger.Info("validating for PortForwarding upon update", "object", klog.KObj(portforwarding))
	defer func(start time.Time) {
		logger.V(verbosity.Verbose).Info("Finished validating for PortForwarding upon update",
			"object", klog.KObj(portforwarding), "duration", time.Since(start))
	}(time.Now())

	return nil, v.validatePortForwarding(portforwarding)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type PortForwarding.
func (v *PortForwardingCustomValidator) ValidateDelete(context.Context, runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// validatePortForwarding validates the fields of an PortForwarding object.
func (v *PortForwardingCustomValidator) validatePortForwarding(portforwarding *networkingv1alpha1.PortForwarding) error {
	var allErrors field.ErrorList
	if len(portforwarding.Spec.ServiceRef.Name) == 0 {
		allErrors = append(allErrors, field.Required(field.NewPath("spec", "serviceRef", "name"), "must have a name"))
	}

	if len(allErrors) != 0 {
		return apierrors.NewInvalid(
			networkingv1alpha1.GroupVersion.WithKind("PortForwarding").GroupKind(),
			portforwarding.Name, allErrors)
	}
	return nil
}
