package userdata_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wjiec/kertical/internal/portforwarding/nftables/userdata"
)

func Fuzz_Marshal(f *testing.F) {
	f.Add("string")
	f.Fuzz(func(t *testing.T, comment string) {
		data := userdata.Marshal(comment)
		assert.Equal(t, comment, userdata.Unmarshal(data))
	})
}
