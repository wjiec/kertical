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

import (
	"context"
	stderrors "errors"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// DefaultContainerAnnotation is the annotation used to specify the default container in a pod.
	DefaultContainerAnnotation = "kubectl.kubernetes.io/default-container"

	// TLSCACertKey is the key for CA certificate in a TLS secret.
	TLSCACertKey = "ca.crt"
)

var (
	ErrNoAssociatedWorkloadFound = stderrors.New("no associated workload found")
	ErrNoAssociatedVolumeFound   = stderrors.New("no associated volume found")
	ErrInvalidSecretProvided     = stderrors.New("invalid secret provided")
)

// DefaultContainer returns the default container specified by an annotation on the given object.
// If the annotation is not found, it defaults to returning the first container in the slice.
func DefaultContainer(object client.Object, containers []corev1.Container) corev1.Container {
	if name, found := object.GetAnnotations()[DefaultContainerAnnotation]; found {
		for _, container := range containers {
			if name == container.Name {
				return container
			}
		}
	}
	return containers[0]
}

// MountedVolume checks if a given path is mounted in the specified container and returns the name of the volume.
//
// It returns the name of the mounted volume and a boolean indicating whether the mount path was found.
func MountedVolume(container *corev1.Container, path string) (string, bool) {
	for _, mountedVolume := range container.VolumeMounts {
		if mountedVolume.MountPath == path {
			return mountedVolume.Name, true
		}
	}

	return "", false
}

// ExtractCaCertFromSecret reads the secret data to obtain the CA certificate bundle located within secrets.
func ExtractCaCertFromSecret(ctx context.Context, r client.Reader, secretKey types.NamespacedName) ([]byte, error) {
	var secret corev1.Secret
	if err := r.Get(ctx, secretKey, &secret); err != nil {
		return nil, err
	}

	if cert, found := secret.Data[TLSCACertKey]; found {
		return cert, nil
	}

	return nil, ErrInvalidSecretProvided
}

// FindMountedVolumeByService determines the volume mounted at a given path and extracts the secret associated with it.
func FindMountedVolumeByService(ctx context.Context, r client.Reader, serviceKey types.NamespacedName, mountPath string) (types.NamespacedName, error) {
	var service corev1.Service
	if err := r.Get(ctx, serviceKey, &service); err != nil {
		return types.NamespacedName{}, err
	}

	var podList corev1.PodList
	if err := r.List(ctx, &podList, client.InNamespace(service.Namespace), client.MatchingLabels(service.Spec.Selector)); err != nil {
		return types.NamespacedName{}, err
	}

	if len(podList.Items) == 0 {
		return types.NamespacedName{}, ErrNoAssociatedWorkloadFound
	}

	var secretKey types.NamespacedName
	for _, pod := range podList.Items {
		defaultContainer := DefaultContainer(&pod, pod.Spec.Containers)
		if volumeName, found := MountedVolume(&defaultContainer, mountPath); found {
			for _, volume := range pod.Spec.Volumes {
				if volume.Name == volumeName && volume.Secret != nil {
					secretKey.Namespace = pod.Namespace
					secretKey.Name = volume.Secret.SecretName
					break
				}
			}
		}

		if len(secretKey.Name) != 0 {
			break
		}
	}

	if len(secretKey.Name) == 0 {
		return types.NamespacedName{}, ErrNoAssociatedVolumeFound
	}

	return secretKey, nil
}
