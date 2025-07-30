package portforwarding

import (
	"bytes"
	"net"
	"slices"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	netutils "k8s.io/utils/net"
	"k8s.io/utils/ptr"

	networkingv1alpha1 "github.com/wjiec/kertical/api/networking/v1alpha1"
	"github.com/wjiec/kertical/internal/kertical"
)

// ServiceObjectKey extracts the namespaced name of the referenced service from a [*networkingv1alpha1.PortForwarding] resource
func ServiceObjectKey(pf *networkingv1alpha1.PortForwarding) types.NamespacedName {
	return types.NamespacedName{
		Name:      pf.Spec.ServiceRef.Name,
		Namespace: pf.Namespace,
	}
}

// FindEndpointSlicePort searches for a port with the specified name across all provided EndpointSlices.
//
// If an EndpointSlice has only one port, it returns that port regardless of name.
// Returns nil if no matching port is found.
func FindEndpointSlicePort(target intstr.IntOrString, endpointSlices []*discoveryv1.EndpointSlice) *discoveryv1.EndpointPort {
	for _, endpointSlice := range endpointSlices {
		if len(endpointSlice.Ports) == 1 {
			return &endpointSlice.Ports[0]
		}

		for _, port := range endpointSlice.Ports {
			if ptr.Deref(port.Name, "") == target.StrVal || ptr.Deref(port.Port, int32(0)) == target.IntVal {
				return &port
			}
		}
	}
	return nil
}

// FindServicePort searches for a port in a service that matches the target.
func FindServicePort(svc *corev1.Service, target intstr.IntOrString, endpointSlices []*discoveryv1.EndpointSlice) (corev1.ServicePort, bool) {
	for _, port := range svc.Spec.Ports {
		if port.Port == target.IntVal || port.Name == target.StrVal {
			// For headless services, we need to find the actual port used in the pod
			if svc.Spec.ClusterIP == corev1.ClusterIPNone {
				endpointSlicePort := FindEndpointSlicePort(port.TargetPort, endpointSlices)
				if endpointSlicePort == nil {
					break
				}

				return corev1.ServicePort{
					Name:     ptr.Deref(endpointSlicePort.Name, ""),
					Protocol: ptr.Deref(endpointSlicePort.Protocol, corev1.ProtocolTCP),
					Port:     ptr.Deref(endpointSlicePort.Port, int32(0)),
				}, true

			}
			return port, true
		}
	}
	return corev1.ServicePort{}, false
}

type ForwardedPorts = []networkingv1alpha1.ForwardedPort

// SyncForwardingPorts reconciles the current forwarded ports with the desired configuration
//
// Returns three lists: ports to add, ports to delete, and unchanged ports
func SyncForwardingPorts(pf *networkingv1alpha1.PortForwarding, svc *corev1.Service, endpointSlices []*discoveryv1.EndpointSlice) (
	ForwardedPorts, ForwardedPorts, ForwardedPorts, ForwardedPorts,
) {
	forwardedPorts := make(map[intstr.IntOrString]networkingv1alpha1.ForwardedPort)
	for _, nodeStatus := range pf.Status.NodePortForwardingStatus {
		if nodeStatus.NodeName == kertical.NodeName() {
			for _, elem := range nodeStatus.ForwardedPorts {
				forwardedPorts[elem.SourcePort] = elem
			}
			break
		}
	}

	var additions, deletions, unchanged, errors ForwardedPorts
	for _, forwarding := range pf.Spec.Ports {
		// Find the service port that matches the forwarding target
		servicePort, found := FindServicePort(svc, forwarding.Target, endpointSlices)
		if !found {
			errors = append(errors, networkingv1alpha1.ForwardedPort{
				SourcePort: forwarding.Target,
				State:      networkingv1alpha1.PortForwardingFailed,
			})
			continue
		}

		// Determine target hosts based on service type
		var targetHosts []string
		if svc.Spec.ClusterIP != corev1.ClusterIPNone {
			// For regular services, use the cluster IP
			targetHosts = append(targetHosts, svc.Spec.ClusterIP)
		} else {
			// For headless services, use the individual pod IPs
			for _, endpointSlice := range endpointSlices {
				for _, endpoint := range endpointSlice.Endpoints {
					if ptr.Deref(endpoint.Conditions.Ready, false) {
						targetHosts = append(targetHosts, endpoint.Addresses...)
					}
				}
			}
		}
		slices.SortFunc(targetHosts, func(a, b string) int {
			return bytes.Compare(net.ParseIP(a), net.ParseIP(b))
		})

		// If no specific host port is specified, use the service port as the source port
		sourcePort := intstr.FromInt32(servicePort.Port)
		if forwarding.HostPort != nil {
			sourcePort = intstr.FromInt32(*forwarding.HostPort)
		}

		protocol := netutils.Protocol(servicePort.Protocol)
		if forwardPort, exists := forwardedPorts[sourcePort]; exists {
			// If this port is already forwarded, check if we need to recreate the rule
			sameTargetHosts := slices.Equal(targetHosts, forwardPort.TargetHosts)
			if forwardPort.Protocol != protocol || forwardPort.SourcePort != sourcePort || !sameTargetHosts {
				// We need to delete the old forwarding rule and add a new one
				deletions = append(deletions, forwardPort)
				additions = append(additions, networkingv1alpha1.ForwardedPort{
					Protocol:    protocol,
					SourcePort:  sourcePort,
					TargetHosts: targetHosts,
					TargetPort:  servicePort.Port,
					State:       networkingv1alpha1.PortForwardingReady,
				})
			} else {
				// The current forwarding rule hasn't changed, no action needed
				unchanged = append(unchanged, forwardPort)
			}

			delete(forwardedPorts, sourcePort)
		} else {
			// If the forwarding rule doesn't exist, we need to create a new one
			additions = append(additions, networkingv1alpha1.ForwardedPort{
				Protocol:    protocol,
				SourcePort:  sourcePort,
				TargetHosts: targetHosts,
				TargetPort:  servicePort.Port,
				State:       networkingv1alpha1.PortForwardingReady,
			})
		}
	}

	for _, forwardedPort := range forwardedPorts {
		deletions = append(deletions, forwardedPort)
	}
	return additions, deletions, unchanged, errors
}
