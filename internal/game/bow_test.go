package game

import (
	"bytes"
	"context"
	"encoding/binary"
	"math/rand/v2"
	"net"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/mob"
	"github.com/SatoriAI/MT2-Go/internal/network"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/world"
)

const flyTargetSize = 17

// archer is Cien, a ninja at x east of a wild dog, holding Łuk+0 with a stack of arrows
// worn, of count, at attack speed 100, and Kira beside her, who sees her. The dog has
// hit points enough to take any shot.
func archer(t *testing.T, x int32, count uint8) (*session, *keptItems, net.Conn, net.Conn) {
	t.Helper()
	return archerAmong(t, x, count, 1)
}

// archerAmong is archer among as many dogs, all at one spot.
func archerAmong(t *testing.T, x int32, count uint8, dogs int) (*session, *keptItems, net.Conn, net.Conn) {
	t.Helper()
	svc := worldWith(t, world.Spawn{VNum: 101, X: homeX, Y: homeY, Count: dogs})
	for _, m := range svc.monsters {
		bare := *m.proto
		bare.Drops, bare.Yang = nil, mob.Yang{}
		m.proto = &bare
		m.p.fight.hp, m.p.fight.maxHP = 100000, 100000
	}
	svc.now = func() time.Time { return noon }
	kept := &keptItems{moved: map[int64]uint16{}, worn: map[int64]item.Wear{}, count: map[int64]uint8{}}
	svc.items = kept
	client, cien := aPlayer(t, svc, 1000, 9, "Cien", x)
	c := cien.char
	c.Class, c.Level, c.HP, c.Weapon, c.Arrow = account.Ninja, 10, 500, 2000, 8000
	c.Dexterity = 30
	cien.add.AttackSpeed = 100
	viewer, kira := aPlayer(t, svc, 1001, 10, "Kira", x+100)
	kira.char.HP = 500
	drain(t, client)
	drain(t, viewer)
	s := &session{svc: svc, conn: cien.out.(*network.Conn), phase: protocol.PhaseGame, rand: rand.New(rand.NewPCG(5, 6)), //nolint:gosec // a fixed seed
		playing: playingAs(cien), inventory: item.Inventory{
			{ID: 1, VNum: 2000, Count: 1, Worn: item.Weapon}, {ID: 2, VNum: 8000, Count: count, Worn: item.Arrow}}}
	return s, kept, client, viewer
}

func flyAt(vid uint32, x, y int32) []byte {
	b := binary.LittleEndian.AppendUint32(nil, vid)
	b = binary.LittleEndian.AppendUint32(b, uint32(x))    //nolint:gosec // the bytes as sent
	return binary.LittleEndian.AppendUint32(b, uint32(y)) //nolint:gosec // the bytes as sent
}

// shot aims the player's next arrow at the monster and lets it go at a time.
func shot(t *testing.T, s *session, at time.Duration) {
	t.Helper()
	s.svc.now = func() time.Time { return noon.Add(at) }
	dog := s.svc.monsters[0].p.add
	if err := s.onFlyTarget(flyAt(dog.VID, dog.X, dog.Y)); err != nil {
		t.Fatal(err)
	}
	if err := s.onShoot([]byte{0}); err != nil {
		t.Fatal(err)
	}
}

func hurt(s *session) int { return s.svc.monsters[0].p.fight.damageBy[s.svc.record.state.all[1000]] }

// A shot at a dog 2700 away lands as her blow: she is told its damage, and one of her
// arrows is used up and shown so.
func TestAShotLandsAndUsesAnArrow(t *testing.T) {
	s, kept, client, _ := archer(t, 2700, 100)

	shot(t, s, 0)

	got := drain(t, client)
	if hurt(s) == 0 || !sent(t, got, protocol.ServerDamage) {
		t.Fatalf("the dog took %d, she got %x", hurt(s), got)
	}
	if !bytes.Contains(got, itemSet(item.Item{ID: 2, VNum: 8000, Count: 99, Worn: item.Arrow})) || kept.count[2] != 99 {
		t.Fatalf("kept %d arrows, told %x", kept.count[2], got)
	}
}

// Shots that cannot land are dropped, and use no arrow: past 2800, with no target named
// since the last shot, sooner than 1.25 s after the last at attack speed 100, without
// arrows worn, without a bow in hand, by a skill, or by the dead.
func TestAShotThatCannotLandIsDroppedAndUsesNoArrow(t *testing.T) {
	cases := map[string]struct {
		x    int32
		shot func(t *testing.T, s *session)
	}{
		"too far": {2900, func(t *testing.T, s *session) { shot(t, s, 0) }},
		"no target named": {100, func(t *testing.T, s *session) {
			if err := s.onShoot([]byte{0}); err != nil {
				t.Fatal(err)
			}
		}},
		"too soon": {100, func(t *testing.T, s *session) {
			s.lastShot = noon.Add(-1240 * time.Millisecond)
			shot(t, s, 0)
		}},
		"no arrows": {100, func(t *testing.T, s *session) {
			s.inventory = s.inventory[:1]
			s.svc.record.state.all[1000].char.Arrow = 0
			shot(t, s, 0)
		}},
		"a dagger in hand": {100, func(t *testing.T, s *session) {
			s.svc.record.state.all[1000].char.Weapon = 1000
			shot(t, s, 0)
		}},
		"by a skill": {100, func(t *testing.T, s *session) {
			dog := s.svc.monsters[0].p.add
			if err := s.onFlyTarget(flyAt(dog.VID, dog.X, dog.Y)); err != nil {
				t.Fatal(err)
			}
			if err := s.onShoot([]byte{46}); err != nil {
				t.Fatal(err)
			}
		}},
		"dead": {100, func(t *testing.T, s *session) {
			s.svc.record.state.all[1000].char.HP = 0
			shot(t, s, 0)
		}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			s, kept, client, _ := archer(t, c.x, 100)
			c.shot(t, s)
			if got := drain(t, client); hurt(s) != 0 || len(kept.count) != 0 || len(got) != 0 {
				t.Fatalf("the dog took %d, kept %v, she got %x", hurt(s), kept.count, got)
			}
		})
	}
}

// One target named lets one arrow go: a second shot without naming it again is dropped.
func TestATargetIsShotOnce(t *testing.T) {
	s, kept, client, _ := archer(t, 100, 100)
	shot(t, s, 0)
	took := hurt(s)
	drain(t, client)
	s.svc.now = func() time.Time { return noon.Add(2 * time.Second) }

	if err := s.onShoot([]byte{0}); err != nil {
		t.Fatal(err)
	}

	if hurt(s) != took || kept.count[2] != 99 {
		t.Fatalf("the dog took %d, then %d; kept %d", took, hurt(s), kept.count[2])
	}
}

// At attack speed 100 a shot 1.25 s after the last lands, and at 125 one 1 s after it.
func TestShotsFollowTheAttackSpeed(t *testing.T) {
	for speed, gap := range map[uint8]time.Duration{100: 1250 * time.Millisecond, 125: time.Second} {
		s, kept, _, _ := archer(t, 100, 100)
		s.svc.record.state.all[1000].add.AttackSpeed = speed
		shot(t, s, 0)
		shot(t, s, gap-time.Millisecond)
		if kept.count[2] != 99 {
			t.Fatalf("at %d, %d left after a shot too soon", speed, kept.count[2])
		}
		shot(t, s, gap)
		if kept.count[2] != 98 {
			t.Fatalf("at %d, %d left after a shot in time", speed, kept.count[2])
		}
	}
}

// The last arrow empties its place: it is forgotten and shown gone, she holds no arrows any
// longer, and her next shot is dropped.
func TestTheLastArrowEmptiesItsPlace(t *testing.T) {
	s, kept, client, _ := archer(t, 100, 1)

	shot(t, s, 0)

	if got := drain(t, client); !bytes.Contains(got, itemCleared(item.Item{ID: 2, VNum: 8000, Count: 1, Worn: item.Arrow})) ||
		len(kept.gone) != 1 || len(s.inventory) != 1 || s.svc.record.state.all[1000].char.Arrow != 0 {
		t.Fatalf("gone %v, carrying %+v, holding arrows %d, told %x", kept.gone, s.inventory,
			s.svc.record.state.all[1000].char.Arrow, got)
	}
	took := hurt(s)
	shot(t, s, 2*time.Second)
	if hurt(s) != took {
		t.Fatal("shot with no arrows")
	}
}

// Kira sees what Cien aims at, as the client's FLY_TARGETING and ADD_FLY_TARGETING with
// Cien's vid before the target, so her client draws the arrows flying there.
func TestWhatAnArcherAimsAtIsShown(t *testing.T) {
	s, _, _, viewer := archer(t, 100, 100)

	if err := s.onFlyTarget(flyAt(7, -5, 9)); err != nil {
		t.Fatal(err)
	}
	if err := s.onAddFlyTarget(flyAt(0, 300, 400)); err != nil {
		t.Fatal(err)
	}

	want := []byte{0x47, 0xe8, 3, 0, 0, 7, 0, 0, 0, 0xfb, 0xff, 0xff, 0xff, 9, 0, 0, 0,
		0x45, 0xe8, 3, 0, 0, 0, 0, 0, 0, 0x2c, 1, 0, 0, 0x90, 1, 0, 0}
	if got := read(t, viewer, 2*flyTargetSize); !bytes.Equal(got, want) {
		t.Fatalf("got  %x\nwant %x", got, want)
	}
}

// Nobody is shown the aim of an archer who is dead or holds no bow.
func TestTheAimOfTheDeadOrBowlessIsNotShown(t *testing.T) {
	for name, change := range map[string]func(c *account.Character){
		"dead":   func(c *account.Character) { c.HP = 0 },
		"no bow": func(c *account.Character) { c.Weapon = 1000 },
	} {
		s, _, _, viewer := archer(t, 100, 100)
		change(s.svc.record.state.all[1000].char)
		if err := s.onFlyTarget(flyAt(7, -5, 9)); err != nil {
			t.Fatal(err)
		}
		if got := drain(t, viewer); len(got) != 0 {
			t.Fatalf("%s: Kira saw %x", name, got)
		}
	}
}

// A bow never strikes in melee: a blow she reports from beside the dog lands nothing.
func TestABowDoesNotStrikeInMelee(t *testing.T) {
	s, _, _, _ := archer(t, 100, 100)

	if got := s.svc.record.attack(1000, s.svc.monsters[0].p.add.VID, 0, someRand, noon); got != blowRefused || hurt(s) != 0 {
		t.Fatalf("blow %d, the dog took %d", got, hurt(s))
	}
}

// Arrows add their 3 to the bow's least attack, 7 to 29 becoming 10 to 29, and nothing to
// any other weapon's: the dagger's stays 8 to 11.
func TestArrowsAddToTheBowsAttack(t *testing.T) {
	c := &account.Character{Weapon: 2000, Arrow: 8000}
	if a := attackerOf(c); a.WeaponMin != 10 || a.WeaponMax != 29 {
		t.Fatalf("with a bow %d to %d", a.WeaponMin, a.WeaponMax)
	}
	c.Weapon = 1000
	if a := attackerOf(c); a.WeaponMin != 8 || a.WeaponMax != 11 {
		t.Fatalf("with a dagger %d to %d", a.WeaponMin, a.WeaponMax)
	}
}

// Arrows double-clicked in her bag are put on in their own place, 99 on the wire, and the
// bow shoots them.
func TestArrowsArePutOnInTheirPlace(t *testing.T) {
	s, kept, client, _ := archer(t, 100, 100)
	s.inventory = s.inventory[:1]
	s.inventory = append(s.inventory, item.Item{ID: 3, VNum: 8000, Count: 50, Cell: 4})
	s.svc.record.state.all[1000].char.Arrow = 0

	if err := s.onItemUse(context.Background(), doubleClicked(4)); err != nil {
		t.Fatal(err)
	}

	if got := drain(t, client); kept.worn[3] != item.Arrow || s.svc.record.state.all[1000].char.Arrow != 8000 ||
		!bytes.Contains(got, protocol.EncodeItemSet(protocol.ItemPos{Window: 1, Cell: 99}, 8000, 50, protocol.Bonuses{})) {
		t.Fatalf("worn %q, holding %d, told %x", kept.worn[3], s.svc.record.state.all[1000].char.Arrow, got)
	}
	shot(t, s, 0)
	if kept.count[3] != 49 {
		t.Fatalf("%d left", kept.count[3])
	}
}

// The weapon trader's window holds every ware at its cell and price: a hundred arrows,
// under the dagger, for 20, five a gold, and each weapon for its kind's price.
func TestTheWeaponTraderSellsArrowsByTheGold(t *testing.T) {
	svc := worldWith(t, world.Spawn{VNum: 9001, X: homeX, Y: homeY, Count: 1})
	client, hendra := aPlayer(t, svc, 1000, 7, "Hendra", 100)
	read(t, client, addSize+infoSize)
	s := &session{svc: svc, conn: hendra.out.(*network.Conn), phase: protocol.PhaseGame,
		playing: playingAs(hendra)}

	if err := s.onClick(vidBody(svc.lastVID.Load())); err != nil {
		t.Fatal(err)
	}

	var cells [protocol.ShopCells]protocol.ShopItem
	trader, _ := mob.Lookup(9001)
	for _, w := range trader.Sells {
		kind, _ := item.Lookup(w.VNum)
		cells[w.Cell] = protocol.ShopItem{VNum: w.VNum, Price: kind.PriceOf(w.Count), Count: w.Count}
	}
	if cells[6] != (protocol.ShopItem{VNum: 8000, Price: 20, Count: 100}) || cells[19] != (protocol.ShopItem{VNum: 3040, Price: 3000, Count: 1}) {
		t.Fatalf("arrows %+v, the glaive of 20 %+v", cells[6], cells[19])
	}
	if got := read(t, client, 1728); string(got) != string(protocol.EncodeShopStart(svc.lastVID.Load(), cells)) {
		t.Fatalf("got %x", got[:64])
	}
}
