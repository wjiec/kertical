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

package portforwarding

import (
	"context"

	discoveryv1 "k8s.io/api/discovery/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// IndexServiceOwnerReference is the index key for looking
	// up [discoveryv1.EndpointSlice] by their owning Service.
	IndexServiceOwnerReference = "metadata.labels.service-name"
)

// serviceOwnerReference extracts the service name from a Kubernetes object's labels.
//
// It returns the value of the "kubernetes.io/service-name" label if present.
func serviceOwnerReference(object client.Object) []string {
	owners := make([]string, 0, 1)
	if owner, found := object.GetLabels()[discoveryv1.LabelServiceName]; found {
		owners = append(owners, owner)
	}

	return owners
}

// RegisterFieldIndexes sets up custom field indexes in the Kubernetes client cache.
func RegisterFieldIndexes(ctx context.Context, indexer client.FieldIndexer) error {
	return indexer.IndexField(ctx, &discoveryv1.EndpointSlice{}, IndexServiceOwnerReference, serviceOwnerReference)
}
