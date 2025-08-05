package events

import (
	"context"
	"iter"

	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var _ EventHandler = (*referenced[client.Object])(nil)

// referenced implements EventHandler for resources that are referenced
// by other resources rather than owned through controller references.
type referenced[T client.Object] struct {
	Nothing

	referenceResolver ReferenceResolver[T]
}

// Referenced creates a new EventHandler for resources that are referenced by other resources.
func Referenced[T client.Object](resolver ReferenceResolver[T]) EventHandler {
	return &referenced[T]{referenceResolver: resolver}
}

// Create handles creation events for referenced resources
func (r *referenced[T]) Create(ctx context.Context, evt CreateEvent, wq RateLimiting) {
	if evt.Object.GetDeletionTimestamp() != nil {
		r.Delete(ctx, event.TypedDeleteEvent[client.Object]{Object: evt.Object}, wq)
		return
	}
	r.resolve(ctx, evt.Object.(T), wq)
}

// Update handles update events for referenced resources
func (r *referenced[T]) Update(ctx context.Context, evt UpdateEvent, wq RateLimiting) {
	if evt.ObjectNew.GetDeletionTimestamp() != nil {
		r.Delete(ctx, event.TypedDeleteEvent[client.Object]{Object: evt.ObjectNew}, wq)
		return
	}
	r.resolve(ctx, evt.ObjectNew.(T), wq)
}

// Delete handles deletion events for referenced resources
func (r *referenced[T]) Delete(ctx context.Context, evt DeleteEvent, wq RateLimiting) {
	logger := log.FromContext(ctx)
	if _, ok := evt.Object.(T); !ok {
		logger.Error(nil, "skipping deletion event",
			"deleteStateUnknown", evt.DeleteStateUnknown,
			"object", klog.KObj(evt.Object))
		return
	}

	logger.Info("deleted object", "object", klog.KObj(evt.Object))
	r.resolve(ctx, evt.Object.(T), wq)
}

// ReferenceResolver resolves references from an object to a sequence of reconciliation requests.
type ReferenceResolver[T client.Object] func(ctx context.Context, object T) iter.Seq[ctrl.Request]

// resolve uses the reference resolver to determine which controller should handle the object
func (r *referenced[T]) resolve(ctx context.Context, object T, wq RateLimiting) {
	for req := range r.referenceResolver(ctx, object) {
		wq.Add(req)
	}
}
