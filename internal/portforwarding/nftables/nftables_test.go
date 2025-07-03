//go:build linux

package nftables

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAvailable(t *testing.T) {
	_, err := Available()
	assert.NoError(t, err)
}

func Fuzz_marshalUserComment(f *testing.F) {
	f.Add("string")
	f.Fuzz(func(t *testing.T, comment string) {
		data := marshalUserComment(comment)
		assert.Equal(t, comment, unmarshalUserComment(data))
	})
}
