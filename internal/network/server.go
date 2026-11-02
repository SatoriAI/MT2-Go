// Package network owns the TCP side: it accepts connections, gives each one a goroutine
// that cuts the byte stream into packets, and hands every packet to that connection's
// Session. It knows nothing about what the packets mean.
package network

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"sync"

	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// maxBuffered caps how much unparsed input one connection may hold. Every packet this
// server frames is far smaller, so a client that reaches it is not speaking the protocol.
const maxBuffered = 16 * 1024

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

// Serve accepts connections on ln until ctx ends, then closes every open connection and
// waits for their goroutines to finish.
func Serve(ctx context.Context, ln net.Listener, newSession NewSession, log *slog.Logger) error {
	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		conns = map[*Conn]struct{}{}
	)
	go func() {
		<-ctx.Done()
		_ = ln.Close()
		mu.Lock()
		for c := range conns {
			c.Close()
		}
		mu.Unlock()
	}()
	defer wg.Wait()

	for {
		raw, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		c := newConn(raw, log)
		mu.Lock()
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

// Conn is one client connection.
type Conn struct {
	raw  net.Conn
	log  *slog.Logger
	once sync.Once
}

func newConn(raw net.Conn, log *slog.Logger) *Conn {
	return &Conn{raw: raw, log: log.With("remote", raw.RemoteAddr().String())}
}

// Log is this connection's logger, tagged with the remote address.
func (c *Conn) Log() *slog.Logger { return c.log }

// Send writes packets in order. Only the connection's own goroutine sends, so writes
// never interleave.
func (c *Conn) Send(packets ...[]byte) error {
	for _, p := range packets {
		if _, err := c.raw.Write(p); err != nil {
			return err
		}
	}
	return nil
}

// Close ends the connection; the read loop then returns and the Session is closed.
func (c *Conn) Close() {
	c.once.Do(func() { _ = c.raw.Close() })
}

func (c *Conn) serve(ctx context.Context, s Session) {
	defer s.Close()
	defer c.Close()

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
			c.log.Warn("closing connection", "err", err)
			return
		}
		if len(buf) > maxBuffered {
			c.log.Warn("closing connection", "err", "too much unparsed input")
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
