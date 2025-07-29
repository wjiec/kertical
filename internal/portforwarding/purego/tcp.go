package purego

import (
	"context"
	"fmt"
	"io"
	"net"

	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// TcpDialer is a function type that creates new TCP connections.
type TcpDialer func() (net.Conn, error)

// forwardTCP creates a TCP proxy that forwards connections
// from the specified port to the target address.
func forwardTCP(ctx context.Context, port uint16, dialer TcpDialer) error {
	listen, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}

	incoming := make(chan net.Conn)
	go func() {
		for {
			conn, err := listen.Accept()
			if err != nil {
				if !errors.Is(err, net.ErrClosed) {
					log.FromContext(ctx).Error(err, "failed to accepting incoming connection")
				}
				return
			}
			incoming <- conn
		}
	}()

	go func() {
		defer func() { _ = listen.Close() }()

		for {
			select {
			case <-ctx.Done():
				return
			case conn := <-incoming:
				go func() {
					defer func() { _ = conn.Close() }()

					outgoing, err := dialer()
					if err != nil {
						log.FromContext(ctx).Error(err, "failed to connect to target")
						return
					}
					defer func() { _ = outgoing.Close() }()

					// Bidirectionally forward data between incoming and outgoing connections
					if err = pipe(ctx, conn, outgoing); err != nil {
						log.FromContext(ctx).Error(err, "failed to forwarding data")
					}
				}()
			}
		}
	}()
	return nil
}

// pipe establishes a bidirectional data flow between two ReadWriters with context awareness.
//
// The function will terminate when the context is canceled or when an error occurs during copying.
func pipe(ctx context.Context, dst, src io.ReadWriter) error {
	var eg errgroup.Group
	eg.Go(func() error { _, err := io.Copy(dst, &ctxReader{Context: ctx, underlying: src}); return err })
	eg.Go(func() error { _, err := io.Copy(src, &ctxReader{Context: ctx, underlying: dst}); return err })

	return eg.Wait()
}

// ctxReader wraps an [io.Reader] with context awareness to enable cancellation.
type ctxReader struct {
	context.Context
	underlying io.Reader
}

// Read implements the [io.Reader] interface with context cancellation support.
func (c *ctxReader) Read(p []byte) (int, error) {
	select {
	// If context is canceled, return the context error
	case <-c.Done():
		return 0, c.Err()

	// Otherwise, proceed with reading from the underlying reader
	default:
		return c.underlying.Read(p)
	}
}
