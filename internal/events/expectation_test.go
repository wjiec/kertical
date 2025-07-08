package events_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/wjiec/kertical/internal/events"
	"github.com/wjiec/kertical/internal/expectations"
)

func TestOwnedWithExpectation(t *testing.T) {
	assert.NotNil(t, events.OwnedWithExpectation(
		expectations.NewControllerExpectations(),
		events.GvkResolver[*corev1.Pod](v1.SchemeGroupVersion.String(), "Deployment"),
	))
}
