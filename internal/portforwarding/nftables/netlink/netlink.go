package netlink

import (
	"os"
	"syscall"

	"github.com/mdlayher/netlink"
	"github.com/pkg/errors"
)

// IsError checks if the given error is a netlink operation error with a specific error number.
// It unwraps the error to see if it's a [netlink.OpError] and then checks if the underlying
// error matches the specified errno.
func IsError(err error, errno error) bool {
	var opError *netlink.OpError
	if errors.As(err, &opError) {
		return errors.Is(opError.Err, errno)
	}
	return false
}

// IgnoreNotFound returns nil if the error is nil or represents a "not found" error.
// Otherwise, returns the original error.
func IgnoreNotFound(err error) error {
	if err == nil {
		return nil
	}

	var opError *netlink.OpError
	if errors.As(err, &opError) {
		if os.IsNotExist(opError.Err) {
			return nil
		}
	}
	return err
}

// IgnoreBusy suppresses EBUSY (device or resource busy) errors and returns nil,
// but passes through any other errors unchanged.
func IgnoreBusy(err error) error {
	if err == nil || IsError(err, syscall.EBUSY) {
		return nil
	}
	return err
}
