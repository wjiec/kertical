//go:build linux

package nftables_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wjiec/kertical/internal/portforwarding/nftables"
)

func TestAvailable(t *testing.T) {
	_, err := nftables.Available()
	assert.NoError(t, err)
}
