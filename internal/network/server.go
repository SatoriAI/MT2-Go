// Package network owns the TCP side: it accepts connections, gives each one a goroutine
// that cuts the byte stream into packets, and hands every packet to that connection's
// Session. It knows nothing about what the packets mean.
package network

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// queueLength is how many sends may wait for a client. A client that falls this far
// behind is not reading, and is dropped.
const queueLength = 256

// errFallenBehind is what Send reports when the client is no longer reading.
var errFallenBehind = errors.New("network: client fell behind")

// maxAcceptDelay caps the pause between retries when accepting fails, for example while
// the process is out of file descriptors.
const maxAcceptDelay = time.Second

// Timeouts bound how long a client can hold a connection without earning it.
type Timeouts struct {
	// Login is counted from the accept: a connection still open after it is closed,
	// however busy it was, unless its session called KeepOpen.
	Login time.Duration
	// Write bounds each write to the client, so one that stops reading cannot hold the
	// connection's writer for ever.
	Write time.Duration
}

// ErrHangUp is what a session returns to end its connection on purpose, without the
// warning an error would log.
var ErrHangUp = errors.New("network: session hung up")

// Session is the per-connection logic of one port.
type Session interface {
	// Open runs once, before the first read; servers speak first in this protocol.
	Open() error
	// Handle runs for each complete packet. An error ends the connection.
	Handle(context.Context, protocol.Frame) error
	// Close runs once, after the connection has ended for any reason.
	Close()
}

// NewSession builds the Session for a freshly accepted connection.
type NewSession func(*Conn) Session

// Serve accepts connections on ln until ctx ends or accepting fails for good, then closes
// every open connection and waits for their goroutines to finish.
func Serve(ctx context.Context, ln net.Listener, newSession NewSession, limits Timeouts, log *slog.Logger) error {
	ctx, cancel := context.WithCancel(ctx)
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		stopped bool
		conns   = map[*Conn]struct{}{}
	)
	go func() {
		<-ctx.Done()
		_ = ln.Close()
		mu.Lock()
		stopped = true
		for c := range conns {
			c.Close()
		}
		mu.Unlock()
	}()
	// Deferred calls run last first: cancelling closes the connections, then they are awaited.
	defer wg.Wait()
	defer cancel()

	var delay time.Duration
	for {
		raw, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			if errors.Is(err, net.ErrClosed) {
				return err
			}
			// Running out of file descriptors is not fatal: connections close and free them.
			delay = min(max(2*delay, 5*time.Millisecond), maxAcceptDelay)
			log.Warn("accept failed, retrying", "err", err, "in", delay)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
			}
			continue
		}
		delay = 0
		c := newConn(raw, limits, log)
		mu.Lock()
		if stopped {
			mu.Unlock()
			c.Close()
			continue
		}
		conns[c] = struct{}{}
		mu.Unlock()
		wg.Go(func() {
			defer func() {
				mu.Lock()
				delete(conns, c)
				mu.Unlock()
			}()
			c.serve(ctx, newSession(c))
		})
	}
}

// Conn is one client connection. Any goroutine may send on it; its own reads the packets.
type Conn struct {
	raw   net.Conn
	log   *slog.Logger
	once  sync.Once
	write time.Duration

	queue   chan []byte   // sends waiting for the writer, each written whole
	closed  chan struct{} // closed by Close: the writer stops at once
	ending  chan struct{} // closed as the session ends: the writer sends what waits, then stops
	stopped chan struct{} // closed by the writer as it stops
}

func newConn(raw net.Conn, limits Timeouts, log *slog.Logger) *Conn {
	// Setting a deadline fails only on a closed connection, which the first Read reports.
	if limits.Login > 0 {
		_ = raw.SetReadDeadline(time.Now().Add(limits.Login))
	}
	return &Conn{
		raw:     raw,
		write:   limits.Write,
		log:     log.With("remote", raw.RemoteAddr().String()),
		queue:   make(chan []byte, queueLength),
		closed:  make(chan struct{}),
		ending:  make(chan struct{}),
		stopped: make(chan struct{}),
	}
}

// Log is this connection's logger, tagged with the remote address.
func (c *Conn) Log() *slog.Logger { return c.log }

// Send queues packets to be written together, after everything sent before them. It never
// waits for the client: one too far behind is closed instead.
func (c *Conn) Send(packets ...[]byte) error {
	select {
	case <-c.closed:
		return net.ErrClosed
	default:
	}
	select {
	case c.queue <- bytes.Join(packets, nil):
		return nil
	default:
		c.Close()
		return errFallenBehind
	}
}

// KeepOpen lifts the login deadline. A session calls it once the client has proved who it
// is; the player may then stay as long as they like.
func (c *Conn) KeepOpen() { _ = c.raw.SetReadDeadline(time.Time{}) }

// Close ends the connection at once; the read loop then returns and the Session is closed.
func (c *Conn) Close() {
	c.once.Do(func() {
		close(c.closed)
		_ = c.raw.Close()
	})
}

// writeQueued writes each send in turn. Once the session has ended it writes what is still
// waiting, so a last packet like Logout's reaches the client, and stops.
func (c *Conn) writeQueued() {
	defer close(c.stopped)
	for {
		select {
		case p := <-c.queue:
			if !c.writeOne(p) {
				return
			}
		case <-c.closed:
			return
		case <-c.ending:
			for {
				select {
				case p := <-c.queue:
					if !c.writeOne(p) {
						return
					}
				default:
					return
				}
			}
		}
	}
}

// writeOne writes p within the write timeout, closing the connection if it cannot.
func (c *Conn) writeOne(p []byte) bool {
	if c.write > 0 {
		_ = c.raw.SetWriteDeadline(time.Now().Add(c.write)) // fails only once closed, as Write then does
	}
	if _, err := c.raw.Write(p); err != nil {
		c.log.Debug("write failed", "err", err)
		c.Close()
		return false
	}
	return true
}

func (c *Conn) serve(ctx context.Context, s Session) {
	go c.writeQueued()
	// On the way out the session closes first, so its player leaves the world even when
	// what is still queued for them takes the whole write timeout to fail.
	defer c.Close()
	defer func() {
		close(c.ending)
		<-c.stopped
	}()
	defer s.Close()

	if err := s.Open(); err != nil {
		c.log.Warn("open failed", "err", err)
		return
	}
	var buf []byte
	chunk := make([]byte, 4096)
	for {
		n, err := c.raw.Read(chunk)
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				c.log.Debug("connection ended", "err", err)
			}
			return
		}
		buf = append(buf, chunk[:n]...)
		if buf, err = c.dispatch(ctx, buf, s); err != nil {
			// Shutting down cuts sessions short; that is not the client's doing.
			if !errors.Is(err, ErrHangUp) && ctx.Err() == nil {
				c.log.Warn("closing connection", "err", err)
			}
			return
		}
	}
}

// dispatch hands every complete packet in buf to the session and returns what is left:
// the start of a packet whose remaining bytes have not arrived yet.
func (c *Conn) dispatch(ctx context.Context, buf []byte, s Session) ([]byte, error) {
	for {
		frame, used, err := protocol.NextFrame(buf)
		if errors.Is(err, protocol.ErrIncomplete) {
			return buf, nil
		}
		if err != nil {
			return nil, err
		}
		if err := s.Handle(ctx, frame); err != nil {
			return nil, err
		}
		buf = buf[used:]
	}
}
