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

package webhook

import (
	"os"

	ctrl "sigs.k8s.io/controller-runtime"

	networkingv1alpha1 "github.com/wjiec/kertical/internal/webhook/networking/v1alpha1"
)

// webhookSetups is a slice to hold functions that set up webhooks with the manager.
var webhookSetups []func(mgr ctrl.Manager) error

func init() {
	webhookSetups = append(webhookSetups, networkingv1alpha1.SetupExternalProxyWebhookWithManager)
	webhookSetups = append(webhookSetups, networkingv1alpha1.SetupPortForwardingWebhookWithManager)
}

// +kubebuilder:webhookconfiguration:mutating=true,name=mutating-webhook
// +kubebuilder:webhookconfiguration:mutating=false,name=validating-webhook

// SetupWebhookWithManager executes each webhook setup function in the slice with the given manager.
//
// Returns an error if any webhook fails to initialize
func SetupWebhookWithManager(mgr ctrl.Manager) error {
	for _, setupWebhook := range webhookSetups {
		if err := setupWebhook(mgr); err != nil {
			return err
		}
	}

	return nil
}

// GetCertDir retrieves the directory path for storing webhook certificates.
func GetCertDir() string {
	if env := os.Getenv("WEBHOOK_CERT_DIR"); len(env) > 0 {
		return env
	}

	return "/tmp/kertical-webhook-certs"
}
