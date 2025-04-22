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
	"context"

	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	networkingv1alpha1 "github.com/wjiec/kertical/api/networking/v1alpha1"
)

// StatusUpdater defines an interface for updating the status of ExternalProxy objects.
type StatusUpdater interface {
	UpdateStatus(context.Context, *networkingv1alpha1.ExternalProxy, *networkingv1alpha1.ExternalProxyStatus) error
}

// realStatusUpdater is a concrete implementation of the StatusUpdater interface.
type realStatusUpdater struct {
	c client.Client
}

// NewStatusUpdater creates a new instance of StatusUpdater with the provided client.
func NewStatusUpdater(c client.Client) StatusUpdater {
	return &realStatusUpdater{c: c}
}

// UpdateStatus updates the status of a given ExternalProxy resource.
func (su *realStatusUpdater) UpdateStatus(ctx context.Context, ep *networkingv1alpha1.ExternalProxy, newStatus *networkingv1alpha1.ExternalProxyStatus) error {
	logger := log.FromContext(ctx)

	logger.Info("updating external proxy status", "ready", newStatus.Ready, "serviceName", newStatus.ServiceName)
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var newObject networkingv1alpha1.ExternalProxy
		// Retrieve the latest version of the external proxy from the API server.
		if err := su.c.Get(ctx, client.ObjectKeyFromObject(ep), &newObject); err != nil {
			return err
		}

		newStatus.DeepCopyInto(&newObject.Status)
		return su.c.Status().Update(ctx, &newObject)
	})
}
