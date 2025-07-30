package portforwarding_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wjiec/kertical/internal/portforwarding"
)

func TestNew(t *testing.T) {
	pf, err := portforwarding.New("foo")
	if assert.NoError(t, err) {
		assert.NotNil(t, pf)
	}
}
