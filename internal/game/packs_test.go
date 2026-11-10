package game

import (
	"math"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/mob"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/world"
)

// packed is a world of the spawns, every monster with hit points to spare and its drops
// taken away, and a player at the monsters' home to be their target.
func packed(t *testing.T, spawns ...world.Spawn) (*Service, *present) {
	t.Helper()
	s := worldWith(t, spawns...)
	for _, m := range s.monsters {
		bare := *m.proto
		bare.Drops, bare.Yang = nil, mob.Yang{}
		m.proto = &bare
		m.p.fight.hp, m.p.fight.maxHP = 100000, 100000
		m.nextStroll = noon.Add(time.Hour)
	}
	_, hendra := aPlayer(t, s, 1000, 7, "Hendra", 0)
	return s, hendra
}

// of are the world's monsters of a kind.
func of(s *Service, vnum uint16) []*monster {
	var got []*monster
	for _, m := range s.monsters {
		if m.proto.VNum == vnum {
			got = append(got, m)
		}
	}
	return got
}

// Hit one dog of a pack and the whole pack comes, while a pack beside it does not.
func TestHitOneOfAPackAndThePackComes(t *testing.T) {
	s, hendra := packed(t, world.Spawn{VNum: 101, X: homeX, Y: homeY, Count: 3},
		world.Spawn{VNum: 102, X: homeX + 500, Y: homeY, Count: 3})
	dogs := of(s, 101)

	s.engage([]provocation{{dogs[0].p, hendra}}, noon)

	for _, d := range dogs {
		if d.chase == nil || d.chase.target != hendra {
			t.Fatal("a dog of the pack stands by")
		}
	}
	for _, w := range of(s, 102) {
		if w.chase != nil {
			t.Fatal("a wolf of the other pack joined")
		}
	}
}

// A leader comes with its kin: hit a wolf, and the alpha wolf standing with them comes too.
func TestALeaderComesWithItsPack(t *testing.T) {
	s, hendra := packed(t, world.Spawn{VNum: 102, X: homeX, Y: homeY, Count: 3},
		world.Spawn{VNum: 103, X: homeX, Y: homeY, Count: 1})

	s.engage([]provocation{{of(s, 102)[0].p, hendra}}, noon)

	if a := of(s, 103)[0]; a.chase == nil || a.chase.target != hendra {
		t.Fatal("the alpha wolf stands by")
	}
}

// A packmate further than 3000 from the one hit, or already fighting another, stays out.
func TestAPackmateFarOffOrFightingStaysOut(t *testing.T) {
	s, hendra := packed(t, world.Spawn{VNum: 101, X: homeX, Y: homeY, Count: 3})
	dogs := of(s, 101)
	dogs[1].x, dogs[1].y = dogs[0].x+3001, dogs[0].y
	_, kira := aPlayer(t, s, 1001, 8, "Kira", 0)
	s.chaseOn(dogs[2], kira, noon)

	s.engage([]provocation{{dogs[0].p, hendra}}, noon)

	if dogs[1].chase != nil || dogs[2].chase.target != kira {
		t.Fatalf("the far dog fights %v, the busy one %v", dogs[1].chase != nil, dogs[2].chase.target.info.Name)
	}
}

// A pack comes back together: with two of three dogs dead, neither returns in its time;
// once the third dies, all three return its time after.
func TestAPackComesBackTogether(t *testing.T) {
	s, _ := packed(t, world.Spawn{VNum: 101, X: homeX, Y: homeY, Count: 3, Respawn: 30 * time.Second})
	dogs := of(s, 101)
	kill := func(d *monster, at time.Time) {
		s.record.held(func(l locked) {
			l.kill(s.record.state.all[1000], d.p, at)
		})
		s.tick(at)
	}
	kill(dogs[0], noon)
	kill(dogs[1], noon)

	s.tick(noon.Add(time.Minute))
	s.tick(noon.Add(time.Minute + time.Second)) // the bodies gone, a lone dog's time long past
	if !dogs[0].gone || !dogs[1].gone {
		t.Fatal("a dog came back while its pack lived")
	}
	kill(dogs[2], noon.Add(2*time.Minute))
	s.tick(noon.Add(2*time.Minute + 30*time.Second - time.Millisecond))
	if !dogs[2].gone {
		t.Fatal("back too soon")
	}
	s.tick(noon.Add(2*time.Minute + 30*time.Second))

	for _, d := range dogs {
		if d.gone || !d.diedAt.IsZero() {
			t.Fatal("not every dog is back")
		}
	}
}

// fierce is packed with a pack of three dogs, the first made aggressive and its packmates
// passive, so they come only as its pack, and Hendra at level 1 x east of their home, seen
// at once.
func fierce(t *testing.T, x int32) (*Service, *present) {
	t.Helper()
	s, hendra := packed(t, world.Spawn{VNum: 101, X: homeX, Y: homeY, Count: 3})
	for _, m := range s.monsters {
		m.x, m.y, m.p.add.X, m.p.add.Y = homeX, homeY, homeX, homeY
	}
	angry := *s.monsters[0].proto
	angry.Aggressive = true
	s.monsters[0].proto = &angry
	s.watchers = append(s.watchers, s.monsters[0])
	hendra.add.X, hendra.char.Level = homeX+x, 1
	return s, hendra
}

func fought(s *Service, target *present) int {
	n := 0
	for _, m := range s.monsters {
		if m.chase != nil && m.chase.target == target {
			n++
		}
	}
	return n
}

// An aggressive monster attacks a player it sees within 2000, and its pack comes with it;
// one 2001 off goes unnoticed.
func TestAnAggressiveMonsterAttacksOnSightWithItsPack(t *testing.T) {
	for x, want := range map[int32]int{2000: 3, 2001: 0} {
		s, hendra := fierce(t, x)
		s.lookOut(noon)
		if got := fought(s, hendra); got != want {
			t.Errorf("at %d: %d attack, want %d", x, got, want)
		}
	}
}

// A monster that only fights back never attacks on sight: only an aggressive kind looks
// out for players.
func TestAPassiveMonsterLetsAPlayerBy(t *testing.T) {
	s, hendra := packed(t, world.Spawn{VNum: 101, X: homeX, Y: homeY, Count: 3})

	s.lookOut(noon)

	if fought(s, hendra) != 0 || len(s.watchers) != 0 {
		t.Fatalf("%d attacked; %d look out", fought(s, hendra), len(s.watchers))
	}
}

// An aggressive monster leaves alone a player more than 10 levels above it, the dead, and
// one who rose less than 5 s ago; at 10 levels above, or 5 s after rising, it attacks.
func TestAnAggressiveMonsterLeavesSomeAlone(t *testing.T) {
	cases := map[string]struct {
		change func(p *present)
		want   int
	}{
		"11 levels above": {func(p *present) { p.char.Level = 12 }, 0},
		"10 levels above": {func(p *present) { p.char.Level = 11 }, 3},
		"dead":            {func(p *present) { p.char.HP = 0 }, 0},
		"just risen":      {func(p *present) { p.char.UnseenUntil = noon.Add(time.Millisecond) }, 0},
		"risen 5 s ago":   {func(p *present) { p.char.UnseenUntil = noon }, 3},
	}
	for name, c := range cases {
		s, hendra := fierce(t, 500)
		c.change(hendra)
		s.lookOut(noon)
		if got := fought(s, hendra); got != c.want {
			t.Errorf("%s: %d attack, want %d", name, got, c.want)
		}
	}
}

// Of two players in sight, an aggressive monster attacks the nearer.
func TestAnAggressiveMonsterAttacksTheNearer(t *testing.T) {
	s, hendra := fierce(t, 1500)
	_, kira := aPlayer(t, s, 1001, 8, "Kira", 0)
	kira.add.X = homeX + 800

	s.lookOut(noon)

	if fought(s, kira) != 3 || fought(s, hendra) != 0 {
		t.Fatalf("Kira %d, Hendra %d", fought(s, kira), fought(s, hendra))
	}
}

// A player entering the world, or rising, is unseen by aggressive monsters for 5 s.
func TestEnteringOrRisingLeavesAPlayerUnseenAWhile(t *testing.T) {
	s, hendra := packed(t, world.Spawn{VNum: 101, X: homeX, Y: homeY, Count: 1})
	hendra.char.HP = 0
	if !s.record.revive(1000, false, 0, 0, noon) || !hendra.char.UnseenUntil.Equal(noon.Add(5*time.Second)) {
		t.Fatalf("risen, unseen until %v", hendra.char.UnseenUntil)
	}

	s.now = func() time.Time { return noon }
	c := conns(t, 1)
	entering := &session{svc: s, conn: c[0], phase: protocol.PhaseLoading,
		playing: &character{vid: 2000, Character: account.Character{Name: "Kira", HP: 1, X: homeX, Y: homeY}}}
	if err := entering.onEnterGame(); err != nil {
		t.Fatal(err)
	}
	if got := s.record.state.all[2000].char.UnseenUntil; !got.Equal(noon.Add(5 * time.Second)) {
		t.Fatalf("entered, unseen until %v", got)
	}
}

// A cursed wolf spawned in the world attacks a player it sees on the next tick: the world
// keeps it on its lookout from the spawn.
func TestACursedWolfAttacksOnTheNextTick(t *testing.T) {
	s, hendra := packed(t, world.Spawn{VNum: 131, X: homeX, Y: homeY, Count: 1})
	hendra.char.Level = 8

	s.tick(noon)

	if fought(s, hendra) != 1 {
		t.Fatal("the cursed wolf let her by")
	}
}

// No monster that attacks on sight stands within 10000 of a kingdom's start: the fields
// by the villages are safe to learn in.
func TestNothingAggressiveStandsNearAVillage(t *testing.T) {
	for _, e := range []account.Empire{account.EmpireShinsoo, account.EmpireChunjo, account.EmpireJinno} {
		x, y, _ := e.StartPosition()
		for _, sp := range world.Spawns() {
			p, _ := mob.Lookup(sp.VNum)
			if d := math.Hypot(float64(sp.X-x), float64(sp.Y-y)); p.Aggressive && d < 10000 {
				t.Errorf("%s: %d at (%d, %d), %.0f from the start", e, sp.VNum, sp.X, sp.Y, d)
			}
		}
	}
}

// A pack walking home from a fight it gave up looks out for no one on the way: a player
// beside its home goes unattacked until the walk would be done, then is attacked.
func TestAPackWalkingHomeLooksOutForNoOne(t *testing.T) {
	s, hendra := fierce(t, 500)
	wolf := s.monsters[0]
	s.chaseOn(wolf, hendra, noon)
	wolf.x = homeX + 4000 // given up 4000 from home, a walk of 10 s at 400 a second
	s.goHome(wolf, noon)

	s.lookOut(noon.Add(10*time.Second - time.Millisecond))
	if fought(s, hendra) != 0 {
		t.Fatal("it turned round on the way home")
	}
	s.lookOut(noon.Add(10 * time.Second))
	if fought(s, hendra) == 0 {
		t.Fatal("home, it let her by")
	}
}

// A packmate exactly 3000 from the one hit still comes.
func TestAPackmateAtTheEdgeOfTheRallyComes(t *testing.T) {
	s, hendra := packed(t, world.Spawn{VNum: 101, X: homeX, Y: homeY, Count: 2})
	dogs := of(s, 101)
	dogs[1].x, dogs[1].y = dogs[0].x+3000, dogs[0].y

	s.engage([]provocation{{dogs[0].p, hendra}}, noon)

	if dogs[1].chase == nil {
		t.Fatal("the dog 3000 off stands by")
	}
}
