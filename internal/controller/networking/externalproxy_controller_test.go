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
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	networkingv1alpha1 "github.com/wjiec/kertical/api/networking/v1alpha1"
	"github.com/wjiec/kertical/internal/controller/testsuite"
	networkingwebhook "github.com/wjiec/kertical/internal/webhook/networking/v1alpha1"
)

var _ = Describe("ExternalProxy Controller", func() {
	var (
		ctx           context.Context
		reconciler    *ExternalProxyReconciler
		resourceKey   types.NamespacedName
		externalproxy *networkingv1alpha1.ExternalProxy
	)

	BeforeEach(func() {
		ctx = context.Background()
		ctx = logf.IntoContext(ctx, logr.Discard())

		By("create and setup ExternalProxy controller")
		reconciler = newExternalProxyReconciler(testsuite.KubeClient, testsuite.KubeClient.Scheme())
		Expect(reconciler).NotTo(BeNil())

		externalproxy = &networkingv1alpha1.ExternalProxy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-externalproxy",
				Namespace: "default",
			},
			Spec: networkingv1alpha1.ExternalProxySpec{
				Backends: []networkingv1alpha1.ExternalProxyBackend{
					{
						Addresses: []networkingv1alpha1.ExternalProxyBackendAddress{
							{IP: "10.0.0.1"},
							{IP: "fc00::f090:27ff:fee2:420a"},
						},
						Ports: []discoveryv1.EndpointPort{
							{Name: ptr.To("http"), Port: ptr.To(int32(8080))},
						},
					},
					{
						Addresses: []networkingv1alpha1.ExternalProxyBackendAddress{
							{IP: "10.0.0.2"},
						},
						Ports: []discoveryv1.EndpointPort{
							{Name: ptr.To("https"), Port: ptr.To(int32(8443))},
						},
					},
				},
				Ingress: &networkingv1alpha1.ExternalProxyIngress{
					Rules: []networkingv1alpha1.ExternalProxyIngressRule{
						{
							Host: "test.kertical.com",
							HTTP: &networkingv1alpha1.ExternalProxyIngressHttpRuleValue{
								Paths: []networkingv1alpha1.ExternalProxyIngressHttpPath{
									{
										Path:     "/",
										PathType: ptr.To(networkingv1.PathTypePrefix),
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
				},
			},
		}
		Expect(externalproxy).NotTo(BeNil())

		resourceKey = runtimeclient.ObjectKeyFromObject(externalproxy)
		Expect(externalproxy).NotTo(BeZero())
	})

	BeforeEach(func() {
		By("defaulting for ExternalProxy")
		admissionCtx := admission.NewContextWithRequest(ctx, admission.Request{})
		err := networkingwebhook.NewExternalProxyCustomDefaulter(testsuite.KubeClient.Scheme()).
			Default(admissionCtx, externalproxy)
		Expect(err).NotTo(HaveOccurred())
	})

	BeforeEach(func() {
		By("creating the custom resource for the kind ExternalProxy")
		err := testsuite.KubeClient.Get(ctx, resourceKey, externalproxy)
		if Expect(err).To(HaveOccurred()) && Expect(errors.IsNotFound(err)).To(BeTrue()) {
			err = testsuite.KubeClient.Create(ctx, externalproxy)
			Expect(err).To(Succeed())
		}
	})

	AfterEach(func() {
		var newExternalProxy networkingv1alpha1.ExternalProxy
		err := testsuite.KubeClient.Get(ctx, resourceKey, &newExternalProxy)
		Expect(err).NotTo(HaveOccurred())

		By("cleanup the specific resource instance ExternalProxy")
		err = testsuite.KubeClient.Delete(ctx, &newExternalProxy)
		Expect(err).To(Succeed())
	})

	Context("When reconciling a resource", func() {
		BeforeEach(func() {
			err := wait.PollUntilContextTimeout(ctx, time.Second, time.Minute, false, func(ctx context.Context) (done bool, err error) {
				var foundExternalProxy networkingv1alpha1.ExternalProxy
				err = testsuite.KubeClient.Get(ctx, resourceKey, &foundExternalProxy)
				return err == nil, runtimeclient.IgnoreNotFound(err)
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("should reconciled succeed", func() {
			By("reconciling the externalproxy resource")
			result, err := reconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: resourceKey,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))
		})
	})
})
