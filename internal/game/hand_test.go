package game

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/mob"
	"github.com/SatoriAI/MT2-Go/internal/network"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/world"
)

// bank stands in for the store's yang, which the third hand keeps off the world loop's
// goroutine: under its own lock, refusing every write with fail set.
type bank struct {
	*keptItems
	mu   sync.Mutex
	gold uint32
	fail error
}

func (b *bank) AddGold(_ context.Context, _, amount uint32) (uint32, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.fail != nil {
		return 0, b.fail
	}
	b.gold += amount
	return b.gold, nil
}

func (b *bank) Gold(context.Context, uint32) (uint32, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.gold, nil
}

// handed is Hendra, character 7, beside dogs that leave 7 yang each and nothing else, with
// 20 yang, wearing the third hand at 97 until a day after noon unless worn is false, when it
// is in her bag; her client read up to now.
func handed(t *testing.T, worn bool, dogs int) (*session, *bank, *present, net.Conn) {
	t.Helper()
	spawns := make([]world.Spawn, dogs)
	for i := range spawns {
		spawns[i] = world.Spawn{VNum: 101, X: homeX + int32(i)*10, Y: homeY, Count: 1}
	}
	svc := worldWith(t, spawns...)
	for _, m := range svc.monsters {
		rolled := *m.proto
		rolled.Drops, rolled.Picks, rolled.Yang = nil, nil, mob.Yang{Min: 7, Max: 7, Chance: 10000}
		m.proto = &rolled
	}
	svc.now = func() time.Time { return noon }
	b := &bank{keptItems: &keptItems{moved: map[int64]uint16{}, worn: map[int64]item.Wear{}, count: map[int64]uint8{}}, gold: 20}
	svc.items = b
	client, p := aPlayer(t, svc, 1000, 7, "Hendra", 100)
	drain(t, client)
	hand := item.Item{ID: 5, VNum: item.ThirdHand, Count: 1, Cell: 4, EndsAt: noon.Add(24 * time.Hour)}
	if worn {
		hand.Worn = item.Unique1
	}
	s := &session{svc: svc, conn: p.out.(*network.Conn), phase: protocol.PhaseGame, playing: playingAs(p), gold: 20,
		inventory: item.Inventory{hand}, me: p}
	if err := s.dress(); err != nil {
		t.Fatal(err)
	}
	s.keepLasting()
	drain(t, client)
	return s, b, p, client
}

// kill has Hendra kill the dog at noon, taking all its hit points.
func kill(s *session, p *present, m *monster) {
	m.p.fight.damageBy = map[*present]int{p: 100}
	s.svc.record.held(func(l locked) { l.kill(p, m.p, noon) })
}

// Worn, the third hand takes a dog's yang straight to her: nothing lies, and she is told
// her yang, which the store keeps and her connection reads before it next uses it.
func TestTheThirdHandTakesTheYangStraightToItsWearer(t *testing.T) {
	s, b, p, client := handed(t, true, 1)
	kill(s, p, s.svc.monsters[0])

	s.svc.tick(noon)

	if got := packets(t, drain(t, client)); !holds(got, pointTold(1000, protocol.PointGold, 7, 27)) {
		t.Fatalf("told %x", got)
	}
	if len(s.svc.record.state.ground) != 0 {
		t.Fatal("the yang lay on the ground")
	}
	if _, err := s.catchUp(context.Background()); err != nil || s.gold != 27 || b.gold != 27 {
		t.Fatalf("holds %d, kept %d, %v", s.gold, b.gold, err)
	}
}

// Many dogs falling in one tick are told as one sum, not a line each.
func TestTheThirdHandTellsATicksYangAtOnce(t *testing.T) {
	s, _, p, client := handed(t, true, 3)
	for _, m := range s.svc.monsters {
		kill(s, p, m)
	}

	s.svc.tick(noon)

	if got := packets(t, drain(t, client)); !holds(got, pointTold(1000, protocol.PointGold, 21, 41)) {
		t.Fatalf("told %x", got)
	}
}

// In her bag it does nothing: the yang falls.
func TestTheThirdHandInTheBagPicksNothingUp(t *testing.T) {
	s, _, p, _ := handed(t, false, 1)
	kill(s, p, s.svc.monsters[0])

	s.svc.tick(noon)

	if len(s.svc.record.state.ground) != 1 {
		t.Fatalf("%d lie", len(s.svc.record.state.ground))
	}
}

// Yang kept for another, who took most of the dog, falls, though she struck last.
func TestTheThirdHandTakesOnlyItsWearersYang(t *testing.T) {
	s, _, p, _ := handed(t, true, 1)
	_, kira := aPlayer(t, s.svc, 1001, 8, "Kira", 300)
	m := s.svc.monsters[0]
	m.p.fight.damageBy = map[*present]int{p: 10, kira: 90}
	s.svc.record.held(func(l locked) { l.kill(p, m.p, noon) })

	s.svc.tick(noon)

	for _, it := range s.svc.record.state.ground {
		if it.gold != 7 || it.owner.id != 8 {
			t.Fatalf("lies %+v", it)
		}
	}
	if len(s.svc.record.state.ground) != 1 {
		t.Fatal("Kira's yang did not fall")
	}
}

// Yang the store will not keep, past what she may hold, falls at her feet, hers.
func TestYangTheThirdHandCannotKeepFallsAtItsWearersFeet(t *testing.T) {
	s, b, p, client := handed(t, true, 1)
	b.fail = errors.New("past the cap")
	kill(s, p, s.svc.monsters[0])

	s.svc.tick(noon)

	if got := packets(t, drain(t, client)); !slices.ContainsFunc(got, func(b []byte) bool { return protocol.ServerHeader(b[0]) == protocol.ServerGroundAdd }) {
		t.Fatalf("told %x", got)
	}
	s.svc.record.held(func(l locked) {
		for _, it := range l.ground {
			if it.gold != 7 || it.owner.id != 7 || it.x != p.add.X {
				t.Fatalf("lies %+v", it)
			}
		}
	})
}

// Putting it on shows its icon, taking it off takes the icon away.
func TestTheThirdHandShowsItsIconWhileWorn(t *testing.T) {
	s, _, _, client := handed(t, false, 0)

	if err := s.onItemUse(context.Background(), doubleClicked(4)); err != nil {
		t.Fatal(err)
	}
	on := packets(t, drain(t, client))
	if err := s.onItemUse(context.Background(), doubleClicked(97)); err != nil {
		t.Fatal(err)
	}
	off := packets(t, drain(t, client))

	if !holds(on, handShown()) || !holds(off, handGone()) {
		t.Fatalf("on %x\noff %x", on, off)
	}
}

// Its tooltip reads its end, in unix seconds, from the last socket.
func TestTheThirdHandShowsItsEnd(t *testing.T) {
	ends := noon.Add(30 * 24 * time.Hour)

	got := itemSet(item.Item{ID: 5, VNum: item.ThirdHand, Count: 1, Cell: 4, EndsAt: ends})

	want := protocol.EncodeItemSet(protocol.ItemPos{Window: 1, Cell: 4}, item.ThirdHand, 1,
		protocol.Sockets{2: int32(ends.Unix())}, protocol.Bonuses{}) //nolint:gosec // a test's date
	if string(got) != string(want) {
		t.Fatalf("got  %x\nwant %x", got, want)
	}
}

// At its end, worn or not, it is gone: the client is told, its icon goes, and it picks up
// nothing more; her connection then forgets it.
func TestTheThirdHandEndsOnTime(t *testing.T) {
	s, b, p, client := handed(t, true, 1)

	s.svc.tick(noon.Add(24 * time.Hour))
	got := packets(t, drain(t, client))
	kill(s, p, s.svc.monsters[0])
	s.svc.tick(noon.Add(24 * time.Hour))

	if !holds(got, itemCleared(item.Item{Worn: item.Unique1}), handGone(), infoLine("Czas działania przedmiotu Trzecia Ręka minął.")) {
		t.Fatalf("told %x", got)
	}
	if len(s.svc.record.state.ground) != 1 {
		t.Fatal("the yang after its end did not fall")
	}
	if _, err := s.catchUp(context.Background()); err != nil || len(s.inventory) != 0 || len(b.gone) != 1 {
		t.Fatalf("still holds %v, forgotten %v, %v", s.inventory, b.gone, err)
	}
}

// One past its end is forgotten as she is loaded.
func TestAnEndedThirdHandIsForgottenOnLoading(t *testing.T) {
	s, kept, _ := gloved(t, 0)
	s.svc.items = &ordered{keptItems: kept, inv: item.Inventory{
		{ID: 5, VNum: item.ThirdHand, Count: 1, Cell: 4, EndsAt: noon.Add(-time.Second)}}}

	if err := s.loadInventory(context.Background(), 7); err != nil {
		t.Fatal(err)
	}

	if len(s.inventory) != 0 || len(kept.gone) != 1 {
		t.Fatalf("carrying %v, forgotten %v", s.inventory, kept.gone)
	}
}

// Given by /getitem at noon, the third hand ends thirty days on, kept so and shown so, and
// the world loop watches it from then.
func TestAThirdHandGivenEndsThirtyDaysOn(t *testing.T) {
	s, kept, told := tester(t, 0, nil)
	s.svc.now = func() time.Time { return noon }

	command(t, s, "getitem 71010")

	ends := noon.Add(30 * 24 * time.Hour)
	if len(kept.added) != 1 || !kept.added[0].EndsAt.Equal(ends) {
		t.Fatalf("kept %+v", kept.added)
	}
	if got := told(); !holds(got, itemSet(item.Item{ID: 101, VNum: item.ThirdHand, Count: 1, EndsAt: ends})) {
		t.Fatalf("told %x", got)
	}
	if p := s.svc.record.state.all[1000]; !p.watched || len(p.lasting) != 1 {
		t.Fatal("not watched")
	}
}

// Its wearer gone to another map since she hurt the dog, by the ring, its yang falls by
// the body for her, as any owner's does.
func TestTheThirdHandTakesNothingFromAnotherMap(t *testing.T) {
	s, _, p, _ := handed(t, true, 1)
	s.svc.record.held(func(locked) { p.world = "metin2_map_b1" })
	kill(s, p, s.svc.monsters[0])

	s.svc.tick(noon)

	if len(s.svc.record.state.ground) != 1 {
		t.Fatalf("%d lie", len(s.svc.record.state.ground))
	}
}

// Catching up shows the client the balance it read, in case a credit's reached the client
// after one of its own.
func TestCatchingUpOnTheHandsYangShowsTheBalance(t *testing.T) {
	s, _, p, client := handed(t, true, 1)
	kill(s, p, s.svc.monsters[0])
	s.svc.tick(noon)
	drain(t, client)

	if _, err := s.catchUp(context.Background()); err != nil {
		t.Fatal(err)
	}

	if got := packets(t, drain(t, client)); !holds(got, pointTold(1000, protocol.PointGold, 0, 27)) {
		t.Fatalf("told %x", got)
	}
}

// One that has ended is not watched again by a move of another item before her
// connection forgets it, so it ends once.
func TestAnEndedThirdHandEndsOnce(t *testing.T) {
	s, _, _, client := handed(t, true, 0)
	s.inventory = append(s.inventory, item.Item{ID: 6, VNum: 27001, Count: 1, Cell: 10})
	s.svc.tick(noon.Add(24 * time.Hour))
	drain(t, client)
	at(s, 24*time.Hour)

	moved := slices.Concat([]byte{1}, binary.LittleEndian.AppendUint16(nil, 10), []byte{1},
		binary.LittleEndian.AppendUint16(nil, 11), []byte{1})
	if err := s.onItemMove(moved); err != nil {
		t.Fatal(err)
	}
	drain(t, client)
	s.svc.tick(noon.Add(24*time.Hour + time.Second))

	if got := packets(t, drain(t, client)); len(got) != 0 {
		t.Fatalf("told %x", got)
	}
}
