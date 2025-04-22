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
	"context"

	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

type CreateEvent = event.TypedCreateEvent[client.Object]
type UpdateEvent = event.TypedUpdateEvent[client.Object]
type DeleteEvent = event.TypedDeleteEvent[client.Object]
type GenericEvent = event.TypedGenericEvent[client.Object]
type RateLimiting = workqueue.TypedRateLimitingInterface[ctrl.Request]

type EventHandler = handler.TypedEventHandler[client.Object, ctrl.Request]

// Nothing is a struct implementing no-op handlers for various event types.
// It effectively does nothing, serving as a placeholder for event handling interfaces.
type Nothing struct{}

var _ EventHandler = (*Nothing)(nil)

func (Nothing) Create(context.Context, CreateEvent, RateLimiting)   {}
func (Nothing) Update(context.Context, UpdateEvent, RateLimiting)   {}
func (Nothing) Delete(context.Context, DeleteEvent, RateLimiting)   {}
func (Nothing) Generic(context.Context, GenericEvent, RateLimiting) {}

// ControllerRefResolver to resolves a controller reference and returns
// a *ctrl.Request based on the provided client object.
type ControllerRefResolver[T client.Object] func(ctx context.Context, object T) *ctrl.Request

// GvkResolver constructs a ControllerRefResolver that resolves controller references based on the given
// API version and kind. It returns a function that can resolve these references for the provided objects.
func GvkResolver[T client.Object](apiVersion, kind string) ControllerRefResolver[T] {
	return func(ctx context.Context, object T) *ctrl.Request {
		return ResolveControllerRef(object, apiVersion, kind)
	}
}
