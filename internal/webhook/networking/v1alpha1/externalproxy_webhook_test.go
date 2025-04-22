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
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	networkingv1alpha1 "github.com/wjiec/kertical/api/networking/v1alpha1"
)

const (
	TestExternalProxyName = "test-external-proxy"
)

var _ = Describe("ExternalProxy Defaulter Webhook", func() {
	var (
		defaulter     *ExternalProxyCustomDefaulter
		externalproxy *networkingv1alpha1.ExternalProxy
	)

	BeforeEach(func() {
		ctx = admission.NewContextWithRequest(ctx, admission.Request{})
		defaulter = NewExternalProxyCustomDefaulter(scheme)
		externalproxy = &networkingv1alpha1.ExternalProxy{
			ObjectMeta: metav1.ObjectMeta{
				Name: TestExternalProxyName,
			},
		}

		Expect(defaulter).NotTo(BeNil(), "Expected defaulter to be initialized")
		Expect(externalproxy).NotTo(BeNil(), "Expected externalproxy to be initialized")
	})

	Context("When creating ExternalProxy under Defaulting Webhook", func() {
		It("should set default Service name when not specified", func() {
			externalproxy.Spec = networkingv1alpha1.ExternalProxySpec{
				Service: networkingv1alpha1.ExternalProxyService{
					// Name is intentionally left empty
				},
			}

			err := defaulter.Default(ctx, externalproxy)
			Expect(err).NotTo(HaveOccurred())
			Expect(externalproxy.Spec.Service.Name).To(Equal(externalproxy.Name))
		})

		It("should not override existing Service name", func() {
			existingName := "existing-service"
			externalproxy.Spec = networkingv1alpha1.ExternalProxySpec{
				Service: networkingv1alpha1.ExternalProxyService{
					ExternalProxyServiceMeta: networkingv1alpha1.ExternalProxyServiceMeta{
						Name: existingName,
					},
				},
			}

			err := defaulter.Default(ctx, externalproxy)
			Expect(err).NotTo(HaveOccurred())
			Expect(externalproxy.Spec.Service.Name).To(Equal(existingName))
		})

		It("should set default Service type when not specified", func() {
			externalproxy.Spec = networkingv1alpha1.ExternalProxySpec{
				Service: networkingv1alpha1.ExternalProxyService{
					// Type is intentionally left empty
				},
			}

			err := defaulter.Default(ctx, externalproxy)
			Expect(err).NotTo(HaveOccurred())
			Expect(externalproxy.Spec.Service.Type).To(Equal(defaulter.DefaultServiceType))
		})

		It("should set default Protocol for Backend ports", func() {
			externalproxy.Spec = networkingv1alpha1.ExternalProxySpec{
				Backends: []networkingv1alpha1.ExternalProxyBackend{
					{
						Addresses: []networkingv1alpha1.ExternalProxyBackendAddress{
							{IP: "10.0.0.1"},
						},
						Ports: []discoveryv1.EndpointPort{
							{
								Name: ptr.To("http"),
								Port: ptr.To(int32(80)),
								// Protocol intentionally left nil
							},
						},
					},
				},
				Service: networkingv1alpha1.ExternalProxyService{},
			}

			err := defaulter.Default(ctx, externalproxy)
			Expect(err).NotTo(HaveOccurred())
			Expect(*externalproxy.Spec.Backends[0].Ports[0].Protocol).To(Equal(defaulter.DefaultBackendPortProtocol))
		})

		It("should not override existing Backend port Protocol", func() {
			externalproxy.Spec = networkingv1alpha1.ExternalProxySpec{
				Backends: []networkingv1alpha1.ExternalProxyBackend{
					{
						Addresses: []networkingv1alpha1.ExternalProxyBackendAddress{
							{IP: "10.0.0.1"},
						},
						Ports: []discoveryv1.EndpointPort{
							{
								Name:     ptr.To("http"),
								Port:     ptr.To(int32(80)),
								Protocol: ptr.To(corev1.ProtocolUDP),
							},
						},
					},
				},
				Service: networkingv1alpha1.ExternalProxyService{},
			}

			err := defaulter.Default(ctx, externalproxy)
			Expect(err).NotTo(HaveOccurred())
			Expect(*externalproxy.Spec.Backends[0].Ports[0].Protocol).To(Equal(corev1.ProtocolUDP))
		})

		Context("When Backend is specified", func() {
			BeforeEach(func() {
				externalproxy.Spec.Backends = []networkingv1alpha1.ExternalProxyBackend{
					{
						Addresses: []networkingv1alpha1.ExternalProxyBackendAddress{
							{IP: "10.0.0.1"},
						},
						Ports: []discoveryv1.EndpointPort{
							{
								Name:     ptr.To("http"),
								Port:     ptr.To(int32(80)),
								Protocol: ptr.To(corev1.ProtocolTCP),
							},
						},
					},
					{
						Addresses: []networkingv1alpha1.ExternalProxyBackendAddress{
							{IP: "10.0.0.2"},
						},
						Ports: []discoveryv1.EndpointPort{
							{
								Name:     ptr.To("http"),
								Port:     ptr.To(int32(8080)),
								Protocol: ptr.To(corev1.ProtocolTCP),
							},
							{
								Name:     ptr.To("https"),
								Port:     ptr.To(int32(443)),
								Protocol: ptr.To(corev1.ProtocolTCP),
							},
						},
					},
				}
			})

			It("should generate Service ports from Backend ports when Service ports not specified", func() {
				externalproxy.Spec.Service = networkingv1alpha1.ExternalProxyService{
					// Ports intentionally left empty
				}

				err := defaulter.Default(ctx, externalproxy)
				Expect(err).NotTo(HaveOccurred())
				Expect(externalproxy.Spec.Service.Ports).To(HaveLen(2))

				Expect(externalproxy.Spec.Service.Ports[0].Name).To(Equal(*externalproxy.Spec.Backends[0].Ports[0].Name))
				Expect(externalproxy.Spec.Service.Ports[0].Port).To(Equal(*externalproxy.Spec.Backends[0].Ports[0].Port))
				Expect(externalproxy.Spec.Service.Ports[0].Protocol).To(Equal(*externalproxy.Spec.Backends[0].Ports[0].Protocol))

				Expect(externalproxy.Spec.Service.Ports[1].Name).To(Equal(*externalproxy.Spec.Backends[1].Ports[1].Name))
				Expect(externalproxy.Spec.Service.Ports[1].Port).To(Equal(*externalproxy.Spec.Backends[1].Ports[1].Port))
				Expect(externalproxy.Spec.Service.Ports[1].Protocol).To(Equal(*externalproxy.Spec.Backends[1].Ports[1].Protocol))
			})

			It("should not override existing Service ports", func() {
				portName, portNumber := "ws", int32(8080)
				externalproxy.Spec.Service = networkingv1alpha1.ExternalProxyService{
					Ports: []corev1.ServicePort{
						{
							Name:     portName,
							Port:     portNumber,
							Protocol: corev1.ProtocolTCP,
						},
					},
				}

				err := defaulter.Default(ctx, externalproxy)
				Expect(err).NotTo(HaveOccurred())
				Expect(externalproxy.Spec.Service.Ports).To(HaveLen(1))
				Expect(externalproxy.Spec.Service.Ports[0].Name).To(Equal(portName))
				Expect(externalproxy.Spec.Service.Ports[0].Port).To(Equal(portNumber))
			})

			Context("When Ingress is specified", func() {
				BeforeEach(func() {
					externalproxy.Spec.Ingress = &networkingv1alpha1.ExternalProxyIngress{}
				})

				It("should set default Ingress name when not specified", func() {
					err := defaulter.Default(ctx, externalproxy)
					Expect(err).NotTo(HaveOccurred())
					Expect(externalproxy.Spec.Ingress.Name).To(Equal(TestExternalProxyName))
				})

				It("should not override existing Ingress name", func() {
					existingName := "existing-ingress"
					externalproxy.Spec.Ingress.Name = existingName

					err := defaulter.Default(ctx, externalproxy)
					Expect(err).NotTo(HaveOccurred())
					Expect(externalproxy.Spec.Ingress.Name).To(Equal(existingName))
				})

				Context("When single port in service", func() {
					BeforeEach(func() {
						externalproxy.Spec.Service = networkingv1alpha1.ExternalProxyService{
							Ports: []corev1.ServicePort{
								{
									Name:       "http",
									Port:       80,
									TargetPort: intstr.FromString("http"),
									Protocol:   corev1.ProtocolTCP,
								},
							},
						}
					})

					It("should set default backend for ingress with single service port", func() {
						err := defaulter.Default(ctx, externalproxy)
						Expect(err).NotTo(HaveOccurred())
						Expect(externalproxy.Spec.Ingress.DefaultBackend).NotTo(BeNil())
						Expect(externalproxy.Spec.Ingress.DefaultBackend.Port.Number).To(Equal(int32(80)))
					})

					It("should generate default HTTP paths when HTTP is nil", func() {
						// Setup service with a single port
						externalproxy.Spec.Ingress.Rules = []networkingv1alpha1.ExternalProxyIngressRule{
							{
								Host: "example.com",
								// HTTP intentionally nil
							},
						}

						err := defaulter.Default(ctx, externalproxy)
						Expect(err).NotTo(HaveOccurred())

						// Check default paths are generated
						rule := externalproxy.Spec.Ingress.Rules[0]
						Expect(rule.HTTP).NotTo(BeNil())
						Expect(rule.HTTP.Paths).To(HaveLen(1))
						Expect(rule.HTTP.Paths[0].Path).To(Equal("/"))
					})

					It("should set default path type and backend for ingress rules with single service port", func() {
						externalproxy.Spec.Ingress = &networkingv1alpha1.ExternalProxyIngress{
							Rules: []networkingv1alpha1.ExternalProxyIngressRule{
								{
									Host: "kertical.com",
									HTTP: &networkingv1alpha1.ExternalProxyIngressHttpRuleValue{
										Paths: []networkingv1alpha1.ExternalProxyIngressHttpPath{
											{Path: "/"},
										},
									},
								},
							},
						}

						err := defaulter.Default(ctx, externalproxy)
						Expect(err).NotTo(HaveOccurred())

						// Check the first rule's path type and backend are set
						rule := externalproxy.Spec.Ingress.Rules[0]
						Expect(rule.HTTP).NotTo(BeNil())
						Expect(rule.HTTP.Paths).To(HaveLen(1))
						Expect(*rule.HTTP.Paths[0].PathType).To(Equal(defaulter.DefaultIngressPathType))
						Expect(rule.HTTP.Paths[0].Backend).NotTo(BeNil())
						Expect(rule.HTTP.Paths[0].Backend.Port.Number).To(Equal(int32(80)))
					})
				})
			})
		})

		It("should handle backends with unnamed ports", func() {
			externalproxy.Spec = networkingv1alpha1.ExternalProxySpec{
				Backends: []networkingv1alpha1.ExternalProxyBackend{
					{
						Addresses: []networkingv1alpha1.ExternalProxyBackendAddress{
							{IP: "10.0.0.1"},
						},
						Ports: []discoveryv1.EndpointPort{
							{
								Port:     ptr.To(int32(80)),
								Protocol: ptr.To(corev1.ProtocolTCP),
								// Name intentionally left nil
							},
						},
					},
				},
				Service: networkingv1alpha1.ExternalProxyService{
					// Ports intentionally left empty
				},
			}

			err := defaulter.Default(ctx, externalproxy)
			Expect(err).NotTo(HaveOccurred())

			// No service ports should be generated for unnamed backend ports
			Expect(externalproxy.Spec.Service.Ports).To(BeEmpty())
		})

		It("should handle the case when there are no backends", func() {
			externalproxy.Spec = networkingv1alpha1.ExternalProxySpec{
				// No backends specified
				Service: networkingv1alpha1.ExternalProxyService{
					// Ports intentionally left empty
				},
			}

			err := defaulter.Default(ctx, externalproxy)
			Expect(err).NotTo(HaveOccurred())

			// Service name should still be defaulted
			Expect(externalproxy.Spec.Service.Name).To(Equal(TestExternalProxyName))

			// But no service ports should be generated
			Expect(externalproxy.Spec.Service.Ports).To(BeEmpty())
		})

		It("should not panic when Ingress is nil", func() {
			externalproxy.Spec = networkingv1alpha1.ExternalProxySpec{
				Service: networkingv1alpha1.ExternalProxyService{},
				// Ingress is intentionally nil
			}

			err := defaulter.Default(ctx, externalproxy)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

var _ = Describe("ExternalProxy Validator Webhook", func() {
	var (
		validator     *ExternalProxyCustomValidator
		externalproxy *networkingv1alpha1.ExternalProxy
	)

	BeforeEach(func() {
		validator = NewExternalProxyCustomValidator()
		externalproxy = &networkingv1alpha1.ExternalProxy{
			ObjectMeta: metav1.ObjectMeta{
				Name: TestExternalProxyName,
			},
		}

		Expect(validator).NotTo(BeNil(), "Expected validator to be initialized")
		Expect(externalproxy).NotTo(BeNil(), "Expected externalproxy to be initialized")
	})

	Context("When creating or updating ExternalProxy under Validating Webhook", func() {
		var (
			validBackend = []networkingv1alpha1.ExternalProxyBackend{
				{
					Addresses: []networkingv1alpha1.ExternalProxyBackendAddress{
						{IP: "10.0.0.1"},
					},
					Ports: []discoveryv1.EndpointPort{
						{
							Name:     ptr.To("http"),
							Port:     ptr.To(int32(80)),
							Protocol: ptr.To(corev1.ProtocolTCP),
						},
					},
				},
			}
			validService = networkingv1alpha1.ExternalProxyService{
				ExternalProxyServiceMeta: networkingv1alpha1.ExternalProxyServiceMeta{
					Name: TestExternalProxyName,
				},
				Ports: []corev1.ServicePort{
					{
						Name:       "http",
						Protocol:   corev1.ProtocolTCP,
						Port:       80,
						TargetPort: intstr.FromString("http"),
					},
				},
			}
			validIngress = &networkingv1alpha1.ExternalProxyIngress{
				ExternalProxyIngressMeta: networkingv1alpha1.ExternalProxyIngressMeta{
					Name: TestExternalProxyName,
				},
				Rules: []networkingv1alpha1.ExternalProxyIngressRule{
					{
						Host: "kertical.com",
						HTTP: &networkingv1alpha1.ExternalProxyIngressHttpRuleValue{
							Paths: []networkingv1alpha1.ExternalProxyIngressHttpPath{
								{
									Path:     "/",
									PathType: ptr.To(networkingv1.PathTypeImplementationSpecific),
									Backend: &networkingv1alpha1.ExternalProxyIngressBackend{
										Port: networkingv1alpha1.ExternalProxyServiceBackendPort{
											Name: "http",
										},
									},
								},
							},
						},
					},
				},
			}
		)

		Context("With valid configurations", func() {
			BeforeEach(func() {
				externalproxy.Spec = networkingv1alpha1.ExternalProxySpec{
					Backends: validBackend,
					Service:  validService,
				}
			})

			It("should validate create successfully", func() {
				warnings, err := validator.ValidateCreate(ctx, externalproxy)
				Expect(err).NotTo(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})

			It("should validate update successfully", func() {
				warnings, err := validator.ValidateUpdate(ctx, externalproxy, externalproxy)
				Expect(err).NotTo(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})

			It("should validate successfully with Ingress configuration", func() {
				externalproxy.Spec.Ingress = validIngress

				warnings, err := validator.ValidateCreate(ctx, externalproxy)
				Expect(err).NotTo(HaveOccurred())
				Expect(warnings).To(BeEmpty())
			})
		})

		Context("With invalid configurations", func() {
			It("should fail validation when Backends are missing", func() {
				externalproxy.Spec = networkingv1alpha1.ExternalProxySpec{
					Service: validService,
				}

				_, err := validator.ValidateCreate(ctx, externalproxy)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("backends"))
			})

			It("should fail validation when Backend addresses are missing", func() {
				externalproxy.Spec = networkingv1alpha1.ExternalProxySpec{
					Backends: []networkingv1alpha1.ExternalProxyBackend{
						{
							Addresses: []networkingv1alpha1.ExternalProxyBackendAddress{},
							Ports: []discoveryv1.EndpointPort{
								{
									Name:     ptr.To("http"),
									Port:     ptr.To(int32(80)),
									Protocol: ptr.To(corev1.ProtocolTCP),
								},
							},
						},
					},
					Service: validService,
				}

				_, err := validator.ValidateCreate(ctx, externalproxy)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("addresses"))
			})

			It("should fail validation when Backend ports are missing", func() {
				externalproxy.Spec = networkingv1alpha1.ExternalProxySpec{
					Backends: []networkingv1alpha1.ExternalProxyBackend{
						{
							Addresses: []networkingv1alpha1.ExternalProxyBackendAddress{
								{IP: "10.0.0.1"},
							},
							Ports: []discoveryv1.EndpointPort{},
						},
					},
					Service: validService,
				}

				_, err := validator.ValidateCreate(ctx, externalproxy)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("ports"))
			})

			It("should fail validation with duplicated port names in Backends", func() {
				externalproxy.Spec = networkingv1alpha1.ExternalProxySpec{
					Backends: []networkingv1alpha1.ExternalProxyBackend{
						{
							Addresses: []networkingv1alpha1.ExternalProxyBackendAddress{
								{IP: "10.0.0.1"},
							},
							Ports: []discoveryv1.EndpointPort{
								{
									Name:     ptr.To("http"), // Duplicate name
									Port:     ptr.To(int32(80)),
									Protocol: ptr.To(corev1.ProtocolTCP),
								},
								{
									Name:     ptr.To("http"), // Duplicate name
									Port:     ptr.To(int32(443)),
									Protocol: ptr.To(corev1.ProtocolTCP),
								},
							},
						},
					},
					Service: validService,
				}

				_, err := validator.ValidateCreate(ctx, externalproxy)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Duplicate"))
			})

			It("should fail validation with invalid IP address", func() {
				externalproxy.Spec = networkingv1alpha1.ExternalProxySpec{
					Backends: []networkingv1alpha1.ExternalProxyBackend{
						{
							Addresses: []networkingv1alpha1.ExternalProxyBackendAddress{
								{IP: "256.1.2.3"},
								{IP: "invalid-ip"},
							},
							Ports: []discoveryv1.EndpointPort{
								{
									Name:     ptr.To("http"),
									Port:     ptr.To(int32(80)),
									Protocol: ptr.To(corev1.ProtocolTCP),
								},
							},
						},
					},
					Service: validService,
				}

				_, err := validator.ValidateCreate(ctx, externalproxy)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("IP"))
			})

			It("should fail validation with unsupported protocol", func() {
				externalproxy.Spec = networkingv1alpha1.ExternalProxySpec{
					Backends: []networkingv1alpha1.ExternalProxyBackend{
						{
							Addresses: []networkingv1alpha1.ExternalProxyBackendAddress{
								{IP: "10.0.0.1"},
							},
							Ports: []discoveryv1.EndpointPort{
								{
									Name:     ptr.To("http"),
									Port:     ptr.To(int32(80)),
									Protocol: ptr.To[corev1.Protocol]("ProtocolInvalid"),
								},
							},
						},
					},
					Service: validService,
				}

				_, err := validator.ValidateCreate(ctx, externalproxy)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("protocol"))
			})

			It("should fail validation when service ports are missing", func() {
				externalproxy.Spec = networkingv1alpha1.ExternalProxySpec{
					Backends: validBackend,
					Service: networkingv1alpha1.ExternalProxyService{
						ExternalProxyServiceMeta: networkingv1alpha1.ExternalProxyServiceMeta{
							Name: TestExternalProxyName,
						},
						Ports: []corev1.ServicePort{},
					},
				}

				_, err := validator.ValidateCreate(ctx, externalproxy)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("ports"))
			})

			It("should fail validation with duplicated port names in Service", func() {
				externalproxy.Spec = networkingv1alpha1.ExternalProxySpec{
					Backends: validBackend,
					Service: networkingv1alpha1.ExternalProxyService{
						ExternalProxyServiceMeta: networkingv1alpha1.ExternalProxyServiceMeta{
							Name: TestExternalProxyName,
						},
						Ports: []corev1.ServicePort{
							{
								Name:       "http",
								Protocol:   corev1.ProtocolTCP,
								Port:       80,
								TargetPort: intstr.FromString("http"),
							},
							{
								Name:       "http",
								Protocol:   corev1.ProtocolTCP,
								Port:       443,
								TargetPort: intstr.FromString("https"),
							},
						},
					},
				}

				_, err := validator.ValidateCreate(ctx, externalproxy)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Duplicate"))
			})

			It("should fail validation with invalid Ingress configuration", func() {
				externalproxy.Spec = networkingv1alpha1.ExternalProxySpec{
					Backends: validBackend,
					Service:  validService,
					Ingress: &networkingv1alpha1.ExternalProxyIngress{
						ExternalProxyIngressMeta: networkingv1alpha1.ExternalProxyIngressMeta{
							Name: "test-ingress",
						},
						// Both defaultBackend and rules are missing, which is invalid
					},
				}

				_, err := validator.ValidateCreate(ctx, externalproxy)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("defaultBackend"))
			})
		})
	})
})
