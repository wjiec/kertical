package portforwarding_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	portforwardingutils "github.com/wjiec/kertical/internal/controller/networking/portforwarding"
)

func TestFindEndpointSlicePort(t *testing.T) {
	t.Run("single port", func(t *testing.T) {
		found := portforwardingutils.FindEndpointSlicePort(intstr.FromString("http"), []*discoveryv1.EndpointSlice{
			{
				AddressType: discoveryv1.AddressTypeIPv4,
				Ports: []discoveryv1.EndpointPort{
					{
						Protocol: ptr.To(corev1.ProtocolTCP),
						Port:     ptr.To(int32(8080)),
					},
				},
			},
		})
		if assert.NotNil(t, found) {
			assert.Equal(t, int32(8080), *found.Port)
		}
	})

	t.Run("multiple ports", func(t *testing.T) {
		found := portforwardingutils.FindEndpointSlicePort(intstr.FromString("http"), []*discoveryv1.EndpointSlice{
			{
				AddressType: discoveryv1.AddressTypeIPv4,
				Ports: []discoveryv1.EndpointPort{
					{
						Name:     ptr.To("ws1"),
						Protocol: ptr.To(corev1.ProtocolTCP),
						Port:     ptr.To(int32(8080)),
					},
					{
						Name:     ptr.To("http"),
						Protocol: ptr.To(corev1.ProtocolTCP),
						Port:     ptr.To(int32(8081)),
					},
				},
			},
		})
		if assert.NotNil(t, found) {
			assert.Equal(t, int32(8081), *found.Port)
		}
	})
}

func TestFindServicePort(t *testing.T) {
	t.Run("regular", func(t *testing.T) {
		svc := &corev1.Service{
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{
					{
						Name:       "ws",
						Protocol:   corev1.ProtocolTCP,
						Port:       8080,
						TargetPort: intstr.FromString("ws"),
					},
					{
						Name:       "http",
						Protocol:   corev1.ProtocolTCP,
						Port:       8081,
						TargetPort: intstr.FromInt32(8081),
					},
				},
			},
		}

		endpointSlices := []*discoveryv1.EndpointSlice{
			{
				Endpoints: []discoveryv1.Endpoint{
					{
						Addresses:  []string{"10.0.0.1"},
						Conditions: discoveryv1.EndpointConditions{Ready: ptr.To(true)},
					},
					{
						Addresses:  []string{"10.0.0.2"},
						Conditions: discoveryv1.EndpointConditions{Ready: ptr.To(false)},
					},
					{
						Addresses:  []string{"10.0.0.3"},
						Conditions: discoveryv1.EndpointConditions{Ready: ptr.To(true)},
					},
				},
				Ports: []discoveryv1.EndpointPort{
					{
						Name:     ptr.To("ws"),
						Protocol: ptr.To(corev1.ProtocolTCP),
						Port:     ptr.To(int32(8080)),
					},
					{
						Protocol: ptr.To(corev1.ProtocolTCP),
						Port:     ptr.To(int32(8081)),
					},
				},
			},
		}

		t.Run("name", func(t *testing.T) {
			port, found := portforwardingutils.FindServicePort(svc, intstr.FromString("http"), endpointSlices)
			if assert.NotNil(t, port) && assert.True(t, found) {
				assert.Equal(t, int32(8081), port.Port)
			}
		})
	})
}
