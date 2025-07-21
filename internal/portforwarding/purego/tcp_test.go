package purego

import (
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_forwardTCP(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	if assert.NoError(t, forwardTCP(ctx, 56789, "1.1.1.1:80")) {
		resp, err := http.Get("http://localhost:56789")
		if assert.NoError(t, err) {
			defer func() { _ = resp.Body.Close() }()

			content, err := io.ReadAll(resp.Body)
			if assert.NoError(t, err) {
				assert.NotEmpty(t, content)
			}
		}
	}
}
