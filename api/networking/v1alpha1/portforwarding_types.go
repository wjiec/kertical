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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/net"
)

// PortForwardingSpec defines the desired state of PortForwarding.
type PortForwardingSpec struct {
	// ServiceRef references the Service in the current namespace that needs
	// port forwarding. This field is immutable.
	//
	// +kubebuilder:validation:Required
	ServiceRef corev1.ObjectReference `json:"serviceRef"`

	// The list of ports that are exposed on the host.
	//
	// +optional
	// +kubebuilder:validation:Optional
	Ports []PortForwardingPort `json:"ports,omitempty"`
}

// PortForwardingPort represents the mapping between a service port and a local exposed port.
type PortForwardingPort struct {
	// Target is the port number or name in the referenced Service.
	//
	// If using a numbers, it must be a valid port number (0 < x < 65536), and it
	// must be unique within the Service.
	//
	// If using a string, it must match a named port in the referenced Service.
	//
	// +kubebuilder:validation:Required
	Target intstr.IntOrString `json:"target"`

	// HostPort is the port to expose on the host. If specified, it must be a
	// valid port number, 0 < x < 65536.
	//
	// If not specified, it defaults to the port number specified by the target.
	//
	// +optional
	// +kubebuilder:validation:Optional
	HostPort *int32 `json:"hostPort,omitempty"`
}

// PortForwardingStatus defines the observed state of PortForwarding.
type PortForwardingStatus struct {
	// Represents the observations of a PortForwarding's current state.
	//
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ForwardedPorts represents the current status of each port forwarding
	ForwardedPorts []ForwardedPort `json:"forwardingStatus,omitempty"`
}

// ForwardedPort represents the status of a port forwarding configuration
type ForwardedPort struct {
	// Protocol specifies the network protocol used for port forwarding (TCP or UDP)
	Protocol net.Protocol `json:"protocol"`

	// SourcePort is the port number currently being listened on
	SourcePort int32 `json:"sourcePort"`

	// TargetHosts is the destination host address for the port forwarding
	TargetHosts []string `json:"targetHosts"`

	// TargetPort is the port number to which we are forwarding
	TargetPort int32 `json:"targetPort"`

	// State represents the current state and description
	State PortForwardingState `json:"state"`
}

// PortForwardingState represents the state of a port forwarding
type PortForwardingState string

const (
	PortForwardingReady    PortForwardingState = "Ready"
	PortForwardingConflict PortForwardingState = "Conflict"
	PortForwardingResidual PortForwardingState = "Residual"
	PortForwardingFailed   PortForwardingState = "Failed"
)

const (
	PortForwardingConditionReady    = "Ready"
	PortForwardingConditionDegraded = "Degraded"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:categories=kertical,shortName=kpf
// +kubebuilder:subresource:status

// PortForwarding is the Schema for the portforwardings API.
type PortForwarding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PortForwardingSpec   `json:"spec,omitempty"`
	Status PortForwardingStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PortForwardingList contains a list of PortForwarding.
type PortForwardingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PortForwarding `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PortForwarding{}, &PortForwardingList{})
}
