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

package fieldindex

import (
	"context"
	"errors"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// IndexOwnerReferenceUID defines the index key for owner reference UID lookups
	IndexOwnerReferenceUID = "metadata.controller.uid"
)

// ownerReferenceUIDIndex creates an index based on owner reference UIDs
func ownerReferenceUIDIndex(object client.Object) []string {
	owners := make([]string, 1) // a slices of size 1 is sufficient
	for _, owner := range object.GetOwnerReferences() {
		owners = append(owners, string(owner.UID))
	}

	return owners
}

// RegisterFieldIndexes registers field indexes for various Kubernetes resources
func RegisterFieldIndexes(ctx context.Context, indexer client.FieldIndexer) (err error) {
	err = errors.Join(err, indexer.IndexField(ctx, &corev1.Service{}, IndexOwnerReferenceUID, ownerReferenceUIDIndex))
	err = errors.Join(err, indexer.IndexField(ctx, &discoveryv1.EndpointSlice{}, IndexOwnerReferenceUID, ownerReferenceUIDIndex))
	err = errors.Join(err, indexer.IndexField(ctx, &networkingv1.Ingress{}, IndexOwnerReferenceUID, ownerReferenceUIDIndex))

	return err
}
