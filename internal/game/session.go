// Package game is the game port: the login screen's channel check, the login by key, the
// kingdom choice, creating and listing characters, and the world: moving, talking, and
// seeing the others on the same map.
package game

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/handshake"
	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/network"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/world"
)

// Accounts is what the game port needs from storage.
type Accounts interface {
	AccountByID(ctx context.Context, id int64) (account.Account, error)
	// ChooseEmpire sets the account's kingdom while it has no character, and reports
	// whether the account now has that kingdom.
	ChooseEmpire(ctx context.Context, id int64, empire account.Empire) (bool, error)
	Characters(ctx context.Context, accountID int64) ([]account.Character, error)
	// CreateCharacter stores c and returns its id, while the account's kingdom is still
	// empire: account.ErrKingdomChanged, ErrNameTaken or ErrSlotTaken otherwise.
	CreateCharacter(ctx context.Context, accountID int64, empire account.Empire, c account.Character) (uint32, error)
	// Character reads the account's character in slot, or returns account.ErrNotFound.
	Character(ctx context.Context, accountID int64, slot int) (account.Character, error)
	SavePosition(ctx context.Context, characterID uint32, x, y int32) error
	SaveProgress(ctx context.Context, c account.Character) error
	SaveBlocks(ctx context.Context, characterID uint32, blocks account.Blocks) error
}

// Whispers is where the game port keeps whispers, by character id.
type Whispers interface {
	Save(ctx context.Context, sender, recipient uint32, text string, sentAt time.Time) error
}

// Service is what every game connection shares.
type Service struct {
	store    Accounts
	whispers Whispers
	items    Items
	taskbar  Taskbar
	keys     *account.Keys
	clock    handshake.Clock
	// public is the address the client is told to connect to for this game server. It
	// has to be reachable from the client, so it is never 0.0.0.0.
	public netip.AddrPort
	// lastVID numbers the characters in the world, so each has its own id.
	lastVID atomic.Uint32

	mu     sync.Mutex
	claims map[uint32]*claim // the characters in play, by id

	presence presence

	// The world loop's own: only Run's goroutine touches them.
	spawns    []world.Spawn
	rand      *rand.Rand
	monsters  []*monster
	byPresent map[*present]*monster // the same monsters, by what the world record holds of them
	fighting  []*monster            // the monsters fighting a player, which the loop steps
	healing   []healing             // the hurt monsters, which the loop heals
	// nextPlayerRegen is when the loop next heals the players.
	nextPlayerRegen time.Time
	// now is the time, which tests set.
	now func() time.Time
	// The drops on the ground, in the order they fell, to free and to clear away.
	freeing, vanishing []timed
}

// NewService makes the game port's shared state; Run puts spawns' mobs in the world.
func NewService(store Accounts, whispers Whispers, items Items, taskbar Taskbar, keys *account.Keys, clock handshake.Clock, public netip.AddrPort,
	spawns []world.Spawn) *Service {
	return &Service{store: store, whispers: whispers, items: items, taskbar: taskbar, keys: keys, clock: clock, public: public, claims: map[uint32]*claim{},
		presence: presence{all: map[uint32]*present{}, byName: map[string]*present{}, ground: map[uint32]*groundItem{}}, spawns: spawns, rand: newRand(), now: time.Now,
		byPresent: map[*present]*monster{}}
}

func (s *Service) NewSession(conn *network.Conn) network.Session {
	return &session{svc: s, conn: conn, sync: handshake.NewSync(s.clock), phase: protocol.PhaseHandshake, rand: newRand()}
}

type session struct {
	svc       *Service
	conn      *network.Conn
	sync      *handshake.Sync
	phase     protocol.Phase
	accountID int64               // set once the login key is redeemed
	empire    account.Empire      // the account's kingdom as this connection last knew it
	playing   *character          // set once the player pressed Start
	inventory item.Inventory      // what playing owns, read on Start
	gold      uint32              // playing's yang, read with the inventory and kept as it changes
	shop      uint16              // the kind of the shopkeeper whose shop is open, 0 for none
	taskbar   map[uint8]shownSlot // playing's taskbar, by slot, read with the inventory
	claim     *claim              // this connection's hold on playing
	flood     floodLimit          // the lines this player said lately, to others
	// leftWorld is set from change character until the next Start: the client may still
	// send moves and chat it wrote before it took in the select phase.
	leftWorld bool
	// The player's last blow, for the time a blow on the same victim must wait.
	lastVictim uint32
	lastBlow   time.Time
	rand       *rand.Rand // this connection's own, for its blows' damage
}

// inFlightAfterLeaving are what the client may still send from its game window after
// Change character, dropped rather than taken as out of their phase.
var inFlightAfterLeaving = map[protocol.ClientHeader]bool{
	protocol.ClientMove: true, protocol.ClientChat: true, protocol.ClientWhisper: true,
	protocol.ClientAttack: true, protocol.ClientTarget: true, protocol.ClientOnClick: true, protocol.ClientShop: true,
}

func (s *session) Open() error { return s.conn.Send(s.sync.Greeting()...) }

func (s *session) Close() {
	s.leaveWorld()
	if s.accountID != 0 {
		s.svc.keys.Release(s.accountID)
	}
}

func (s *session) Handle(ctx context.Context, f protocol.Frame) error {
	switch {
	case f.Header == protocol.ClientStateChecker:
		return s.conn.Send(protocol.EncodeChannelStatus([]protocol.ChannelStatus{
			{Port: s.svc.public.Port(), Status: protocol.ChannelNormal},
		}))
	case f.Header == protocol.ClientMarkLogin:
		// Guild marks are not served yet; the client copes with a closed mark socket.
		return network.ErrHangUp
	case s.phase == protocol.PhaseHandshake && f.Header == protocol.ClientHandshake:
		return s.onHandshake(f.Body)
	case s.phase == protocol.PhaseLogin && f.Header == protocol.ClientLoginByKey:
		return s.onLoginByKey(ctx, f.Body)
	case s.phase == protocol.PhaseSelect && f.Header == protocol.ClientEmpire:
		return s.onEmpire(ctx, f.Body)
	case s.phase == protocol.PhaseSelect && f.Header == protocol.ClientCreateCharacter:
		return s.onCreateCharacter(ctx, f.Body)
	case s.phase == protocol.PhaseSelect && f.Header == protocol.ClientSelectCharacter:
		return s.onSelectCharacter(ctx, f.Body)
	case s.phase == protocol.PhaseLoading && f.Header == protocol.ClientSelectCharacter:
		return s.onSelectAgain(f.Body)
	case s.phase == protocol.PhaseLoading && f.Header == protocol.ClientVersion:
		return s.onVersion(f.Body)
	case s.phase == protocol.PhaseLoading && f.Header == protocol.ClientEnterGame:
		return s.onEnterGame()
	case s.phase == protocol.PhaseGame && f.Header == protocol.ClientMove:
		return s.onMove(f.Body)
	case s.phase == protocol.PhaseGame && f.Header == protocol.ClientChat:
		return s.onChat(ctx, f.Body)
	case s.phase == protocol.PhaseGame && f.Header == protocol.ClientWhisper:
		return s.onWhisper(f.Body)
	case s.phase == protocol.PhaseGame && f.Header == protocol.ClientTarget:
		return s.onTarget(f.Body)
	case s.phase == protocol.PhaseGame && f.Header == protocol.ClientAttack:
		return s.onAttack(f.Body)
	case s.phase == protocol.PhaseGame && f.Header == protocol.ClientOnClick:
		return s.onClick(f.Body)
	case s.phase == protocol.PhaseGame && f.Header == protocol.ClientShop:
		return s.onShop(f.Body)
	case s.phase == protocol.PhaseGame && f.Header == protocol.ClientQuickslotAdd:
		return s.onQuickslotAdd(f.Body)
	case s.phase == protocol.PhaseGame && f.Header == protocol.ClientQuickslotDel:
		return s.onQuickslotDel(f.Body)
	case s.phase == protocol.PhaseGame && f.Header == protocol.ClientQuickslotSwap:
		return s.onQuickslotSwap(f.Body)
	case s.phase == protocol.PhaseGame && f.Header == protocol.ClientItemMove:
		return s.onItemMove(f.Body)
	case s.phase == protocol.PhaseGame && f.Header == protocol.ClientItemUse:
		return s.onItemUse(f.Body)
	case s.phase == protocol.PhaseGame && f.Header == protocol.ClientItemPickup:
		return s.onItemPickup(f.Body)
	case s.phase == protocol.PhaseGame && f.Header == protocol.ClientItemDrop2:
		return s.onItemDrop(f.Body)
	case s.phase == protocol.PhaseGame && itemPackets[f.Header]:
		s.conn.Log().Debug("item packet not handled yet", "header", byte(f.Header))
		return nil
	case s.leftWorld && (inFlightAfterLeaving[f.Header] || itemPackets[f.Header]):
		return nil
	}
	return unexpected(f, s.phase)
}

// maxLoggedBody is how much of an unexpected packet's body its error shows.
const maxLoggedBody = 64

// unexpected is the error for a packet out of its phase. It shows the start of the body,
// which is what says what the client was doing, but none of a login packet's, which holds a
// key or a password.
func unexpected(f protocol.Frame, phase protocol.Phase) error {
	body := f.Body[:min(len(f.Body), maxLoggedBody)]
	if f.Header == protocol.ClientLogin || f.Header == protocol.ClientLoginByKey {
		body = nil
	}
	return fmt.Errorf("game: header 0x%02X not expected in phase %d: % x", byte(f.Header), phase, body)
}

func (s *session) onHandshake(body []byte) error {
	done, resend, err := s.sync.Reply(body)
	if err != nil {
		return err
	}
	if !done {
		return s.conn.Send(resend)
	}
	s.phase = protocol.PhaseLogin
	return s.conn.Send(protocol.EncodePhase(protocol.PhaseLogin))
}

// onLoginByKey sends what the selection screens need. The client picks its screen when
// the select phase arrives: with a kingdom known it shows the characters, without one it
// asks for a kingdom first. So the kingdom and the list must come before the phase.
func (s *session) onLoginByKey(ctx context.Context, body []byte) error {
	login, err := protocol.DecodeLoginByKey(body)
	if err != nil {
		return err
	}
	id, ok := s.svc.keys.Redeem(login.Key, login.Name)
	if !ok {
		return fmt.Errorf("game: login key refused for %q", login.Name)
	}
	s.accountID = id
	s.conn.KeepOpen() // the player may sit on the selection screens for as long as they like

	acc, err := s.svc.store.AccountByID(ctx, id)
	if err != nil {
		return err
	}
	s.empire = acc.Empire
	var packets [][]byte
	if acc.Empire != account.EmpireNone {
		characters, err := s.svc.store.Characters(ctx, id)
		if err != nil {
			return err
		}
		packets = append(packets, protocol.EncodeEmpire(empireNumbers[acc.Empire]),
			protocol.EncodeCharacterList(slots(characters), s.svc.public))
	}
	s.phase = protocol.PhaseSelect
	s.conn.Log().Info("entered selection", "account", acc.Name, "empire", acc.Empire)
	return s.conn.Send(append(packets, protocol.EncodePhase(protocol.PhaseSelect))...)
}

// onEmpire stores the kingdom the player picked. The client sends it from the kingdom
// screen, and again from the screen it shows before the first character is created, where
// the player may pick another; nothing is sent back. Once the account has a character,
// only the same kingdom again is accepted.
func (s *session) onEmpire(ctx context.Context, body []byte) error {
	raw, err := protocol.DecodeEmpire(body)
	if err != nil {
		return err
	}
	empire, ok := empireFromNumber(raw)
	if !ok {
		return fmt.Errorf("game: kingdom %d does not exist", raw)
	}
	if empire == s.empire {
		return nil // a repeat, for example a double click; nothing to store
	}
	chosen, err := s.svc.store.ChooseEmpire(ctx, s.accountID, empire)
	if err != nil {
		return err
	}
	if !chosen {
		return fmt.Errorf("game: kingdom %q refused: the account has characters in another", empire)
	}
	s.empire = empire
	return nil
}

// onCreateCharacter creates a character in an empty slot. A request the rules refuse gets
// the failure reply the create screen shows, and the connection stays open.
func (s *session) onCreateCharacter(ctx context.Context, body []byte) error {
	req, err := protocol.DecodeCreateCharacter(body)
	if err != nil {
		return err
	}
	x, y, ok := s.empire.StartPosition()
	if !ok {
		return errors.New("game: a character needs a kingdom first")
	}
	class, sex, ok := raceFromNumber(req.Race)
	if !ok || req.Slot >= protocol.CharacterSlots || req.Shape > 1 || !account.ValidCharacterName(req.Name) {
		s.conn.Log().Info("character refused", "name", req.Name, "race", req.Race, "slot", req.Slot, "shape", req.Shape)
		return s.conn.Send(protocol.EncodeCreateFailure(protocol.CreateRefused))
	}
	stats, _ := class.StartingStats() // every class raceFromNumber returns has them
	c := account.Character{
		Slot:  int(req.Slot),
		Name:  req.Name,
		Class: class,
		Sex:   sex,
		Level: 1,
		Stats: stats,
		Shape: req.Shape,
		X:     x,
		Y:     y,
	}

	c.ID, err = s.svc.store.CreateCharacter(ctx, s.accountID, s.empire, c)
	switch {
	case errors.Is(err, account.ErrNameTaken):
		return s.conn.Send(protocol.EncodeCreateFailure(protocol.CreateNameTaken))
	case errors.Is(err, account.ErrSlotTaken):
		return s.conn.Send(protocol.EncodeCreateFailure(protocol.CreateRefused))
	case errors.Is(err, account.ErrKingdomChanged):
		// Another connection changed it. This one's screens are out of date, and logging in
		// again shows the account as it is.
		return fmt.Errorf("game: the kingdom changed while %q was being created", c.Name)
	case err != nil:
		return err
	}
	s.conn.Log().Info("character created", "name", c.Name, "class", c.Class, "sex", c.Sex)
	return s.conn.Send(protocol.EncodeCharacterCreated(req.Slot, summary(c), s.svc.public))
}
