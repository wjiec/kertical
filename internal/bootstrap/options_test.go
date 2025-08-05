package bootstrap_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/wjiec/kertical/internal/bootstrap"
)

func TestWithBeforeCreate(t *testing.T) {
	assert.NotNil(t, bootstrap.WithBeforeCreate(func(context.Context) error {
		return nil
	}))
}

func TestWithBeforeStart(t *testing.T) {
	assert.NotNil(t, bootstrap.WithBeforeStart(func(context.Context, ctrl.Manager) error {
		return nil
	}))
}

func TestWithBeforeStop(t *testing.T) {
	assert.NotNil(t, bootstrap.WithBeforeStop(func(context.Context, ctrl.Manager) error {
		return nil
	}))
}
