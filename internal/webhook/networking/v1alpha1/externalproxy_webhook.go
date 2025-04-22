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

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimachineryapivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unversionedvalidation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	apimachineryvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	networkingv1alpha1 "github.com/wjiec/kertical/api/networking/v1alpha1"
	"github.com/wjiec/kertical/internal/verbosity"
	webhookutils "github.com/wjiec/kertical/internal/webhook/utils"
	"github.com/wjiec/kertical/internal/webhook/validation"
)

// SetupExternalProxyWebhookWithManager registers the webhook for ExternalProxy in the manager.
func SetupExternalProxyWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&networkingv1alpha1.ExternalProxy{}).
		WithDefaulter(NewExternalProxyCustomDefaulter(mgr.GetScheme())).
		WithValidator(NewExternalProxyCustomValidator()).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-networking-kertical-com-v1alpha1-externalproxy,mutating=true,failurePolicy=fail,sideEffects=None,groups=networking.kertical.com,resources=externalproxies,verbs=create;update,versions=v1alpha1,name=mutating-externalproxy-v1alpha1.kertical.com,admissionReviewVersions=v1

// ExternalProxyCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind ExternalProxy when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type ExternalProxyCustomDefaulter struct {
	scheme  *runtime.Scheme
	decoder admission.Decoder

	DefaultServiceType         corev1.ServiceType
	DefaultBackendPortProtocol corev1.Protocol
	DefaultIngressPathType     networkingv1.PathType
}

// Ensure that ExternalProxyCustomDefaulter implements the CustomDefaulter interface
var _ webhook.CustomDefaulter = &ExternalProxyCustomDefaulter{}

// NewExternalProxyCustomDefaulter creates and returns a new instance of ExternalProxyCustomDefaulter
func NewExternalProxyCustomDefaulter(scheme *runtime.Scheme) *ExternalProxyCustomDefaulter {
	return &ExternalProxyCustomDefaulter{
		scheme:  scheme,
		decoder: admission.NewDecoder(scheme),

		DefaultServiceType:         corev1.ServiceTypeClusterIP,
		DefaultBackendPortProtocol: corev1.ProtocolTCP,
		DefaultIngressPathType:     networkingv1.PathTypeImplementationSpecific,
	}
}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind ExternalProxy.
func (d *ExternalProxyCustomDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	externalproxy, ok := obj.(*networkingv1alpha1.ExternalProxy)
	if !ok {
		return fmt.Errorf("expected an ExternalProxy object but got %T", obj)
	}

	logger := logf.FromContext(ctx).WithName("networking-externalproxy-defaulter")
	logger.Info("defaulting for ExternalProxy", "object", klog.KObj(externalproxy))
	defer func(start time.Time) {
		logger.V(verbosity.Verbose).Info("Finished defaulting for ExternalProxy",
			"object", klog.KObj(externalproxy), "duration", time.Since(start))
	}(time.Now())

	admissionReq, err := admission.RequestFromContext(ctx)
	if err != nil {
		return err
	}

	// 如果我们通过 kubectl apply 的方式更新对象，我们应该要为实际 apply 的对象生成默认值
	if admissionReq.Operation == admissionv1.Update {
		var updateOption metav1.UpdateOptions
		if err = d.decoder.DecodeRaw(admissionReq.Options, &updateOption); err != nil {
			return err
		}

		if updateOption.FieldManager == "kubectl-client-side-apply" {
			lastAppliedConfiguration, present := webhookutils.GetLastAppliedConfiguration(externalproxy)
			if present {
				var lastAppliedObject networkingv1alpha1.ExternalProxy
				err = d.decoder.DecodeRaw(runtime.RawExtension{Raw: []byte(lastAppliedConfiguration)}, &lastAppliedObject)
				if err != nil {
					return err
				}

				lastAppliedObject.Spec.DeepCopyInto(&externalproxy.Spec)
			}
		}
	}

	return d.applyDefaults(externalproxy)
}

// applyDefaults applies default values to ExternalProxy fields.
func (d *ExternalProxyCustomDefaulter) applyDefaults(externalproxy *networkingv1alpha1.ExternalProxy) error {
	// if the service name is not set, use the ExternalProxy's name as the default.
	if len(externalproxy.Spec.Service.Name) == 0 {
		externalproxy.Spec.Service.Name = externalproxy.Name
	}
	// if the service type is not set, use the default service type provided by the defaulter.
	if len(externalproxy.Spec.Service.Type) == 0 {
		externalproxy.Spec.Service.Type = d.DefaultServiceType
	}

	// If the port protocol is not set in the backends, TCP is used by default.
	for i := range externalproxy.Spec.Backends {
		backend := &externalproxy.Spec.Backends[i]
		for j := range backend.Ports {
			port := &backend.Ports[j]
			if port.Protocol == nil {
				port.Protocol = ptr.To(d.DefaultBackendPortProtocol)
			}
		}
	}

	// if the user has not configured the ports in the service, then we try to
	// extract the port information from the backends.
	if len(externalproxy.Spec.Service.Ports) == 0 && len(externalproxy.Spec.Backends) != 0 {
		registeredPorts := make(map[string]*discoveryv1.EndpointPort)
		for i := range externalproxy.Spec.Backends {
			backend := &externalproxy.Spec.Backends[i]
			for j := range backend.Ports {
				backendPort := &backend.Ports[j]
				if backendPort.Name != nil && len(*backendPort.Name) != 0 {
					if _, found := registeredPorts[*backendPort.Name]; !found {
						registeredPorts[*backendPort.Name] = backendPort
						externalproxy.Spec.Service.Ports = append(externalproxy.Spec.Service.Ports, corev1.ServicePort{
							Name:        *backendPort.Name,
							Protocol:    *backendPort.Protocol,
							AppProtocol: backendPort.AppProtocol,
							Port:        *backendPort.Port,
							TargetPort:  preferredTargetPort(backendPort),
						})
					}
				}
			}
		}
	}

	// if ingress is configured, then we also need to deal with the
	// default values in this section.
	if ingress := externalproxy.Spec.Ingress; ingress != nil {
		if len(ingress.Name) == 0 {
			ingress.Name = externalproxy.Name
		}

		if len(externalproxy.Spec.Service.Ports) == 1 && len(ingress.Rules) == 0 {
			ingress.DefaultBackend = &networkingv1alpha1.ExternalProxyIngressBackend{
				Port: networkingv1alpha1.ExternalProxyServiceBackendPort{
					Number: externalproxy.Spec.Service.Ports[0].Port,
				},
			}
		}

		// if there is only one port, then we can generate default HTTP traffic rules for it
		if len(externalproxy.Spec.Service.Ports) == 1 && len(ingress.Rules) != 0 {
			for i := range ingress.Rules {
				rule := &ingress.Rules[i]
				if rule.Host != "" && rule.HTTP == nil {
					rule.HTTP = &networkingv1alpha1.ExternalProxyIngressHttpRuleValue{
						Paths: []networkingv1alpha1.ExternalProxyIngressHttpPath{
							{Path: "/"},
						},
					}
				}

				if len(rule.HTTP.Paths) != 0 {
					for j := range rule.HTTP.Paths {
						path := &rule.HTTP.Paths[j]
						if path.PathType == nil {
							path.PathType = ptr.To(d.DefaultIngressPathType)
						}
						if path.Backend == nil {
							path.Backend = &networkingv1alpha1.ExternalProxyIngressBackend{}
						}

						if len(path.Backend.Port.Name) == 0 && path.Backend.Port.Number == 0 {
							path.Backend.Port.Number = externalproxy.Spec.Service.Ports[0].Port
						}
					}
				}
			}
		}
	}

	return nil
}

// preferredTargetPort prefers using the port name as the mapping criteria for services and endpoints.
func preferredTargetPort(port *discoveryv1.EndpointPort) intstr.IntOrString {
	if port.Name != nil && len(*port.Name) != 0 {
		return intstr.FromString(*port.Name)
	}
	return intstr.FromInt32(*port.Port)
}

// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-networking-kertical-com-v1alpha1-externalproxy,mutating=false,failurePolicy=fail,sideEffects=None,groups=networking.kertical.com,resources=externalproxies,verbs=create;update,versions=v1alpha1,name=validating-externalproxy-v1alpha1.kertical.com,admissionReviewVersions=v1

// ExternalProxyCustomValidator struct is responsible for validating the ExternalProxy resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type ExternalProxyCustomValidator struct {
	supportedPortProtocols sets.Set[corev1.Protocol]
	ingressValidationOpts  validation.IngressValidationOptions
}

// Ensure that ExternalProxyCustomValidator implements the CustomValidator interface
var _ webhook.CustomValidator = &ExternalProxyCustomValidator{}

// NewExternalProxyCustomValidator creates and returns a new instance of ExternalProxyCustomValidator
func NewExternalProxyCustomValidator() *ExternalProxyCustomValidator {
	return &ExternalProxyCustomValidator{
		supportedPortProtocols: sets.New(corev1.ProtocolTCP, corev1.ProtocolUDP, corev1.ProtocolSCTP),
		ingressValidationOpts: validation.IngressValidationOptions{
			AllowInvalidSecretName:       false,
			AllowInvalidWildcardHostRule: false,
		},
	}
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type ExternalProxy.
func (v *ExternalProxyCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	externalproxy, ok := obj.(*networkingv1alpha1.ExternalProxy)
	if !ok {
		return nil, fmt.Errorf("expected an ExternalProxy object but got %T", obj)
	}

	logger := logf.FromContext(ctx).WithName("networking-externalproxy-validator")
	logger.Info("validating for ExternalProxy upon creation", "object", klog.KObj(externalproxy))
	defer func(start time.Time) {
		logger.V(verbosity.Verbose).Info("Finished validating for ExternalProxy upon creation",
			"object", klog.KObj(externalproxy), "duration", time.Since(start))
	}(time.Now())

	return nil, v.validateExternalProxy(externalproxy)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type ExternalProxy.
func (v *ExternalProxyCustomValidator) ValidateUpdate(ctx context.Context, _, newObj runtime.Object) (admission.Warnings, error) {
	externalproxy, ok := newObj.(*networkingv1alpha1.ExternalProxy)
	if !ok {
		return nil, fmt.Errorf("expected a ExternalProxy object for the newObj but got %T", newObj)
	}

	logger := logf.FromContext(ctx).WithName("networking-externalproxy-validator")
	logger.Info("validating for ExternalProxy upon update", "object", klog.KObj(externalproxy))
	defer func(start time.Time) {
		logger.V(verbosity.Verbose).Info("Finished validating for ExternalProxy upon update",
			"object", klog.KObj(externalproxy), "duration", time.Since(start))
	}(time.Now())

	return nil, v.validateExternalProxy(externalproxy)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type ExternalProxy.
func (v *ExternalProxyCustomValidator) ValidateDelete(context.Context, runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// validateExternalProxy validates the fields of an ExternalProxy object.
func (v *ExternalProxyCustomValidator) validateExternalProxy(externalproxy *networkingv1alpha1.ExternalProxy) error {
	allErrors := v.validateExternalProxySpec(&externalproxy.Spec, field.NewPath("spec"))
	if len(allErrors) != 0 {
		return apierrors.NewInvalid(
			networkingv1alpha1.GroupVersion.WithKind("ExternalProxy").GroupKind(),
			externalproxy.Name, allErrors)
	}

	return nil
}

func (v *ExternalProxyCustomValidator) validateExternalProxySpec(spec *networkingv1alpha1.ExternalProxySpec, path *field.Path) field.ErrorList {
	var allErrors field.ErrorList

	allErrors = append(allErrors, v.validateExternalProxyBackends(spec.Backends, path.Child("backends"))...)
	allErrors = append(allErrors, v.validateExternalProxyService(&spec.Service, path.Child("service"))...)
	if spec.Ingress != nil {
		allErrors = append(allErrors, v.validateExternalProxyIngress(spec.Ingress, path.Child("ingress"))...)
	}

	return allErrors
}

func (v *ExternalProxyCustomValidator) validateExternalProxyBackends(backends []networkingv1alpha1.ExternalProxyBackend, path *field.Path) field.ErrorList {
	var allErrors field.ErrorList
	if len(backends) == 0 {
		allErrors = append(allErrors, field.Required(path, "must be specified"))
	}

	for i := range backends {
		backend, bePath := &backends[i], path.Index(i)
		if len(backend.Addresses) == 0 {
			allErrors = append(allErrors, field.Required(bePath.Child("addresses"), "must have at least one address"))
		}
		if len(backend.Ports) == 0 {
			allErrors = append(allErrors, field.Required(bePath.Child("ports"), "must have at least one port"))
		}

		for j := range backend.Addresses {
			address, addrPath := &backend.Addresses[j], bePath.Child("addresses").Index(j)
			allErrors = append(allErrors, v.validateExternalProxyBackendAddress(address, addrPath)...)
		}

		allErrors = append(allErrors, v.validateExternalProxyBackendPorts(backend.Ports, bePath.Child("ports"))...)
	}

	return allErrors
}

func (v *ExternalProxyCustomValidator) validateExternalProxyBackendAddress(address *networkingv1alpha1.ExternalProxyBackendAddress, path *field.Path) field.ErrorList {
	var allErrors field.ErrorList
	allErrors = append(allErrors, validation.ValidateEndpointIP(address.IP, path.Child("ip"))...)

	return allErrors
}

func (v *ExternalProxyCustomValidator) validateExternalProxyBackendPorts(ports []discoveryv1.EndpointPort, path *field.Path) field.ErrorList {
	var allErrors field.ErrorList

	portNames := sets.Set[string]{}
	for j := range ports {
		port, portPath := ports[j], path.Child("ports").Index(j)
		if port.Name != nil && len(*port.Name) > 0 {
			allErrors = append(allErrors, validation.ValidateDNS1123Label(*port.Name, portPath.Child("name"))...)
			if portNames.Has(*port.Name) {
				allErrors = append(allErrors, field.Duplicate(portPath.Child("name"), port.Name))
			} else {
				portNames.Insert(*port.Name)
			}
		}

		if port.Protocol == nil {
			allErrors = append(allErrors, field.Required(portPath.Child("protocol"), ""))
		} else if !v.supportedPortProtocols.Has(*port.Protocol) {
			allErrors = append(allErrors, field.NotSupported(portPath.Child("protocol"), *port.Protocol, sets.List(v.supportedPortProtocols)))
		}

		if port.AppProtocol != nil {
			allErrors = append(allErrors, validation.ValidateQualifiedName(*port.AppProtocol, portPath.Child("appProtocol"))...)
		}
	}

	return allErrors
}

func (v *ExternalProxyCustomValidator) validateExternalProxyService(service *networkingv1alpha1.ExternalProxyService, path *field.Path) field.ErrorList {
	allErrs := v.validateExternalProxyServiceMeta(&service.ExternalProxyServiceMeta, path.Child("metadata"))
	// If the user does not specify the port of the Service, we will automatically populate
	// the Service's port information in Defaulter.
	if len(service.Ports) == 0 {
		allErrs = append(allErrs, field.Required(path.Child("ports"), "must specify at least one port"))
	}

	portNames := sets.Set[string]{}
	requireName := len(service.Ports) > 1
	for i := range service.Ports {
		port, portPath := &service.Ports[i], path.Child("ports").Index(i)
		allErrs = append(allErrs, v.validateExternalProxyServicePort(port, requireName, portNames, portPath)...)
	}

	return allErrs
}

func (v *ExternalProxyCustomValidator) validateExternalProxyServiceMeta(meta *networkingv1alpha1.ExternalProxyServiceMeta, path *field.Path) field.ErrorList {
	var allErrs field.ErrorList
	for _, msg := range validation.ValidateServiceName(meta.Name, false) {
		allErrs = append(allErrs, field.Invalid(path.Child("name"), meta.Name, msg))
	}
	allErrs = append(allErrs, apimachineryapivalidation.ValidateAnnotations(meta.Annotations, path.Child("annotations"))...)
	allErrs = append(allErrs, unversionedvalidation.ValidateLabels(meta.Labels, path.Child("labels"))...)

	return allErrs
}

func (v *ExternalProxyCustomValidator) validateExternalProxyServicePort(port *corev1.ServicePort, requireName bool, portNames sets.Set[string], path *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if requireName && len(port.Name) == 0 {
		allErrs = append(allErrs, field.Required(path.Child("name"), ""))
	} else if len(port.Name) != 0 {
		allErrs = append(allErrs, validation.ValidateDNS1123Label(port.Name, path.Child("name"))...)
		if portNames.Has(port.Name) {
			allErrs = append(allErrs, field.Duplicate(path.Child("name"), port.Name))
		} else {
			portNames.Insert(port.Name)
		}
	}

	for _, msg := range apimachineryvalidation.IsValidPortNum(int(port.Port)) {
		allErrs = append(allErrs, field.Invalid(path.Child("port"), port.Port, msg))
	}

	if len(port.Protocol) == 0 {
		allErrs = append(allErrs, field.Required(path.Child("protocol"), ""))
	} else if !v.supportedPortProtocols.Has(port.Protocol) {
		allErrs = append(allErrs, field.NotSupported(path.Child("protocol"), port.Protocol, sets.List(v.supportedPortProtocols)))
	}

	allErrs = append(allErrs, validation.ValidatePortNumOrName(port.TargetPort, path.Child("targetPort"))...)

	if port.AppProtocol != nil {
		allErrs = append(allErrs, validation.ValidateQualifiedName(*port.AppProtocol, path.Child("appProtocol"))...)
	}

	return allErrs
}

func (v *ExternalProxyCustomValidator) validateExternalProxyIngress(ingress *networkingv1alpha1.ExternalProxyIngress, path *field.Path) field.ErrorList {
	allErrs := v.validateExternalProxyIngressMetadata(&ingress.ExternalProxyIngressMeta, path.Child("metadata"))
	if ingress.IngressClassName != nil {
		for _, msg := range validation.ValidateIngressClassName(*ingress.IngressClassName, false) {
			allErrs = append(allErrs, field.Invalid(path.Child("ingressClassName"), *ingress.IngressClassName, msg))
		}
	}
	if ingress.DefaultBackend != nil {
		allErrs = append(allErrs, validation.ValidateIngressBackend(ingress.DefaultBackend, path.Child("defaultBackend"))...)
	}
	if len(ingress.Rules) == 0 && ingress.DefaultBackend == nil {
		allErrs = append(allErrs, field.Invalid(path, ingress.Rules,
			"either `defaultBackend` or `rules` must be specified"))
	}
	if len(ingress.TLS) > 0 {
		allErrs = append(allErrs, validation.ValidateIngressTLS(ingress.TLS, path.Child("tls"), validation.IngressValidationOptions{})...)
	}
	if len(ingress.Rules) > 0 {
		allErrs = append(allErrs, validation.ValidateIngressRules(ingress.Rules, path.Child("rules"), v.ingressValidationOpts)...)
	}

	return allErrs
}

func (v *ExternalProxyCustomValidator) validateExternalProxyIngressMetadata(meta *networkingv1alpha1.ExternalProxyIngressMeta, path *field.Path) field.ErrorList {
	var allErrs field.ErrorList
	for _, msg := range validation.ValidateServiceName(meta.Name, false) {
		allErrs = append(allErrs, field.Invalid(path.Child("name"), meta.Name, msg))
	}
	allErrs = append(allErrs, apimachineryapivalidation.ValidateAnnotations(meta.Annotations, path.Child("annotations"))...)
	allErrs = append(allErrs, unversionedvalidation.ValidateLabels(meta.Labels, path.Child("labels"))...)

	return allErrs
}
