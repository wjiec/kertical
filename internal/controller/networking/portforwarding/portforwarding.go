package portforwarding

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/net"

	networkingv1alpha1 "github.com/wjiec/kertical/api/networking/v1alpha1"
)

// ServiceObjectKey extracts the namespaced name of the referenced service from a [*networkingv1alpha1.PortForwarding] resource
func ServiceObjectKey(pf *networkingv1alpha1.PortForwarding) types.NamespacedName {
	return types.NamespacedName{
		Name:      pf.Spec.ServiceRef.Name,
		Namespace: pf.Spec.ServiceRef.Namespace,
	}
}

// FindServicePort searches for a port in a service that matches the target.
func FindServicePort(svc *corev1.Service, target intstr.IntOrString) (corev1.ServicePort, bool) {
	for _, port := range svc.Spec.Ports {
		if port.Port == target.IntVal || port.Name == target.StrVal {
			return port, true
		}
	}
	return corev1.ServicePort{}, false
}

type ForwardedPorts = []networkingv1alpha1.ForwardedPort

// SyncForwardingPorts reconciles the current forwarded ports with the desired configuration
// Returns three lists: ports to add, ports to delete, and unchanged ports
func SyncForwardingPorts(pf *networkingv1alpha1.PortForwarding, svc *corev1.Service) (ForwardedPorts, ForwardedPorts, ForwardedPorts, error) {
	forwardedPorts := make(map[int32]networkingv1alpha1.ForwardedPort)
	for _, elem := range pf.Status.ForwardedPorts {
		forwardedPorts[elem.SourcePort] = elem
	}

	var additions, deletions, unchanged ForwardedPorts
	for _, forwarding := range pf.Spec.Ports {
		servicePort, found := FindServicePort(svc, forwarding.Target)
		if !found {
			return nil, nil, nil, fmt.Errorf("port %q not found in the service %q", forwarding.Target, svc.Name)
		}

		sourcePort := servicePort.Port
		if forwarding.HostPort != nil {
			sourcePort = *forwarding.HostPort
		}

		protocol := net.Protocol(servicePort.Protocol)
		if forwardPort, forwarded := forwardedPorts[sourcePort]; forwarded {
			// If this port is already forwarded, check if we need to recreate the rule
			if forwardPort.Protocol != protocol || forwardPort.TargetPort != servicePort.Port {
				// We need to delete the old forwarding rule and add a new one
				deletions = append(deletions, forwardPort)
				additions = append(additions, networkingv1alpha1.ForwardedPort{
					Protocol:   protocol,
					SourcePort: sourcePort,
					TargetHost: svc.Spec.ClusterIP, // TODO headless service
					TargetPort: servicePort.Port,
					State:      networkingv1alpha1.PortForwardingReady,
				})
			} else {
				// The current forwarding rule hasn't changed, no action needed
				unchanged = append(unchanged, forwardPort)
			}
		} else {
			// If the forwarding rule doesn't exist, we need to create a new one
			additions = append(additions, networkingv1alpha1.ForwardedPort{
				Protocol:   protocol,
				SourcePort: sourcePort,
				TargetHost: svc.Spec.ClusterIP, // TODO headless service
				TargetPort: servicePort.Port,
				State:      networkingv1alpha1.PortForwardingReady,
			})
		}
	}

	return additions, deletions, unchanged, nil
}
