package game

import (
	"context"
	"net"
	"net/netip"
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

var gameServer = netip.MustParseAddrPort("192.168.1.200:13001")

const ringQuestion = "Dokąd chcesz się przenieść?[ENTER][QUESTION 1;Strefa Yongan|2;Anuluj]"

// ringing is Hendra of Shinsoo, character 7, at level level in play far from her
// village, with the teleport ring at cell 4, and Kira beside her.
func ringing(t *testing.T, level uint8) (*session, *saves, net.Conn, net.Conn) {
	t.Helper()
	s, _, client, viewer := dressing(t)
	store := newSaves()
	s.svc.accounts, s.svc.public, s.empire = store, gameServer, account.EmpireShinsoo
	s.playing.ID, s.playing.Level = 7, level
	s.inventory = append(s.inventory, item.Item{ID: 5, VNum: item.TeleportRing, Count: 1, Cell: 4})
	claim, _, err := s.svc.claimCharacter(context.Background(), 7, func() {})
	if err != nil {
		t.Fatal(err)
	}
	s.claim = claim
	return s, store, client, viewer
}

func useRing(t *testing.T, s *session) {
	t.Helper()
	if err := s.onItemUse(context.Background(), doubleClicked(4)); err != nil {
		t.Fatal(err)
	}
}

func answer(t *testing.T, s *session, button uint8) {
	t.Helper()
	if err := s.onScriptAnswer(context.Background(), answered(button)); err != nil {
		t.Fatal(err)
	}
}

// The ring asks where to go, offering her kingdom's village by the client's name for it
// and a cancel; the ring stays in her bag.
func TestTheRingAsksWhereToGo(t *testing.T) {
	s, _, client, _ := ringing(t, 30)

	useRing(t, s)

	expectSaid(t, client, ringQuestion)
	if _, held := s.inventory.ByID(5); !held {
		t.Fatal("the ring was used up")
	}
}

// Picking the village takes her out of Kira's view, saves her at her kingdom's start, and
// tells her client to connect again there; the ring is still hers.
func TestPickingTheVillageWarpsThePlayerToTheStart(t *testing.T) {
	s, store, client, viewer := ringing(t, 30)
	useRing(t, s)
	drain(t, client)

	answer(t, s, 0)

	x, y, _ := account.EmpireShinsoo.StartPosition()
	got := packets(t, drain(t, client))
	if !holds(got, protocol.EncodeWarp(x, y, gameServer)) {
		t.Fatalf("told %x, want a warp to (%d, %d)", got, x, y)
	}
	if seen := packets(t, drain(t, viewer)); !holds(seen, protocol.EncodeCharacterDelete(1000)) {
		t.Fatalf("Kira saw %x", seen)
	}
	if _, at, _ := store.ofHendra(); at != [2]int32{x, y} {
		t.Fatalf("saved at %v, want (%d, %d)", at, x, y)
	}
	if _, inWorld := s.svc.record.state.all[1000]; inWorld || s.svc.claims[7] != nil || s.phase != protocol.PhaseSelect {
		t.Fatalf("in the world %v, claimed %v, in phase %d", inWorld, s.svc.claims[7] != nil, s.phase)
	}
	if _, held := s.inventory.ByID(5); !held {
		t.Fatal("the ring was used up")
	}
}

// What the client still sends from the game before it lets go of the connection is dropped.
func TestAMoveAfterAWarpIsDropped(t *testing.T) {
	s, _, client, _ := ringing(t, 30)
	useRing(t, s)
	answer(t, s, 0)
	drain(t, client)

	move := protocol.Frame{Header: protocol.ClientMove, Body: make([]byte, 16)}
	if err := s.Handle(context.Background(), move); err != nil {
		t.Fatal(err)
	}
}

// Cancel takes her nowhere.
func TestCancellingTheRingGoesNowhere(t *testing.T) {
	s, _, client, _ := ringing(t, 30)
	useRing(t, s)
	drain(t, client)

	answer(t, s, 1)

	expectNothing(t, client)
	if _, inWorld := s.svc.record.state.all[1000]; !inWorld {
		t.Fatal("she left the world")
	}
}

// Below level 30, item_proto's limit, the ring asks nothing and says why.
func TestTheRingRefusesAPlayerBelowItsLevel(t *testing.T) {
	s, _, client, _ := ringing(t, 29)

	useRing(t, s)

	if got := read(t, client, len(infoLine(ringTooLow))); string(got) != string(infoLine(ringTooLow)) {
		t.Fatalf("told %q", got)
	}
	if s.talking != nil {
		t.Fatal("asked where to go")
	}
}

// With a shop open the ring is refused.
func TestTheRingIsRefusedAtAShop(t *testing.T) {
	s, _, client, _ := ringing(t, 30)
	s.shop = 9003

	useRing(t, s)

	if got := read(t, client, len(infoLine(ringRefused))); string(got) != string(infoLine(ringRefused)) {
		t.Fatalf("told %q", got)
	}
}

// One who died with the question open is not taken anywhere when they answer it.
func TestTheRingTakesNobodyWhoDiedSinceTheQuestion(t *testing.T) {
	s, _, client, _ := ringing(t, 30)
	useRing(t, s)
	drain(t, client)
	s.svc.record.held(func(l locked) { l.all[1000].char.HP = 0 })

	answer(t, s, 0)

	if got := packets(t, drain(t, client)); len(got) != 1 || !holds(got, infoLine(ringRefused)) {
		t.Fatalf("told %x", got)
	}
}
