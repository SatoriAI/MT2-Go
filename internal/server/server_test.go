package server

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/config"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/store"
)

// These tests drive both ports over real sockets with the bytes the game client sends,
// against a real database, so they prove the flow rather than the parts.

const sequenceByte = 0xAF // the first entry of the client's sequence table

const password = "secret" // every test account's

type harness struct {
	t        *testing.T
	accounts store.Accounts
	auth     string
	game     string
}

func start(t *testing.T) *harness {
	t.Helper()
	if testing.Short() {
		t.Skip("needs PostgreSQL; run without -short")
	}
	url := config.TestDatabaseURL()
	if url == "" {
		t.Fatal("TEST_DATABASE_URL is not set: copy .env.example to .env and run `make db`, or pass -short")
	}
	ctx, cancel := context.WithCancel(context.Background())
	pool, err := store.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `TRUNCATE accounts, characters RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	authLn := listen(t)
	gameLn := listen(t)
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

func listen(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	return ln
}

func (h *harness) createAccount(name string) int64 {
	h.t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		h.t.Fatal(err)
	}
	id, err := h.accounts.CreateAccount(context.Background(), name, hash)
	if err != nil {
		h.t.Fatal(err)
	}
	return id
}

// exec runs SQL the server has no query for yet, such as creating a character.
func (h *harness) exec(sql string, args ...any) {
	h.t.Helper()
	if _, err := h.accounts.Pool.Exec(context.Background(), sql, args...); err != nil {
		h.t.Fatal(err)
	}
}

// waitForEmpire waits until the account admin, the one every kingdom test uses, has want.
func (h *harness) waitForEmpire(want account.Empire) {
	h.t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if acc, err := h.accounts.AccountByName(context.Background(), "admin"); err == nil && acc.Empire == want {
			return
		}
	}
	h.t.Fatalf("kingdom %q was never stored for admin", want)
}

// client plays the game client's side of one connection.
type client struct {
	t    *testing.T
	conn net.Conn
}

func (h *harness) dial(addr string) *client {
	h.t.Helper()
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
	c.t.Helper()
	c.send([]byte{byte(protocol.ClientLogin)}, field(name, 31), field(password, 17), make([]byte, 16), []byte{sequenceByte})
}

func (c *client) loginByKey(name string, key uint32) {
	c.t.Helper()
	c.send([]byte{byte(protocol.ClientLoginByKey)}, field(name, 31), binary.LittleEndian.AppendUint32(nil, key),
		make([]byte, 16), []byte{sequenceByte})
}

// chooseEmpire sends the kingdom by the client's number: 1 Shinsoo, 2 Chunjo, 3 Jinno.
func (c *client) chooseEmpire(number, sequence byte) {
	c.t.Helper()
	c.send([]byte{byte(protocol.ClientEmpire), number, sequence})
}

// createCharacter sends what the create screen sends. The real client's four stat points
// are always 0; these are not, so every test shows the server ignores them.
func (c *client) createCharacter(slot byte, name string, race uint16, shape byte) {
	c.t.Helper()
	c.send([]byte{byte(protocol.ClientCreateCharacter), slot}, field(name, 25),
		binary.LittleEndian.AppendUint16(nil, race), []byte{shape, 9, 9, 9, 9, sequenceByte})
}

func (c *client) selectCharacter(slot byte) {
	c.t.Helper()
	c.send([]byte{byte(protocol.ClientSelectCharacter), slot, sequenceByte})
}

func (c *client) enterGame() {
	c.t.Helper()
	c.send([]byte{byte(protocol.ClientEnterGame), sequenceByte})
}

// expectOpen proves the server still serves the connection: it answers the channel check
// only after handling every packet sent before it.
func (c *client) expectOpen() {
	c.t.Helper()
	port := netip.MustParseAddrPort(c.conn.RemoteAddr().String()).Port()
	c.send([]byte{byte(protocol.ClientStateChecker)})
	c.expect(protocol.EncodeChannelStatus([]protocol.ChannelStatus{{Port: port, Status: protocol.ChannelNormal}}))
}

// expectClosed waits for the server to end the connection.
func (c *client) expectClosed() {
	c.t.Helper()
	if n, err := c.conn.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
		c.t.Fatalf("expected the server to close the connection, read %d bytes, err %v", n, err)
	}
}

// enterSelection logs in with a key on a new game connection, up to the select phase
// and anything sent before it.
func (h *harness) enterSelection(name string, key uint32) *client {
	h.t.Helper()
	game := h.dial(h.game)
	game.handshake(protocol.PhaseLogin)
	game.loginByKey(name, key)
	return game
}

// authenticate logs in on the auth port and returns the key for the game port.
func (h *harness) authenticate(name string) uint32 {
	h.t.Helper()
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
	h.createAccount("admin")
	key := h.authenticate("admin")

	game := h.enterSelection("admin", key)

	game.expect(protocol.EncodePhase(protocol.PhaseSelect))
}

func TestTheNameLogsInWhateverItsCase(t *testing.T) {
	h := start(t)
	h.createAccount("admin")
	key := h.authenticate("ADMIN")

	game := h.enterSelection("ADMIN", key)

	game.expect(protocol.EncodePhase(protocol.PhaseSelect))
}

func TestTheChosenKingdomIsStoredAndTheNextLoginShowsTheCharacterList(t *testing.T) {
	h := start(t)
	id := h.createAccount("admin")
	key := h.authenticate("admin")
	first := h.enterSelection("admin", key)
	first.expect(protocol.EncodePhase(protocol.PhaseSelect))

	first.chooseEmpire(2, 0xCA)
	h.waitForEmpire(account.EmpireChunjo) // the server sends nothing back to wait on
	_ = first.conn.Close()
	h.exec(`INSERT INTO characters (account_id, slot, name, class, sex, level, strength, vitality, dexterity,
		intelligence, hair, x, y, skill_group) VALUES ($1, 2, 'Hendra', 'sura', 'female', 21, 6, 4, 3, 2, 3, 469300, 964200,
		'black_magic')`, id)

	second := h.enterSelection("admin", key) // the client reuses its key for a second game connection
	second.expect(protocol.EncodeEmpire(2))
	var slots [protocol.CharacterSlots]protocol.CharacterSummary
	slots[2] = protocol.CharacterSummary{ID: 1, Name: "Hendra", Race: 6, Level: 21, ST: 6, HT: 4, DX: 3, IQ: 2,
		HairPart: 3, X: 469300, Y: 964200, SkillGroup: 2}
	second.expect(protocol.EncodeCharacterList(slots, netip.MustParseAddrPort(h.game)))
	second.expect(protocol.EncodePhase(protocol.PhaseSelect))
}

func TestTheDatabaseRefusesASkillGroupOfAnotherClass(t *testing.T) {
	h := start(t)
	id := h.createAccount("admin")

	_, err := h.accounts.Pool.Exec(context.Background(), `INSERT INTO characters
		(account_id, slot, name, class, sex, strength, vitality, dexterity, intelligence, x, y, skill_group)
		VALUES ($1, 0, 'Hendra', 'warrior', 'male', 6, 4, 3, 2, 0, 0, 'healing')`, id)

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" { // check_violation
		t.Fatalf("a warrior with the healing path was stored: %v", err)
	}
}

func TestADoubleClickedKingdomKeepsTheConnection(t *testing.T) {
	h := start(t)
	h.createAccount("admin")
	game := h.enterSelection("admin", h.authenticate("admin"))
	game.expect(protocol.EncodePhase(protocol.PhaseSelect))

	game.chooseEmpire(3, 0xCA)
	game.chooseEmpire(3, 0x8A)

	h.waitForEmpire(account.EmpireJinno)
	game.expectOpen()
}

// Before the first character is created, the client shows the kingdom screen again,
// starting on a random kingdom, and sends whichever the player picks there.
func TestANewAccountMayChangeItsKingdomBeforeItsFirstCharacter(t *testing.T) {
	h := start(t)
	h.createAccount("admin")
	game := h.enterSelection("admin", h.authenticate("admin"))
	game.expect(protocol.EncodePhase(protocol.PhaseSelect))
	game.chooseEmpire(1, 0xCA)
	h.waitForEmpire(account.EmpireShinsoo)

	game.chooseEmpire(3, 0x8A)

	h.waitForEmpire(account.EmpireJinno)
	game.expectOpen()
}

func TestAKingdomCannotChangeOnceTheAccountHasACharacter(t *testing.T) {
	h := start(t)
	id := h.createAccount("admin")
	h.exec(`UPDATE accounts SET empire = 'shinsoo' WHERE id = $1`, id)
	h.exec(`INSERT INTO characters (account_id, slot, name, class, sex, strength, vitality, dexterity, intelligence, x, y)
		VALUES ($1, 0, 'Hendra', 'sura', 'female', 6, 4, 3, 2, 469300, 964200)`, id)
	game := h.enterSelection("admin", h.authenticate("admin"))
	game.expect(protocol.EncodeEmpire(1))
	game.read(329) // the character list, whose bytes another test checks
	game.expect(protocol.EncodePhase(protocol.PhaseSelect))

	game.chooseEmpire(3, 0xCA)

	game.expectClosed()
	h.waitForEmpire(account.EmpireShinsoo)
}

func TestASecondLoginIsRefusedWhileTheAccountIsInTheGame(t *testing.T) {
	h := start(t)
	h.createAccount("admin")
	game := h.enterSelection("admin", h.authenticate("admin"))
	game.expect(protocol.EncodePhase(protocol.PhaseSelect))
	auth := h.dial(h.auth)
	auth.handshake(protocol.PhaseAuth)

	auth.login("admin", password)

	auth.expect(protocol.EncodeLoginFailure(protocol.LoginAlreadyOnline))
}

func TestAKeyIsRefusedForAnotherName(t *testing.T) {
	h := start(t)
	h.createAccount("admin")
	h.createAccount("other")
	key := h.authenticate("admin")

	game := h.enterSelection("other", key)

	game.expectClosed()
}

func TestAPacketOutOfItsPhaseEndsTheConnection(t *testing.T) {
	h := start(t)
	h.createAccount("admin")
	game := h.dial(h.game)
	game.handshake(protocol.PhaseLogin)

	game.chooseEmpire(1, sequenceByte) // before logging in

	game.expectClosed()
}

func TestAWrongPasswordIsRefusedWithTheStatusTheClientShows(t *testing.T) {
	h := start(t)
	h.createAccount("admin")
	auth := h.dial(h.auth)
	auth.handshake(protocol.PhaseAuth)

	auth.login("admin", "wrong")

	auth.expect(protocol.EncodeLoginFailure(protocol.LoginWrongPassword))
}

func TestAnUnknownNameGetsTheSameAnswerAsAWrongPassword(t *testing.T) {
	h := start(t)
	auth := h.dial(h.auth)
	auth.handshake(protocol.PhaseAuth)

	auth.login("nobody", password)

	auth.expect(protocol.EncodeLoginFailure(protocol.LoginWrongPassword))
}

func TestTheConnectionClosesAfterThreeWrongPasswords(t *testing.T) {
	h := start(t)
	h.createAccount("admin")
	auth := h.dial(h.auth)
	auth.handshake(protocol.PhaseAuth)

	for range 3 {
		auth.login("admin", "wrong")
		auth.expect(protocol.EncodeLoginFailure(protocol.LoginWrongPassword))
	}

	auth.expectClosed()
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

// inKingdom logs admin in on a new game connection and chooses the kingdom by its number.
func (h *harness) inKingdom(number byte) *client {
	h.t.Helper()
	game := h.enterSelection("admin", h.authenticate("admin"))
	game.expect(protocol.EncodePhase(protocol.PhaseSelect))
	game.chooseEmpire(number, sequenceByte)
	return game
}

func TestANewCharacterIsCreatedAndListedOnTheNextLogin(t *testing.T) {
	h := start(t)
	h.createAccount("admin")
	game := h.inKingdom(3)

	game.createCharacter(0, "Hendra", 6, 1) // a female sura in her second look

	hendra := protocol.CharacterSummary{ID: 1, Name: "Hendra", Race: 6, Level: 1,
		ST: 5, HT: 3, DX: 3, IQ: 5, X: 963400, Y: 278200, // the sura's start, at Jinno's village
		BodyPart: 1} // her look, shown as the body part
	address := netip.MustParseAddrPort(h.game)
	game.expect(protocol.EncodeCharacterCreated(0, hendra, address))
	_ = game.conn.Close()

	next := h.enterSelection("admin", h.authenticate("admin"))
	next.expect(protocol.EncodeEmpire(3))
	var slots [protocol.CharacterSlots]protocol.CharacterSummary
	slots[0] = hendra
	next.expect(protocol.EncodeCharacterList(slots, address))
	var shape int
	if err := h.accounts.Pool.QueryRow(context.Background(), `SELECT shape FROM characters`).Scan(&shape); err != nil || shape != 1 {
		t.Errorf("shape %d stored, err %v", shape, err)
	}
}

func TestANameTakenInAnyCaseIsRefusedAsTaken(t *testing.T) {
	h := start(t)
	h.createAccount("admin")
	game := h.inKingdom(1)
	game.createCharacter(0, "Hendra", 0, 0)
	game.read(65)

	game.createCharacter(1, "HENDRA", 0, 0)

	game.expect(protocol.EncodeCreateFailure(protocol.CreateNameTaken))
	game.expectOpen()
}

func TestACharacterTheRulesRefuseGetsTheGeneralFailure(t *testing.T) {
	cases := map[string]struct {
		slot  byte
		name  string
		race  uint16
		shape byte
	}{
		"name too short":         {0, "H", 0, 0},
		"name too long":          {0, "Hendrakowalsk", 0, 0},
		"name filling the field": {0, "Aaaaaaaaaaaaaaaaaaaaaaaaa", 0, 0},
		"name not ASCII":         {0, "Hendr\xb1", 0, 0},
		"name with a symbol":     {0, "Hendra!", 0, 0},
		"unknown race":           {0, "Hendra", 8, 0},
		"no such slot":           {4, "Hendra", 0, 0},
		"no such look":           {0, "Hendra", 0, 2},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			h := start(t)
			h.createAccount("admin")
			game := h.inKingdom(1)

			game.createCharacter(c.slot, c.name, c.race, c.shape)

			game.expect(protocol.EncodeCreateFailure(protocol.CreateRefused))
			game.expectOpen()
		})
	}
}

func TestATakenSlotGetsTheGeneralFailure(t *testing.T) {
	h := start(t)
	h.createAccount("admin")
	game := h.inKingdom(1)
	game.createCharacter(0, "Hendra", 0, 0)
	game.read(65)

	game.createCharacter(0, "Kowal", 0, 0)

	game.expect(protocol.EncodeCreateFailure(protocol.CreateRefused))
	game.expectOpen()
}

func TestNamesAtTheLengthLimitsAreAccepted(t *testing.T) {
	h := start(t)
	h.createAccount("admin")
	game := h.inKingdom(1)

	game.createCharacter(0, "Hk", 0, 0)
	game.expect([]byte{byte(protocol.ServerCharacterCreated), 0})
	game.read(63)
	game.createCharacter(1, "Hendrakowals", 0, 0)
	game.expect([]byte{byte(protocol.ServerCharacterCreated), 1})
}

// Two connections of one account: the other changes the kingdom while this one, which
// still knows the old kingdom, creates a character.
func TestACreationAfterAnotherConnectionChangedTheKingdomEndsTheConnection(t *testing.T) {
	h := start(t)
	h.createAccount("admin")
	key := h.authenticate("admin")
	this := h.enterSelection("admin", key)
	this.expect(protocol.EncodePhase(protocol.PhaseSelect))
	this.chooseEmpire(1, sequenceByte)
	h.waitForEmpire(account.EmpireShinsoo)
	other := h.enterSelection("admin", key)
	other.expect(protocol.EncodeEmpire(1))
	other.read(329)
	other.expect(protocol.EncodePhase(protocol.PhaseSelect))
	other.chooseEmpire(3, sequenceByte)
	h.waitForEmpire(account.EmpireJinno)

	this.createCharacter(0, "Hendra", 0, 0)

	this.expectClosed()
	var characters int
	if err := h.accounts.Pool.QueryRow(context.Background(), `SELECT count(*) FROM characters`).Scan(&characters); err != nil || characters != 0 {
		t.Errorf("%d characters stored, err %v", characters, err)
	}
}

func TestOnceACharacterExistsTheKingdomIsFixed(t *testing.T) {
	h := start(t)
	h.createAccount("admin")
	game := h.inKingdom(1)
	game.createCharacter(0, "Hendra", 0, 0)
	game.read(65)

	game.chooseEmpire(3, sequenceByte)

	game.expectClosed()
	h.waitForEmpire(account.EmpireShinsoo)
}

func TestACharacterWithoutAKingdomEndsTheConnection(t *testing.T) {
	h := start(t)
	h.createAccount("admin")
	game := h.enterSelection("admin", h.authenticate("admin"))
	game.expect(protocol.EncodePhase(protocol.PhaseSelect))

	game.createCharacter(0, "Hendra", 0, 0)

	game.expectClosed()
}

// withHendra is admin in Jinno with Hendra, a female sura in her second look, in slot 0.
func (h *harness) withHendra() *client {
	h.t.Helper()
	h.createAccount("admin")
	game := h.inKingdom(3)
	game.createCharacter(0, "Hendra", 6, 1)
	game.read(65)
	return game
}

// expectLoading reads what Start makes the server send, for Hendra as the first character
// in the world.
func (c *client) expectLoading() {
	c.t.Helper()
	c.expect(protocol.EncodePhase(protocol.PhaseLoading))
	c.expect(protocol.EncodeMainCharacter(protocol.MainCharacter{VID: 1, Race: 6, Name: "Hendra",
		X: 963400, Y: 278200, Empire: 3}))
	c.expect(protocol.EncodePoints(protocol.CharacterPoints{Level: 1, NextExp: 300,
		HP: 950, MaxHP: 950, SP: 850, MaxSP: 850, Stamina: 1000, MaxStamina: 1000, // a level-1 sura's
		ST: 5, HT: 3, DX: 3, IQ: 5, AttackSpeed: 150, MoveSpeed: 150}))
	c.expect(protocol.EncodeSkillLevels())
}

func TestStartLoadsTheWorldAndEnteringPutsThePlayerInIt(t *testing.T) {
	h := start(t)
	game := h.withHendra()

	game.selectCharacter(0)
	game.expectLoading()
	game.enterGame()

	game.expect(protocol.EncodePhase(protocol.PhaseGame))
	if clock := game.read(5); clock[0] != byte(protocol.ServerTime) {
		t.Fatalf("expected the time, got %x", clock)
	}
	game.expect(protocol.EncodeChannel(1))
	game.expect(protocol.EncodeCharacterAdd(protocol.CharacterAdd{VID: 1, X: 963400, Y: 278200,
		Type: protocol.EntityPlayer, Race: 6, MoveSpeed: 150, AttackSpeed: 150}))
	game.expect(protocol.EncodeCharacterInfo(protocol.CharacterInfo{VID: 1, Name: "Hendra", BodyPart: 1,
		Empire: 3, Level: 1}))
	game.expectOpen()
}

// The loading screen may ask for the character the select screen already asked for.
func TestTheSameCharacterSelectedAgainWhileLoadingIsIgnored(t *testing.T) {
	h := start(t)
	game := h.withHendra()
	game.selectCharacter(0)
	game.expectLoading()

	game.selectCharacter(0)
	game.enterGame()

	game.expect(protocol.EncodePhase(protocol.PhaseGame))
}

func TestAnotherCharacterSelectedWhileLoadingEndsTheConnection(t *testing.T) {
	h := start(t)
	game := h.withHendra()
	game.selectCharacter(0)
	game.expectLoading()

	game.selectCharacter(1)

	game.expectClosed()
}

func TestStartOnAnEmptySlotEndsTheConnection(t *testing.T) {
	h := start(t)
	game := h.withHendra()

	game.selectCharacter(1)

	game.expectClosed()
}

func TestEnteringBeforeStartEndsTheConnection(t *testing.T) {
	h := start(t)
	game := h.withHendra()

	game.enterGame()

	game.expectClosed()
}
