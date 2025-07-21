package sysctl

import (
	"os"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

// NetIPv4Forward checks if IPv4 forwarding is enabled in the Linux kernel.
//
// IP forwarding is required for port forwarding to work, as it allows the system
// to forward packets between network interfaces.
func NetIPv4Forward() (int, error) {
	content, err := os.ReadFile("/proc/sys/net/ipv4/ip_forward")
	if err != nil {
		return 0, errors.Wrapf(err, "failed to read /proc/sys/net/ipv4/ip_forward")
	}

	number, err := strconv.Atoi(strings.TrimSpace(string(content)))
	if err != nil {
		return 0, errors.Wrapf(err, "bad value from /proc/sys/net/ipv4/ip_forward")
	}
	return number, nil
}
