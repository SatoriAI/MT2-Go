package server

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/SatoriAI/MT2-Go/internal/config"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/store"
)

// These tests drive both ports over real sockets with the bytes the game client sends,
// against a real database, so they prove the flow rather than the parts.

const sequenceByte = 0xAF // the first entry of the client's sequence table

type harness struct {
	t        *testing.T
	accounts store.Accounts
	auth     string
	game     string
}

func start(t *testing.T) *harness {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set; run `make db` and use .env")
	}
	ctx, cancel := context.WithCancel(context.Background())
	pool, err := store.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `TRUNCATE accounts, characters RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	authLn, _ := net.Listen("tcp", "127.0.0.1:0")
	gameLn, _ := net.Listen("tcp", "127.0.0.1:0")
	cfg := config.Config{GamePublic: netip.MustParseAddrPort(gameLn.Addr().String())}
	accounts := store.Accounts{Pool: pool}
	done := make(chan error, 1)
	go func() {
		done <- Serve(ctx, Ports{Auth: authLn, Game: gameLn}, accounts, cfg, slog.New(slog.DiscardHandler))
	}()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
		pool.Close()
	})
	return &harness{t: t, accounts: accounts, auth: authLn.Addr().String(), game: gameLn.Addr().String()}
}

func (h *harness) createAccount(name, password string) {
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if _, err := h.accounts.CreateAccount(context.Background(), name, hash); err != nil {
		h.t.Fatal(err)
	}
}

func (h *harness) waitForEmpire(name string, want protocol.Empire) {
	h.t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if acc, err := h.accounts.AccountByName(context.Background(), name); err == nil && acc.Empire == want {
			return
		}
	}
	h.t.Fatalf("kingdom %d was never stored for %s", want, name)
}

// client plays the game client's side of one connection.
type client struct {
	t    *testing.T
	conn net.Conn
}

func (h *harness) dial(addr string) *client {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		h.t.Fatal(err)
	}
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	h.t.Cleanup(func() { _ = conn.Close() })
	return &client{t: h.t, conn: conn}
}

func (c *client) read(n int) []byte {
	c.t.Helper()
	b := make([]byte, n)
	if _, err := io.ReadFull(c.conn, b); err != nil {
		c.t.Fatalf("reading %d bytes: %v", n, err)
	}
	return b
}

func (c *client) expect(want []byte) {
	c.t.Helper()
	if got := c.read(len(want)); !bytes.Equal(got, want) {
		c.t.Fatalf("got %x, want %x", got, want)
	}
}

func (c *client) send(parts ...[]byte) {
	c.t.Helper()
	if _, err := c.conn.Write(bytes.Join(parts, nil)); err != nil {
		c.t.Fatal(err)
	}
}

// handshake answers the greeting the way the client does, then expects the next phase.
func (c *client) handshake(next protocol.Phase) {
	c.t.Helper()
	c.expect(protocol.EncodePhase(protocol.PhaseHandshake))
	greeting := c.read(13)
	c.send(greeting[:9], []byte{0, 0, 0, 0}) // same id and time, delta 0
	c.expect(protocol.EncodePhase(next))
}

func field(s string, size int) []byte {
	b := make([]byte, size)
	copy(b, s)
	return b
}

func (c *client) login(name, password string) {
	c.send([]byte{byte(protocol.ClientLogin)}, field(name, 31), field(password, 17), make([]byte, 16), []byte{sequenceByte})
}

func (c *client) loginByKey(name string, key uint32) {
	c.send([]byte{byte(protocol.ClientLoginByKey)}, field(name, 31), binary.LittleEndian.AppendUint32(nil, key),
		make([]byte, 16), []byte{sequenceByte})
}

// authenticate logs in on the auth port and returns the key for the game port.
func (h *harness) authenticate(name, password string) uint32 {
	auth := h.dial(h.auth)
	auth.handshake(protocol.PhaseAuth)
	auth.login(name, password)
	reply := auth.read(6)
	if reply[0] != byte(protocol.ServerAuthSuccess) || reply[5] != 1 {
		h.t.Fatalf("login refused: %x", reply)
	}
	return binary.LittleEndian.Uint32(reply[1:5])
}

func TestANewAccountLogsInAndIsAskedForAKingdom(t *testing.T) {
	h := start(t)
	h.createAccount("admin", "secret")
	key := h.authenticate("admin", "secret")

	game := h.dial(h.game)
	game.handshake(protocol.PhaseLogin)
	game.loginByKey("admin", key)

	game.expect(protocol.EncodePhase(protocol.PhaseSelect))
}

func TestTheChosenKingdomIsStoredAndTheNextLoginShowsTheCharacterList(t *testing.T) {
	h := start(t)
	h.createAccount("admin", "secret")
	key := h.authenticate("admin", "secret")
	first := h.dial(h.game)
	first.handshake(protocol.PhaseLogin)
	first.loginByKey("admin", key)
	first.expect(protocol.EncodePhase(protocol.PhaseSelect))

	first.send([]byte{byte(protocol.ClientEmpire), byte(protocol.EmpireYellow), 0xCA})
	h.waitForEmpire("admin", protocol.EmpireYellow) // the server sends nothing back to wait on
	_ = first.conn.Close()

	second := h.dial(h.game) // the client reuses its key for a second game connection
	second.handshake(protocol.PhaseLogin)
	second.loginByKey("admin", key)
	second.expect(protocol.EncodeEmpire(protocol.EmpireYellow))
	if list := second.read(329); list[0] != byte(protocol.ServerCharacterList) {
		t.Fatalf("expected the character list, got %x", list[:1])
	}
	second.expect(protocol.EncodePhase(protocol.PhaseSelect))
}

func TestAWrongPasswordIsRefusedWithTheStatusTheClientShows(t *testing.T) {
	h := start(t)
	h.createAccount("admin", "secret")
	auth := h.dial(h.auth)
	auth.handshake(protocol.PhaseAuth)

	auth.login("admin", "wrong")

	auth.expect(protocol.EncodeLoginFailure(protocol.LoginWrongPassword))
}

func TestTheLoginScreenSeesTheChannelAsOpen(t *testing.T) {
	h := start(t)
	game := h.dial(h.game)
	game.expect(protocol.EncodePhase(protocol.PhaseHandshake))
	game.read(13)
	port := netip.MustParseAddrPort(h.game).Port()

	game.send([]byte{byte(protocol.ClientStateChecker)})

	game.expect(protocol.EncodeChannelStatus([]protocol.ChannelStatus{{Port: port, Status: protocol.ChannelNormal}}))
}
