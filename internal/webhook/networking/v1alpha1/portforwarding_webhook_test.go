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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	networkingv1alpha1 "github.com/wjiec/kertical/api/networking/v1alpha1"
)

const (
	TestPortForwardingName = "test-port-forwarding"
)

var _ = Describe("PortForwarding Defaulter Webhook", func() {
	var (
		defaulter      *PortForwardingCustomDefaulter
		portforwarding *networkingv1alpha1.PortForwarding
	)

	BeforeEach(func() {
		defaulter = NewPortForwardingCustomDefaulter(scheme)
		portforwarding = &networkingv1alpha1.PortForwarding{
			ObjectMeta: metav1.ObjectMeta{
				Name: TestPortForwardingName,
			},
		}

		Expect(defaulter).NotTo(BeNil(), "Expected defaulter to be initialized")
		Expect(portforwarding).NotTo(BeNil(), "Expected portforwarding to be initialized")
	})

	Context("When creating ExternalProxy under Defaulting Webhook", func() {
		It("should not error occurred", func() {
			err := defaulter.Default(ctx, portforwarding)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("When providing incorrect object type", func() {
		It("should return an error", func() {
			err := defaulter.Default(ctx, &corev1.Pod{})
			Expect(err).To(HaveOccurred())
		})
	})
})

var _ = Describe("PortForwarding Validator Webhook", func() {
	var (
		validator      *PortForwardingCustomValidator
		portforwarding *networkingv1alpha1.PortForwarding
	)

	BeforeEach(func() {
		validator = NewPortForwardingCustomValidator()
		portforwarding = &networkingv1alpha1.PortForwarding{
			ObjectMeta: metav1.ObjectMeta{
				Name: TestPortForwardingName,
			},
		}

		Expect(validator).NotTo(BeNil(), "Expected validator to be initialized")
		Expect(portforwarding).NotTo(BeNil(), "Expected portforwarding to be initialized")
	})

	Context("When creating or updating ExternalProxy under Validating Webhook", func() {
		Context("With valid configuration", func() {
			BeforeEach(func() {
				portforwarding.Spec = networkingv1alpha1.PortForwardingSpec{
					ServiceRef: corev1.LocalObjectReference{
						Name: "foobar",
					},
					Ports: []networkingv1alpha1.PortForwardingPort{
						{Target: intstr.FromString("http")},
					},
				}
			})

			It("should validate without errors during creation", func() {
				warnings, err := validator.ValidateCreate(ctx, portforwarding)
				Expect(err).NotTo(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})

			It("should validate without errors during update", func() {
				warnings, err := validator.ValidateUpdate(ctx, portforwarding, portforwarding)
				Expect(err).NotTo(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})
		})

		Context("With missing serviceRef name", func() {
			BeforeEach(func() {
				portforwarding.Spec.ServiceRef.Name = ""
				portforwarding.Spec.Ports = []networkingv1alpha1.PortForwardingPort{
					{Target: intstr.FromString("http")},
				}
			})

			It("should return validation error during creation", func() {
				warnings, err := validator.ValidateCreate(ctx, portforwarding)
				Expect(err).To(HaveOccurred())
				Expect(warnings).To(BeEmpty())

				Expect(apierrors.HasStatusCause(err, metav1.CauseTypeFieldValueRequired)).To(BeTrue())
			})

			It("should return validation error during update", func() {
				warnings, err := validator.ValidateUpdate(ctx, portforwarding, portforwarding)
				Expect(err).To(HaveOccurred())
				Expect(warnings).To(BeEmpty())

				Expect(apierrors.HasStatusCause(err, metav1.CauseTypeFieldValueRequired)).To(BeTrue())
			})
		})

		Context("With empty ports list", func() {
			BeforeEach(func() {
				portforwarding.Spec.ServiceRef.Name = "foobar"
				portforwarding.Spec.Ports = nil
			})

			It("should return validation error during creation", func() {
				warnings, err := validator.ValidateCreate(ctx, portforwarding)
				Expect(err).To(HaveOccurred())
				Expect(warnings).To(BeEmpty())

				Expect(apierrors.HasStatusCause(err, metav1.CauseTypeFieldValueRequired)).To(BeTrue())
			})

			It("should return validation error during update", func() {
				warnings, err := validator.ValidateUpdate(ctx, portforwarding, portforwarding)
				Expect(err).To(HaveOccurred())
				Expect(warnings).To(BeEmpty())

				Expect(apierrors.HasStatusCause(err, metav1.CauseTypeFieldValueRequired)).To(BeTrue())
			})
		})
	})

	Context("When providing incorrect object type", func() {
		It("should return error during creation", func() {
			warnings, err := validator.ValidateCreate(ctx, &corev1.Service{})
			Expect(err).To(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})

		It("should return error during update", func() {
			warnings, err := validator.ValidateUpdate(ctx, &corev1.Service{}, &corev1.Service{})
			Expect(err).To(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})
	})
})
