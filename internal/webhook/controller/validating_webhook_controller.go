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

package controller

import (
	"bytes"
	"context"
	stderrors "errors"
	"time"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/wjiec/kertical/internal/kertical"
	"github.com/wjiec/kertical/internal/verbosity"
	"github.com/wjiec/kertical/internal/webhook"
	"github.com/wjiec/kertical/internal/webhook/controller/utils"
)

// ValidatingWebhookReconciler is responsible for ensuring ValidatingWebhookConfiguration
// objects have valid CA bundles set in their webhook configurations by extracting the
// required certificates from associated secrets.
type ValidatingWebhookReconciler struct {
	client client.Client
	scheme *runtime.Scheme
}

// NewValidatingWebhookReconciler constructs and returns a new instance of ValidatingWebhookReconciler.
func NewValidatingWebhookReconciler(client client.Client, scheme *runtime.Scheme) *ValidatingWebhookReconciler {
	return &ValidatingWebhookReconciler{client: client, scheme: scheme}
}

// Reconcile ensures that for each ValidatingWebhookConfiguration, the CA bundle from the appropriate secret is up-to-date.
func (v *ValidatingWebhookReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	defer func(start time.Time) {
		logger.V(verbosity.Verbose).Info("Finished reconcile", "duration", time.Since(start))
	}(time.Now())

	var instance admissionregistrationv1.ValidatingWebhookConfiguration
	if err := v.client.Get(ctx, req.NamespacedName, &instance); err != nil {
		if errors.IsNotFound(err) {
			logger.V(verbosity.Verbose).Info("Resource has been deleted")
		}

		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var shouldUpdate bool
	for i, elem := range instance.Webhooks {
		if svc := elem.ClientConfig.Service; svc != nil {
			if svc.Namespace != kertical.GetNamespace() {
				logger.Info("Namespace unmatched, skipping reconciling webhook", "webhook", elem.Name)
				continue
			}

			svcKey := types.NamespacedName{Namespace: svc.Namespace, Name: svc.Name}
			logger.Info("Start reconciling webhook", "webhook", elem.Name)
			secretKey, err := utils.FindMountedVolumeByService(ctx, v.client, svcKey, webhook.GetCertDir())
			if err != nil {
				// Handle case where associated workload is not yet created
				if stderrors.Is(err, utils.ErrNoAssociatedWorkloadFound) {
					// Wait a short period and requeue attempt
					return ctrl.Result{Requeue: true, RequeueAfter: 3 * time.Second}, err
				}

				return ctrl.Result{}, err
			}

			// Extract CA certificate from the resolved secret and inject into webhook
			caCert, err := utils.ExtractCaCertFromSecret(ctx, v.client, secretKey)
			if err != nil {
				return ctrl.Result{}, err
			}

			if !bytes.Equal(caCert, elem.ClientConfig.CABundle) {
				shouldUpdate = true
				instance.Webhooks[i].ClientConfig.CABundle = caCert
			}
		}
	}

	if shouldUpdate {
		if err := v.client.Update(ctx, &instance); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager registers this reconciler with the provided manager, configuring predicates and watches.
func (v *ValidatingWebhookReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("validating-webhook").
		For(&admissionregistrationv1.ValidatingWebhookConfiguration{}).
		Watches(&corev1.Secret{}, NewSecretEventHandler(v.client, ListValidatingWebhookByService(v.client)),
			builder.WithPredicates(namespacePredicate(kertical.GetNamespace()))).
		Complete(v)
}

type ValidatingWebhookLister = RefObjectLister[*admissionregistrationv1.ValidatingWebhookConfiguration]

func ListValidatingWebhookByService(r client.Reader) ValidatingWebhookLister {
	return func(service *corev1.Service) ([]*admissionregistrationv1.ValidatingWebhookConfiguration, error) {
		var webhookList admissionregistrationv1.ValidatingWebhookConfigurationList
		if err := r.List(context.Background(), &webhookList); err != nil {
			return nil, err
		}

		var res []*admissionregistrationv1.ValidatingWebhookConfiguration
		for _, validatingWebhook := range webhookList.Items {
			for _, elem := range validatingWebhook.Webhooks {
				if isMatchedWebhookService(&elem.ClientConfig, service) {
					res = append(res, &validatingWebhook)
					break
				}
			}
		}

		return res, nil
	}
}
