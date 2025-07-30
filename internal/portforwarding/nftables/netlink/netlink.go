package netlink

import (
	"os"
)

// IgnoreNotFound returns nil if the error is nil or represents a "not found" error.
// Otherwise, returns the original error.
func IgnoreNotFound(err error) error {
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
