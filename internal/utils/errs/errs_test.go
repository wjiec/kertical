package errs

import (
	"io"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestIgnore(t *testing.T) {
	t.Run("happy", func(t *testing.T) {
		assert.Nil(t, Ignore(io.EOF, io.EOF))
	})

	t.Run("wrapped", func(t *testing.T) {
		assert.Nil(t, Ignore(errors.Wrap(io.EOF, "wrapped"), io.EOF))
		assert.Nil(t, Ignore(errors.Wrap(io.EOF, "wrapped"), io.EOF))
	})

	t.Run("nil", func(t *testing.T) {
		assert.Nil(t, Ignore(nil, io.EOF))
	})
}
