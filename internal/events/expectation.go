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
	"time"

	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/wjiec/kertical/internal/expectations"
	"github.com/wjiec/kertical/internal/verbosity"
)

var (
	// initialingRateLimiter calculates the delay duration for existing resources
	// triggered Create event when the Informer cache has just synced.
	initialingRateLimiter = workqueue.NewTypedItemExponentialFailureRateLimiter[ctrl.Request](3*time.Second, 30*time.Second)
)

var _ EventHandler = (*ownedByExpectation[client.Object])(nil)

// ownedByExpectation determines whether processing is required based on the
// expectations for the current resource according to its owner.
type ownedByExpectation[T client.Object] struct {
	Nothing

	expectations          expectations.ControllerExpectations
	controllerRefResolver ControllerRefResolver[T]
}

// OwnedWithExpectation creates a new instance of ownedByExpectation, which manages
// expectations for resources owned by a specific controller.
func OwnedWithExpectation[T client.Object](exp expectations.ControllerExpectations, resolver ControllerRefResolver[T]) EventHandler {
	return &ownedByExpectation[T]{
		expectations:          exp,
		controllerRefResolver: resolver,
	}
}

// Create handles the creation event for a resource and updates the work queue appropriately.
func (g *ownedByExpectation[T]) Create(ctx context.Context, evt CreateEvent, wq RateLimiting) {
	logger := log.FromContext(ctx)
	if evt.Object.GetDeletionTimestamp() != nil {
		g.Delete(ctx, event.TypedDeleteEvent[client.Object]{Object: evt.Object}, wq)
		return
	}

	req := g.controllerRefResolver(ctx, evt.Object.(T))
	if req == nil {
		return
	}

	// We should check if the expectations for req are already satisfied before triggering an observe event.
	// If expectations are satisfied, there's no need to trigger a reconcile immediately.
	// This can reduce the load on the controller and API server.
	isSatisfied, _ := g.expectations.SatisfiedExpectations(req.String())
	logger.V(verbosity.VeryVerbose).Info("observed object creation", "object", klog.KObj(evt.Object))
	g.expectations.Observe(req.String(), expectations.ActionCreations, evt.Object.GetName())

	if isSatisfied {
		// If the expectation is satisfied, it should be an existing resource and the Informer
		// cache should have just synced.
		wq.AddAfter(*req, initialingRateLimiter.When(*req))
	} else {
		// Otherwise, add it immediately and reset the rate limiter
		initialingRateLimiter.Forget(*req)
		wq.Add(*req)
	}
}

// Update handles the update event for a resource.
func (g *ownedByExpectation[T]) Update(ctx context.Context, evt UpdateEvent, wq RateLimiting) {
	if evt.ObjectNew.GetDeletionTimestamp() != nil {
		g.Delete(ctx, event.TypedDeleteEvent[client.Object]{Object: evt.ObjectNew}, wq)
		return
	}
}

// Delete handles the deletion event for a resource.
func (g *ownedByExpectation[T]) Delete(ctx context.Context, evt DeleteEvent, wq RateLimiting) {
	logger := log.FromContext(ctx)
	if _, ok := evt.Object.(T); !ok {
		logger.Error(nil, "skipping deletion event", "deleteStateUnknown", evt.DeleteStateUnknown, "object", klog.KObj(evt.Object))
		return
	}

	logger.Info("deleted object", "object", klog.KObj(evt.Object))
	if req := g.controllerRefResolver(ctx, evt.Object.(T)); req != nil {
		logger.Info("deleted resolved ref", "object", klog.KObj(evt.Object), "ref", req)

		g.expectations.Observe(req.String(), expectations.ActionDeletions, evt.Object.GetName())
		wq.Add(*req)
	}
}
