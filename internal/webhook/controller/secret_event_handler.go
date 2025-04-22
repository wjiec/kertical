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
	"context"
	"iter"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/wjiec/kertical/internal/events"
	"github.com/wjiec/kertical/internal/webhook/controller/utils"
)

type SecretEventHandler[T client.Object] struct {
	events.Nothing // Provides no-op implementations of event handlers

	r      client.Reader
	lister RefObjectLister[T]
}

func NewSecretEventHandler[T client.Object](r client.Reader, lister RefObjectLister[T]) *SecretEventHandler[T] {
	return &SecretEventHandler[T]{r: r, lister: lister}
}

func (s *SecretEventHandler[T]) Create(ctx context.Context, evt events.CreateEvent, wq events.RateLimiting) {
	if evt.Object.GetDeletionTimestamp() != nil {
		s.Delete(ctx, event.TypedDeleteEvent[client.Object]{Object: evt.Object}, wq)
		return
	}
}

func (s *SecretEventHandler[T]) Update(ctx context.Context, evt events.UpdateEvent, wq events.RateLimiting) {
	if evt.ObjectNew.GetDeletionTimestamp() != nil {
		s.Delete(ctx, event.TypedDeleteEvent[client.Object]{Object: evt.ObjectNew}, wq)
		return
	}
}

type RefObjectLister[T client.Object] func(*corev1.Service) ([]T, error)

func (s *SecretEventHandler[T]) resolveAssociatedWebhook(ctx context.Context, secret *corev1.Secret, wq events.RateLimiting) {
	logger := log.FromContext(ctx).WithValues("secret", klog.KObj(secret))
	if secret.Type != corev1.SecretTypeTLS || secret.Data[utils.TLSCACertKey] == nil {
		return
	}

	var podList corev1.PodList
	if err := s.r.List(ctx, &podList, client.InNamespace(secret.Namespace)); err != nil {
		logger.Error(err, "failed to list pods")
		return
	}

	var serviceList corev1.ServiceList
	if err := s.r.List(ctx, &serviceList, client.InNamespace(secret.Namespace)); err != nil {
		logger.Error(err, "failed to list services")
		return
	}

	serviceSeen := sets.New[types.NamespacedName]()
	for _, pod := range podList.Items {
		if !hasSecretVolume(pod.Spec.Volumes, secret.Name) {
			continue
		}

		for matchedService := range iterMatchedService(&serviceList, pod.Labels) {
			if serviceSeen.Has(client.ObjectKeyFromObject(matchedService)) {
				continue
			}

			serviceSeen.Insert(client.ObjectKeyFromObject(matchedService))
			logger.Info("found matched service", "pod", klog.KObj(&pod), "service", klog.KObj(matchedService))

			targetObjects, err := s.lister(matchedService)
			if err != nil {
				logger.Error(err, "failed to list target objects", "pod", klog.KObj(&pod), "service", klog.KObj(matchedService))
				continue
			}

			for _, target := range targetObjects {
				wq.Add(ctrl.Request{NamespacedName: client.ObjectKeyFromObject(target)})
			}
		}
	}
}

// namespacePredicate creates a predicate that filters Kubernetes objects based on namespace.
func namespacePredicate(namespace string) predicate.Predicate {
	return predicate.NewPredicateFuncs(func(object client.Object) bool {
		return object.GetNamespace() == namespace
	})
}

// hasSecretVolume checks whether a list of volumes contains a secret with the specified name.
//
// It returns true if such a secret volume is found and false otherwise.
func hasSecretVolume(volumes []corev1.Volume, secretName string) bool {
	for _, volume := range volumes {
		if volume.Secret != nil && volume.Secret.SecretName == secretName {
			return true
		}
	}
	return false
}

// iterMatchedService generates an iterator over services that match a specific set of pod labels.
// It returns an [iter.Seq] for services whose selectors match the provided pod labels.
func iterMatchedService(services *corev1.ServiceList, podLabel map[string]string) iter.Seq[*corev1.Service] {
	return func(yield func(*corev1.Service) bool) {
		for _, service := range services.Items {
			selector := labels.SelectorFromSet(service.Spec.Selector)
			if selector.Matches(labels.Set(podLabel)) {
				if !yield(&service) {
					return
				}
			}
		}
	}
}
