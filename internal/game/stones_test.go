package game

import (
	"bytes"
	"math"
	"net"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/mob"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/world"
)

// aStone is a world with one Metin Cierpienia at the monsters' home, its only spot, its
// first wave about it, and a player at x east of it, with what entering showed them read.
func aStone(t *testing.T, x int32) (*Service, *monster, *present, net.Conn) {
	t.Helper()
	s := worldWith(t, world.Spawn{VNum: 8001, Count: 1, Spots: []world.Spot{{X: homeX, Y: homeY}}})
	client, p := aPlayer(t, s, 1000, 7, "Hendra", x)
	drain(t, client)
	return s, s.monsters[0], p, client
}

// strikeStone takes damage off the stone for the player, as a blow or skill would.
func strikeStone(s *Service, p *present, stone *monster, damage int, now time.Time) {
	s.presence.mu.Lock()
	defer s.presence.mu.Unlock()
	s.presence.strike(p, stone.p, damage, now)
}

// alive is the monsters the stone called that are alive.
func alive(s *Service, stone *monster) []*monster {
	var living []*monster
	for _, m := range s.called[stone] {
		if m.diedAt.IsZero() && !m.gone {
			living = append(living, m)
		}
	}
	return living
}

// A stone stands at its spot as the client's stone, and calls its first wave out about it:
// three wolves and two alpha wolves, each 500 to 800 from it.
func TestAStoneAppearsAsAStoneWithItsFirstWaveAboutIt(t *testing.T) {
	s, stone, _, _ := aStone(t, 20000)

	if stone.p.add.Type != protocol.EntityStone || stone.x != homeX || stone.y != homeY || stone.wanders {
		t.Fatalf("the stone: %+v at (%d, %d), wandering %v", stone.p.add, stone.x, stone.y, stone.wanders)
	}
	kinds := map[uint16]int{}
	for _, m := range alive(s, stone) {
		kinds[m.proto.VNum]++
		if d := math.Hypot(float64(m.x-homeX), float64(m.y-homeY)); d < waveFrom-1 || d > waveFrom+waveDepth+1 {
			t.Errorf("%d stands %.0f from the stone", m.proto.VNum, d)
		}
	}
	if kinds[102] != 3 || kinds[103] != 2 || len(kinds) != 2 {
		t.Fatalf("the first wave: %v, want three wolves and two alpha wolves", kinds)
	}
}

// A stone hit neither walks nor bites: it is not among those that fight back.
func TestAHitStoneNeverFightsBack(t *testing.T) {
	s, stone, p, _ := aStone(t, 100)

	strikeStone(s, p, stone, 10, noon)
	s.tick(noon)

	if stone.chase != nil || len(s.fighting) != 0 {
		t.Fatalf("the stone fights: chase %+v, %d fighting", stone.chase, len(s.fighting))
	}
}

// Each fifth of its hit points lost calls a wave on the player who wore it down: with its
// first wave dead, the whole of the next comes, each monster chasing them.
func TestAStoneWornDownAFifthCallsAWaveOnThePlayer(t *testing.T) {
	s, stone, p, _ := aStone(t, 100)
	for _, m := range alive(s, stone) {
		m.diedAt = noon
	}

	strikeStone(s, p, stone, 1400, noon) // 80 in a hundred left
	s.tick(noon)

	wave := alive(s, stone)
	if len(wave) != 5 {
		t.Fatalf("%d called, want a wave of 5", len(wave))
	}
	for _, m := range wave {
		if m.chase == nil || m.chase.target != p {
			t.Errorf("%d does not chase the player: %+v", m.proto.VNum, m.chase)
		}
	}
}

// A stone keeps at most six of its monsters alive: with its first wave about it, the next
// adds one.
func TestAStoneKeepsAtMostSixOfItsMonstersAlive(t *testing.T) {
	s, stone, p, _ := aStone(t, 100)

	strikeStone(s, p, stone, 6999, noon) // past all four marks at once
	s.tick(noon)

	if got := len(alive(s, stone)); got != waveCap {
		t.Fatalf("%d alive, want %d", got, waveCap)
	}
}

// A stone calls each wave once: healed back over a mark and worn past it again, it calls
// nothing more.
func TestAStoneCallsEachWaveOnce(t *testing.T) {
	s, stone, p, _ := aStone(t, 100)
	for _, m := range alive(s, stone) {
		m.diedAt = noon
	}
	strikeStone(s, p, stone, 1400, noon)
	s.tick(noon)
	for _, m := range alive(s, stone) {
		m.diedAt = noon
	}

	stone.p.fight.hp = stone.p.fight.maxHP
	strikeStone(s, p, stone, 1400, noon.Add(time.Second))
	s.tick(noon.Add(time.Second))

	if got := len(alive(s, stone)); got != 0 {
		t.Fatalf("%d called again", got)
	}
}

// A stone heals 1 in a hundred every five seconds, but only once ten seconds have passed
// without a hit.
func TestAStoneHealsOnlyAfterTenSecondsUnhit(t *testing.T) {
	s, stone, p, _ := aStone(t, 20000)
	strikeStone(s, p, stone, 1000, noon)
	s.tick(noon)
	hurt := stone.p.fight.hp

	s.tick(noon.Add(5 * time.Second))
	if got := stone.p.fight.hp; got != hurt {
		t.Fatalf("healed to %d five seconds after a hit, from %d", got, hurt)
	}
	s.tick(noon.Add(10 * time.Second))
	if got := stone.p.fight.hp; got != hurt+70 {
		t.Fatalf("healed to %d ten seconds after a hit, want %d", got, hurt+70)
	}
}

// As it is worn down, the stone shows those who see it its smoke, and a player coming into
// view sees the smoke it has.
func TestAWornStoneShowsItsSmoke(t *testing.T) {
	s, stone, p, client := aStone(t, 100)

	strikeStone(s, p, stone, 1100, noon) // 84 in a hundred left
	if got := drain(t, client); !bytes.Contains(got, protocol.EncodeCharacterUpdate(stone.p.info, stone.p.add, 1<<1)) {
		t.Fatalf("got %x, want the first smoke", got)
	}

	newcomer, _ := aPlayer(t, s, 1001, 8, "Kira", 200)
	if got := drain(t, newcomer); !bytes.Contains(got, protocol.EncodeCharacterAdd(stone.p.add, 1<<1)) {
		t.Fatalf("the newcomer got %x, want the stone with its smoke", got)
	}
}

// The player who destroys a stone is told so, and it leaves them its yang, potions of one
// kind and materials of one kind.
func TestADestroyedStoneTellsItsWinnerAndLeavesThemItsDrops(t *testing.T) {
	s, stone, p, client := aStone(t, 100)
	strikeStone(s, p, stone, 7000, noon)

	if got := drain(t, client); !bytes.Contains(got, protocol.EncodeChat(protocol.ChatInfo, 0, 0, textToClient("Zniszczyłeś kamień Metin."))) {
		t.Fatalf("got %x, want the winner told", got)
	}
	s.tick(noon)

	kinds := map[uint32]uint8{}
	var yang uint32
	for _, it := range s.presence.ground {
		if it.owner.id != 7 {
			t.Errorf("%+v is not kept for the winner", it)
		}
		kinds[it.vnum] += it.count
		yang += it.gold
	}
	potions, materials := kinds[27001]+kinds[27004], kinds[30028]+kinds[30004]
	if potions != 3 || materials != 2 || yang < 500 || yang > 800 {
		t.Fatalf("left %v and %d yang, want 3 potions, 2 materials and 500 to 800 yang", kinds, yang)
	}
	if (kinds[27001] != 0) == (kinds[27004] != 0) || (kinds[30028] != 0) == (kinds[30004] != 0) {
		t.Fatalf("left %v, want one kind of potion and one of material", kinds)
	}
}

// Ten minutes after it is destroyed the stone comes back, at one of its spots, and every
// player on its map is told; what it called before and is still about is sent away.
func TestADestroyedStoneComesBackAfterTenMinutesAndIsAnnounced(t *testing.T) {
	s, stone, p, client := aStone(t, 100)
	far, _ := aPlayer(t, s, 1001, 8, "Kira", 30000) // on the map, out of view
	drain(t, far)
	first := alive(s, stone)
	strikeStone(s, p, stone, 7000, noon)
	s.tick(noon)
	s.tick(noon.Add(corpseTime))
	drain(t, client)

	s.tick(noon.Add(10*time.Minute - time.Second))
	if !stone.gone {
		t.Fatal("back before ten minutes")
	}
	s.tick(noon.Add(10 * time.Minute))

	if stone.gone || stone.x != homeX || stone.y != homeY {
		t.Fatalf("not back at its spot: gone %v at (%d, %d)", stone.gone, stone.x, stone.y)
	}
	notice := protocol.EncodeChat(protocol.ChatNotice, 0, 0, textToClient("Pojawił się Metin Cierpienia."))
	if got := drain(t, far); !bytes.Contains(got, notice) {
		t.Fatalf("the far player got %x, want the notice", got)
	}
	for _, m := range first {
		if _, kept := s.byPresent[m.p]; kept || !m.gone {
			t.Errorf("%d of the first wave is still about", m.proto.VNum)
		}
	}
	if got := len(alive(s, stone)); got != 5 {
		t.Fatalf("%d called on its return, want a wave of 5", got)
	}
}

// A monster a stone called never comes back once it dies: the loop forgets it.
func TestAMonsterAStoneCalledNeverComesBack(t *testing.T) {
	s, stone, _, _ := aStone(t, 20000)
	m := alive(s, stone)[0]
	m.diedAt = noon

	s.tick(noon.Add(corpseTime))
	s.tick(noon.Add(time.Hour))

	for _, kept := range s.monsters {
		if kept == m {
			t.Fatalf("the loop still holds %d", m.proto.VNum)
		}
	}
	if _, kept := s.byPresent[m.p]; kept {
		t.Fatal("still known by its presence")
	}
}

// Every stone is a stone to the client, and calls waves the world can put down.
func TestEveryStoneKindIsShownAsAStone(t *testing.T) {
	for _, vnum := range []uint16{8001, 8002, 8003} {
		if p, _ := mob.Lookup(vnum); entityType(p.Kind) != protocol.EntityStone {
			t.Errorf("%d is shown as %d", vnum, entityType(p.Kind))
		}
	}
}

// A monster the stone called, still about when the stone comes back, is sent away before
// its stroll falls due that tick: it never walks out of the world.
func TestAMonsterSentAwayAsItsStoneComesBackNeverStrolls(t *testing.T) {
	s, stone, p, _ := aStone(t, 20000)
	left := alive(s, stone)
	strikeStone(s, p, stone, 7000, noon)
	s.tick(noon)
	s.tick(noon.Add(corpseTime))
	for _, m := range left {
		m.nextStroll = noon.Add(10 * time.Minute)
	}

	s.tick(noon.Add(10 * time.Minute))

	for _, m := range left {
		if _, kept := s.byPresent[m.p]; kept {
			t.Errorf("%d is still about", m.proto.VNum)
		}
	}
}

// A monster the stone called that a player killed since the loop last took the news is
// not sent away with the rest as the stone comes back: it dies as any monster does,
// leaving its body.
func TestAMonsterKilledAsItsStoneComesBackStillDies(t *testing.T) {
	s, stone, p, client := aStone(t, 100)
	wolf := alive(s, stone)[0]
	s.presence.mu.Lock()
	s.presence.strike(p, wolf.p, wolf.p.fight.hp, noon)
	s.presence.mu.Unlock()

	s.dismiss(stone)
	s.tick(noon)

	if wolf.diedAt.IsZero() || wolf.gone {
		t.Fatalf("the wolf: died at %v, gone %v; want it lying dead", wolf.diedAt, wolf.gone)
	}
	if got := drain(t, client); bytes.Contains(got, protocol.EncodeCharacterDelete(wolf.p.add.VID)) {
		t.Fatal("its body was taken away at once")
	}
}

// A wave that cannot all come picks its monsters at random: with the first wave about the
// stone, each later one adds a single monster, and over ten of them both kinds come.
func TestAPartWaveTakesItsKindsAtRandom(t *testing.T) {
	s, stone, _, _ := aStone(t, 20000)
	kinds := map[uint16]bool{}
	for range 10 {
		s.callWave(stone, nil, noon)
		lone := alive(s, stone)[5]
		kinds[lone.proto.VNum] = true
		lone.diedAt = noon
	}
	if !kinds[102] || !kinds[103] {
		t.Fatalf("the lone monsters were %v, want wolves and alpha wolves", kinds)
	}
}
