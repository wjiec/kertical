package purego_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/wjiec/kertical/internal/portforwarding/purego"
)

func TestNew(t *testing.T) {
	assert.NotNil(t, purego.New("foobar"))
}
