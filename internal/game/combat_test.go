package game

import (
	"math"
	"math/rand/v2"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/combat"
	"github.com/SatoriAI/MT2-Go/internal/mob"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/world"
)

const (
	targetSize = 6
	damageSize = 10
	deadSize   = 5
	deleteSize = 5
)

// A brawler takes about 56 off a wild dog a blow; a slayer kills it in one.
var (
	brawler  = combat.Attacker{Level: 1, Class: account.Warrior, Stats: account.Stats{Strength: 30, Dexterity: 3}}
	slayer   = combat.Attacker{Level: 50, Class: account.Warrior, Stats: account.Stats{Strength: 90, Dexterity: 60}}
	someRand = rand.New(rand.NewPCG(3, 4)) //nolint:gosec // a fixed seed for each blow's spread
)

// hit lands a blow of player 1000 on the victim, as a character with a's level, class and
// stats and all its hit points, which the player keeps from their first blow on.
func (s *Service) hit(victim uint32, a combat.Attacker, gap time.Duration, now time.Time) blow {
	if p := s.presence.all[1000]; p.char == nil {
		c := &account.Character{Level: a.Level, Class: a.Class, Stats: a.Stats}
		points, _ := c.Points()
		c.HP, c.SP = points.MaxHP, points.MaxSP // whole, so nothing heals
		p.char = c
	}
	return s.presence.attack(1000, victim, gap, someRand, now)
}

func dogDefender(t *testing.T) combat.Defender {
	t.Helper()
	p, _ := mob.Lookup(101)
	return combat.Defender{Level: p.Level, Vitality: p.Vitality, Armour: uint16(p.Armour)}
}

// aDog is a world with one wild dog at home, and the player at playerX east of it, with
// what entering showed them read.
func aDog(t *testing.T, playerX int32) (*Service, *monster, net.Conn) {
	t.Helper()
	s := worldWith(t, world.Spawn{VNum: 101, X: homeX, Y: homeY, Count: 1})
	shown := addSize
	if playerX-homeX > viewRange {
		shown = 0
	}
	return s, s.monsters[0], s.playerAt(t, playerX, shown)
}

func TestTargetingAMonsterShowsItsHitPoints(t *testing.T) {
	s, dog, client := aDog(t, homeX+100)

	s.presence.target(1000, dog.p.add.VID)

	if got, want := read(t, client, targetSize), protocol.EncodeTarget(dog.p.add.VID, 100); string(got) != string(want) {
		t.Fatalf("got %x, want %x", got, want)
	}
}

// A blow takes its damage off the hit points: the attacker is told the damage, and,
// having picked the dog, how much it has left.
func TestABlowTakesHitPointsAndTellsTheAttacker(t *testing.T) {
	s, dog, client := aDog(t, homeX+100)
	vid := dog.p.add.VID
	s.presence.target(1000, vid)
	read(t, client, targetSize)
	mean := combat.Mean(brawler, dogDefender(t))

	if got := s.hit(vid, brawler, 0, noon); got != blowLanded {
		t.Fatalf("got %v", got)
	}

	got := read(t, client, damageSize+targetSize)
	damage := int(protocol.NewReader(got[6:]).I32())
	if float64(damage) < math.Round(mean*0.9) || float64(damage) > math.Round(mean*1.1) {
		t.Fatalf("a blow took %d, want within a tenth of %v", damage, mean)
	}
	want := string(protocol.EncodeDamage(vid, protocol.DamageNormal, int32(damage))) + //nolint:gosec // small
		string(protocol.EncodeTarget(vid, dog.p.fight.hpPercent()))
	if string(got) != want || dog.p.fight.hp != 126-damage {
		t.Fatalf("got %x and %d hit points, want %x and %d", got, dog.p.fight.hp, want, 126-damage)
	}
}

func TestABlowOutOfReachLandsNowhere(t *testing.T) {
	s, dog, client := aDog(t, homeX+attackReach+1)

	if got := s.hit(dog.p.add.VID, brawler, 0, noon); got != blowRefused {
		t.Fatalf("got %v", got)
	}
	expectNothing(t, client)
	if dog.p.fight.hp != 126 {
		t.Fatalf("the dog has %d hit points", dog.p.fight.hp)
	}
}

// The edge of reach is still in reach.
func TestABlowAtTheEdgeOfReachLands(t *testing.T) {
	s, dog, _ := aDog(t, homeX+attackReach)

	if got := s.hit(dog.p.add.VID, brawler, 0, noon); got != blowLanded {
		t.Fatalf("got %v", got)
	}
}

func TestAnNPCCannotBeFoughtOrTargeted(t *testing.T) {
	s := worldWith(t, world.Spawn{VNum: 9001, X: homeX, Y: homeY, Count: 1})
	client := s.playerAt(t, homeX+100, addSize+54)
	var npc uint32
	for vid, p := range s.presence.all {
		if p.out == nil {
			npc = vid
		}
	}

	s.presence.target(1000, npc)
	if got := s.hit(npc, slayer, 0, noon); got != blowRefused {
		t.Fatalf("got %v", got)
	}
	expectNothing(t, client)
}

// A killing blow: the attacker is told the damage, its last hit points and its death;
// everyone else who sees it its death; and nobody targets it any more.
func TestAKillTellsEveryoneWhoSeesItAndEndsTheirTargets(t *testing.T) {
	s, dog, attacker := aDog(t, homeX+100)
	vid := dog.p.add.VID
	conn, onlooker := watcher(t)
	s.presence.enter(&present{out: conn, add: protocol.CharacterAdd{VID: 2000, X: homeX - 100, Y: homeY}})
	read(t, onlooker, addSize+addSize) // the dog and the attacker
	s.presence.target(1000, vid)
	read(t, attacker, addSize+targetSize) // the onlooker arriving, then the dog's hit points

	if got := s.hit(vid, slayer, 0, noon); got != blowKilled {
		t.Fatalf("got %v", got)
	}

	want := string(protocol.EncodeDamage(vid, protocol.DamageNormal, 126)) + string(protocol.EncodeTarget(vid, 0)) +
		string(protocol.EncodeDead(vid))
	if got := read(t, attacker, damageSize+targetSize+deadSize); string(got) != want {
		t.Fatalf("the attacker got %x, want %x", got, want)
	}
	if got := read(t, onlooker, deadSize); string(got) != string(protocol.EncodeDead(vid)) {
		t.Fatalf("the onlooker got %x", got)
	}
	if s.presence.all[1000].target != nil || len(dog.p.fight.targetedBy) != 0 {
		t.Fatal("the attacker still targets the body")
	}
}

// A body cannot be hit, picked or walked.
func TestADeadMonsterCannotBeHitTargetedOrMoved(t *testing.T) {
	s, dog, client := aDog(t, homeX+100)
	vid := dog.p.add.VID
	s.hit(vid, slayer, 0, noon)
	read(t, client, damageSize+deadSize)

	if got := s.hit(vid, slayer, 0, noon); got != blowRefused {
		t.Fatalf("hit a body: %v", got)
	}
	s.presence.target(1000, vid)
	s.tick(noon.Add(time.Second)) // the dog was due a stroll, and is dead
	s.presence.move(vid, protocol.Move{X: homeX + 500, Y: homeY})

	expectNothing(t, client)
}

// The body lies two seconds, then is taken away; a new monster stands in the spawn's area
// twenty seconds after the death, with a new vid and all its hit points.
func TestABodyIsTakenAwayAndAMonsterReturns(t *testing.T) {
	s, dog, client := aDog(t, homeX+100)
	old := dog.p.add.VID
	s.hit(old, slayer, 0, noon)
	read(t, client, damageSize+deadSize)
	for _, m := range s.monsters {
		m.nextStroll = noon.Add(time.Hour) // nothing but the death in these ticks
	}

	s.tick(noon.Add(corpseTime - time.Millisecond))
	expectNothing(t, client)
	s.tick(noon.Add(corpseTime))
	if got := read(t, client, deleteSize); string(got) != string(protocol.EncodeCharacterDelete(old)) {
		t.Fatalf("got %x, want the body deleted", got)
	}
	s.tick(noon.Add(respawnAfter - time.Millisecond))
	expectNothing(t, client)
	s.tick(noon.Add(respawnAfter))

	add := protocol.NewReader(read(t, client, addSize)[1:])
	if vid := add.U32(); vid == old || vid != dog.p.add.VID || dog.p.fight.hp != 126 || dog.p.dead() {
		t.Fatalf("returned as %d with %d hit points; the dead one was %d", vid, dog.p.fight.hp, old)
	}
}

// Walking away from a monster ends targeting it: a blow the player can no longer see is
// no business of theirs.
func TestWalkingOutOfViewEndsTheTarget(t *testing.T) {
	s, dog, client := aDog(t, homeX+100)
	s.presence.target(1000, dog.p.add.VID)
	read(t, client, targetSize)

	s.presence.move(1000, protocol.Move{X: homeX + 9000, Y: homeY})

	if s.presence.all[1000].target != nil || len(dog.p.fight.targetedBy) != 0 {
		t.Fatal("still targeting what they no longer see")
	}
}

// A boar is "Dzik", and so may a player be: taking the boar's body away keeps the player
// findable by name.
func TestTakingAwayAMobKeepsAPlayerOfTheSameName(t *testing.T) {
	s := worldWith(t, world.Spawn{VNum: 108, X: homeX, Y: homeY, Count: 1})
	conn, _ := watcher(t)
	s.presence.enter(&present{out: conn, info: protocol.CharacterInfo{Name: "dzik"},
		add: protocol.CharacterAdd{VID: 1000, X: homeX + 100, Y: homeY}})
	boar := s.monsters[0].p.add.VID
	s.hit(boar, slayer, 0, noon)

	s.presence.leave(boar)

	if _, ok := s.presence.byName[strings.ToLower(textToClient("Dzik"))]; !ok {
		t.Fatal("the player named dzik is gone from the names")
	}
}

// open-mt2's gap between blows on one victim: 15000 ms over the attack speed, at least
// 80 ms.
func TestTheGapBetweenBlowsFollowsAttackSpeed(t *testing.T) {
	cases := map[uint8]time.Duration{100: 150 * time.Millisecond, 150: 100 * time.Millisecond, 250: 80 * time.Millisecond}
	for speed, want := range cases {
		if got := blowGap(speed); got != want {
			t.Errorf("speed %d: got %v, want %v", speed, got, want)
		}
	}
}

// Picking another monster ends the first pick: its bar no longer comes.
func TestPickingAnotherMonsterEndsTheFirstPick(t *testing.T) {
	s := worldWith(t, world.Spawn{VNum: 101, X: homeX, Y: homeY, Count: 2})
	client := s.playerAt(t, homeX+100, 2*addSize)
	first, second := s.monsters[0].p, s.monsters[1].p
	s.presence.target(1000, first.add.VID)
	s.presence.target(1000, second.add.VID)
	read(t, client, 2*targetSize)

	if _, still := first.fight.targetedBy[s.presence.all[1000]]; still {
		t.Fatal("the first monster still counts the player among its pickers")
	}
}

// The victim keeps its last attacker too: a player switching between two monsters cannot
// land blows on either sooner than the gap allows.
func TestSwitchingVictimsDoesNotGetRoundTheBlowGap(t *testing.T) {
	s := worldWith(t, world.Spawn{VNum: 101, X: homeX, Y: homeY, Count: 2})
	s.playerAt(t, homeX+100, 2*addSize)
	a, b := s.monsters[0].p.add.VID, s.monsters[1].p.add.VID
	const gap = 100 * time.Millisecond

	s.hit(a, brawler, gap, noon)
	s.hit(b, brawler, gap, noon.Add(time.Millisecond))

	if got := s.hit(a, brawler, gap, noon.Add(2*time.Millisecond)); got != blowTooSoon {
		t.Fatalf("a third blow, back on the first dog, got %v", got)
	}
	if got := s.hit(a, brawler, gap, noon.Add(gap)); got != blowLanded {
		t.Fatalf("a blow after the gap got %v", got)
	}
}

// A player arriving while a body lies sees it lying, not standing.
func TestABodyIsShownLyingToSomeoneArriving(t *testing.T) {
	s, dog, _ := aDog(t, homeX+100)
	s.hit(dog.p.add.VID, slayer, 0, noon)
	conn, newcomer := watcher(t)

	s.presence.enter(&present{out: conn, add: protocol.CharacterAdd{VID: 2000, X: homeX - 100, Y: homeY}})

	shown := read(t, newcomer, addSize+addSize) // the dog and the attacker, in either order
	lying := shown[:addSize]
	if protocol.NewReader(lying[1:]).U32() != dog.p.add.VID {
		lying = shown[addSize:]
	}
	if state := lying[26]; state != 1 { // after header, vid, angle, x, y, z, type, race and speeds
		t.Fatalf("the body's state is %d, want 1, dead", state)
	}
}
