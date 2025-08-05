package events_test

import (
	"context"
	"iter"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/wjiec/kertical/internal/events"
)

func FindPvClaim(context.Context, *corev1.PersistentVolume) iter.Seq[ctrl.Request] {
	return func(yield func(ctrl.Request) bool) {}
}

func TestReferenced(t *testing.T) {
	assert.NotNil(t, events.Referenced[*corev1.PersistentVolume](FindPvClaim))
}
