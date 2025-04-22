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

package utils

import "sigs.k8s.io/controller-runtime/pkg/client"

// LastAppliedConfigAnnotation is the annotation used to store the previous
// configuration of a resource.
const LastAppliedConfigAnnotation = "kubectl.kubernetes.io/last-applied-configuration"

// GetLastAppliedConfiguration retrieves the last applied configuration stored in the annotations of the given object.
func GetLastAppliedConfiguration(object client.Object) (string, bool) {
	lastAppliedConfiguration := object.GetAnnotations()[LastAppliedConfigAnnotation]
	return lastAppliedConfiguration, len(lastAppliedConfiguration) > 0
}
