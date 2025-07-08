//go:build linux

package sysctl_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wjiec/kertical/internal/portforwarding/nftables/sysctl"
)

func TestNetIPv4Forward(t *testing.T) {
	forwarding, err := sysctl.NetIPv4Forward()
	if assert.NoError(t, err) {
		assert.Contains(t, []int{0, 1}, forwarding)
	}
}
