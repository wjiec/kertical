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

package externalproxy

import (
	"fmt"
	"maps"
	"net"
	"slices"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/wjiec/kertical/api/networking"
	networkingv1alpha1 "github.com/wjiec/kertical/api/networking/v1alpha1"
	"github.com/wjiec/kertical/internal/kertical"
)

const (
	Kind = "ExternalProxy"
)

// NewService creates a new Service object based on the given ExternalProxy instance.
func NewService(instance *networkingv1alpha1.ExternalProxy) *corev1.Service {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Spec.Service.Name,
			Namespace: instance.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Type:  instance.Spec.Service.Type,
			Ports: instance.Spec.Service.Ports,
		},
	}

	// copy labels and annotations from the ExternalProxy specification to the service's metadata.
	service.Labels = maps.Clone(instance.Spec.Service.Labels)
	service.Annotations = maps.Clone(instance.Spec.Service.Annotations)

	return InjectExternalProxyRevision(instance, service)
}

// ServiceKey generates a client.ObjectKey for accessing the Service associated
// with a given ExternalProxy instance.
func ServiceKey(instance *networkingv1alpha1.ExternalProxy) client.ObjectKey {
	return client.ObjectKey{Namespace: instance.Namespace, Name: instance.Spec.Service.Name}
}

// NewEndpointSlices generates a list of EndpointSlice objects from the ExternalProxy instance.
//
// It creates separate EndpointSlices for IPv4 and IPv6 addresses for each backend specified in the instance.
func NewEndpointSlices(instance *networkingv1alpha1.ExternalProxy) []*discoveryv1.EndpointSlice {
	var res []*discoveryv1.EndpointSlice
	for idx, backend := range instance.Spec.Backends {
		namePrefix := fmt.Sprintf("%s-be%d", instance.Spec.Service.Name, idx)
		// EndpointSlices require explicit configuration of their address types, so we need
		// to create separate objects based on the IP address type.
		v4EndpointSlice := NewEndpointSliceFromBackend(namePrefix+"v4", &backend, filterIPv4)
		v6EndpointSlice := NewEndpointSliceFromBackend(namePrefix+"v6", &backend, filterIPv6)

		if len(v4EndpointSlice.Endpoints) != 0 {
			v4EndpointSlice.AddressType = discoveryv1.AddressTypeIPv4
			res = append(res, InjectExternalProxyRevision(instance, v4EndpointSlice))
		}

		if len(v6EndpointSlice.Endpoints) != 0 {
			v6EndpointSlice.AddressType = discoveryv1.AddressTypeIPv6
			res = append(res, InjectExternalProxyRevision(instance, v6EndpointSlice))
		}
	}

	for _, endpointSlice := range res {
		endpointSlice.Namespace = instance.Namespace
		endpointSlice.Labels = map[string]string{
			discoveryv1.LabelManagedBy:   kertical.ControllerName(),
			discoveryv1.LabelServiceName: instance.Spec.Service.Name,
		}
	}

	return res
}

// NewEndpointSliceFromBackend creates an EndpointSlice from a given
// backend and filters out IP addresses using a specified function.
func NewEndpointSliceFromBackend(name string, backend *networkingv1alpha1.ExternalProxyBackend, filter func(string) net.IP) *discoveryv1.EndpointSlice {
	endpointSlice := &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	for _, address := range backend.Addresses {
		if ip := filter(address.IP); ip != nil {
			endpointSlice.Endpoints = append(endpointSlice.Endpoints, discoveryv1.Endpoint{
				Addresses: []string{ip.String()},
				Conditions: discoveryv1.EndpointConditions{
					Ready: ptr.To(true),
				},
			})
		}
	}

	endpointSlice.Ports = append(endpointSlice.Ports, backend.Ports...)
	return endpointSlice
}

// NewIngress creates a new Ingress object based on the specification provided in the ExternalProxy instance.
func NewIngress(instance *networkingv1alpha1.ExternalProxy) *networkingv1.Ingress {
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Spec.Ingress.Name,
			Namespace: instance.Namespace,
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: instance.Spec.Ingress.IngressClassName,
			TLS:              slices.Clone(instance.Spec.Ingress.TLS),
		},
	}

	// copy labels and annotations from the ExternalProxy specification to the ingress's metadata.
	ingress.Labels = maps.Clone(instance.Spec.Ingress.Labels)
	ingress.Annotations = maps.Clone(instance.Spec.Ingress.Annotations)

	toIngressBackend := func(backend *networkingv1alpha1.ExternalProxyIngressBackend) *networkingv1.IngressBackend {
		return &networkingv1.IngressBackend{
			Service: &networkingv1.IngressServiceBackend{
				Name: instance.Spec.Service.Name,
				Port: networkingv1.ServiceBackendPort{
					Name:   backend.Port.Name,
					Number: backend.Port.Number,
				},
			},
		}
	}

	// If a default backend is specified, set it in the Ingress spec.
	if instance.Spec.Ingress.DefaultBackend != nil {
		ingress.Spec.DefaultBackend = toIngressBackend(instance.Spec.Ingress.DefaultBackend)
	}

	for _, rule := range instance.Spec.Ingress.Rules {
		ingressRule := networkingv1.IngressRule{
			Host: rule.Host,
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{},
			},
		}

		for _, path := range rule.HTTP.Paths {
			ingressRule.HTTP.Paths = append(ingressRule.HTTP.Paths, networkingv1.HTTPIngressPath{
				Path:     path.Path,
				PathType: path.PathType,
				Backend:  *toIngressBackend(path.Backend),
			})
		}

		ingress.Spec.Rules = append(ingress.Spec.Rules, ingressRule)
	}

	return InjectExternalProxyRevision(instance, ingress)
}

// InjectExternalProxyRevision sets the revision annotation on the target object.
func InjectExternalProxyRevision[T client.Object](controller *networkingv1alpha1.ExternalProxy, target T) T {
	annotations := target.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	annotations[networking.ExternalProxyRevisionAnnotationKey] = strconv.FormatInt(controller.Generation, 10)
	target.SetAnnotations(annotations)

	return target
}

// ShouldReconcileResource checks whether reconciliation is necessary for the given resource
// based on the revision of the ExternalProxy controller.
func ShouldReconcileResource[T client.Object](controller *networkingv1alpha1.ExternalProxy, object T) bool {
	revision := object.GetAnnotations()[networking.ExternalProxyRevisionAnnotationKey]
	return revision != strconv.FormatInt(controller.Generation, 10)
}

// filterIPv4 parses the given string as an IP address and returns the IPv4 address if valid.
//
// If the input cannot be parsed into a valid IPv4 address, it returns nil.
func filterIPv4(s string) net.IP {
	return net.ParseIP(s).To4()
}

// filterIPv6 parses the given string as an IP address and returns the IPv6 address if valid.
//
// If the parsed IP is not an IPv6 address, it returns nil.
func filterIPv6(s string) net.IP {
	if ip := net.ParseIP(s); ip.To4() == nil {
		return ip
	}
	return nil
}
