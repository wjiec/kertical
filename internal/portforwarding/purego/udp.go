package purego

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// writeMsg represents a UDP message to be written
type writeMsg struct {
	addr net.Addr
	buf  []byte
}

// forwardUDP creates a UDP proxy that listens on the specified port and
// forwards traffic to the target address.
func forwardUDP(ctx context.Context, port uint16, target string) error {
	dstAddr, err := net.ResolveUDPAddr("udp", target)
	if err != nil {
		return err
	}

	conn, err := net.ListenPacket("udp", fmt.Sprintf(":%d", port))
	if err != nil {
		return err
	}

	// Create a buffered channel for queuing outbound messages
	writeQueue := make(chan *writeMsg, 256)
	go func() {
		defer close(writeQueue)

		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-writeQueue:
				if _, err := conn.WriteTo(msg.buf, msg.addr); err != nil {
					log.FromContext(ctx).Error(err, "failed write to udp", "addr", msg.addr.String())
				}
			}
		}
	}()

	sess := &sessions{m: make(map[string]*udpSession)}
	go sess.start(ctx)
	defer sess.stop()

	go func() {
		defer func() { _ = conn.Close() }()

		for {
			select {
			case <-ctx.Done():
				return
			default:
				if err = conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
					log.FromContext(ctx).Error(err, "failed to set read deadline on udp connection")
					return
				}

				buf := make([]byte, 32<<10)
				n, addr, rErr := conn.ReadFrom(buf)
				if rErr != nil {
					if errors.Is(rErr, os.ErrDeadlineExceeded) {
						continue
					}

					log.FromContext(ctx).Error(err, "failed to read udp packet", "addr", addr.String())
					return
				}

				us := sess.addIfAbsent(addr, writeQueue, dstAddr)
				select {
				case us.data <- buf[:n]:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return nil
}

// udpSession represents an active UDP forwarding session
type udpSession struct {
	src        net.Addr
	data       chan []byte
	cancel     context.CancelFunc
	lastActive time.Time
	writeQueue chan<- *writeMsg
}

// serve handles the bidirectional forwarding for a UDP session
func (s *udpSession) serve(target *net.UDPAddr) {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	conn, err := net.DialUDP("udp", nil, target)
	if err != nil {
		log.FromContext(ctx).Error(err, "failed to dial udp server")
		return
	}
	defer func() { _ = conn.Close() }()

	// Handle responses from the target back to the client
	go func() {
		for {
			buf := make([]byte, 32<<10)

			n, err := conn.Read(buf)
			if err != nil {
				log.FromContext(ctx).Error(err, "failed to read from udp")
				continue
			}

			select {
			case <-ctx.Done():
				return
			case s.writeQueue <- &writeMsg{addr: s.src, buf: buf[:n]}:
				s.lastActive = time.Now()
			}
		}
	}()

	// Handle incoming data from the client
	for {
		select {
		case <-ctx.Done():
			return
		case buf := <-s.data:
			if _, err = conn.Write(buf); err != nil {
				log.FromContext(ctx).Error(err, "failed to send udp packet", "target", target.String())
				continue
			}
			s.lastActive = time.Now()
		default:
		}
	}
}

// sessions provides thread-safe storage and management of UDP sessions
type sessions struct {
	sync.RWMutex
	m map[string]*udpSession
}

// addIfAbsent gets an existing session or creates a new one if not found.
func (s *sessions) addIfAbsent(src net.Addr, wq chan<- *writeMsg, target *net.UDPAddr) *udpSession {
	s.RLock()
	if us, found := s.m[src.String()]; found {
		s.RUnlock()
		return us
	}

	s.RUnlock()
	s.Lock()
	defer s.Unlock()

	us := &udpSession{
		src:        src,
		data:       make(chan []byte, 32),
		lastActive: time.Now(),
		writeQueue: wq,
	}

	go us.serve(target)
	s.m[src.String()] = us
	return us
}

// start initiates a background goroutine that periodically cleans up inactive sessions
func (s *sessions) start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(30 * time.Second):
			s.clean()
		}
	}
}

// stop terminates all active sessions.
func (s *sessions) stop() {
	s.Lock()
	defer s.Unlock()

	for _, sess := range s.m {
		if sess.cancel != nil {
			sess.cancel()
		}
	}
}

// clean removes sessions that have been inactive for more than 5 minutes
func (s *sessions) clean() {
	s.Lock()
	defer s.Unlock()

	now := time.Now()
	for k, sess := range s.m {
		if now.Sub(sess.lastActive) > 5*time.Minute {
			delete(s.m, k)
		}
	}
}
