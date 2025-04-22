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

package events

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ResolveControllerRef checks the controller reference of the given
// object and converts it to a controller request.
func ResolveControllerRef(object metav1.Object, apiVersion, kind string) *ctrl.Request {
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		if ownerRef.APIVersion == apiVersion && ownerRef.Kind == kind {
			return &ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: object.GetNamespace(),
					Name:      ownerRef.Name,
				},
			}
		}
	}

	return nil
}
