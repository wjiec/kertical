//go:build linux

package portforwarding_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wjiec/kertical/internal/portforwarding"
)

func TestFactory(t *testing.T) {
	pf, err := portforwarding.Factory("foo")
	if assert.NoError(t, err) {
		assert.NotNil(t, pf)
	}
}
