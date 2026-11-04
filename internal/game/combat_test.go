package game

import (
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

// A blow of 31 on a wild dog, never grazing: level 1, strength 30.
var (
	brawler  = combat.Attacker{Level: 1, Class: account.Warrior, Stats: account.Stats{Strength: 30, Dexterity: 3}}
	slayer   = combat.Attacker{Level: 50, Class: account.Warrior, Stats: account.Stats{Strength: 90, Dexterity: 60}}
	someRand = rand.New(rand.NewPCG(3, 4)) //nolint:gosec // damage only grazes at random, which these blows never do
)

func dogDefender(t *testing.T) combat.Defender {
	t.Helper()
	p, _ := mob.Lookup(101)
	return combat.Defender{Level: p.Level, Dexterity: p.Dexterity, Vitality: p.Vitality, Armour: p.Armour}
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
	damage := combat.Attack(brawler, dogDefender(t)) - dogDefender(t).Defence()

	if got := s.presence.attack(1000, vid, brawler, someRand, noon); got != blowLanded {
		t.Fatalf("got %v", got)
	}

	want := string(protocol.EncodeDamage(vid, protocol.DamageNormal, int32(damage))) + //nolint:gosec // small
		string(protocol.EncodeTarget(vid, dog.p.fight.hpPercent()))
	if got := read(t, client, damageSize+targetSize); string(got) != want || dog.p.fight.hp != 126-damage {
		t.Fatalf("got %x and %d hit points, want %x and %d", got, dog.p.fight.hp, want, 126-damage)
	}
}

func TestABlowOutOfReachLandsNowhere(t *testing.T) {
	s, dog, client := aDog(t, homeX+attackReach+1)

	if got := s.presence.attack(1000, dog.p.add.VID, brawler, someRand, noon); got != blowRefused {
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

	if got := s.presence.attack(1000, dog.p.add.VID, brawler, someRand, noon); got != blowLanded {
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
	if got := s.presence.attack(1000, npc, slayer, someRand, noon); got != blowRefused {
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

	if got := s.presence.attack(1000, vid, slayer, someRand, noon); got != blowKilled {
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
	s.presence.attack(1000, vid, slayer, someRand, noon)
	read(t, client, damageSize+deadSize)

	if got := s.presence.attack(1000, vid, slayer, someRand, noon); got != blowRefused {
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
	s.presence.attack(1000, old, slayer, someRand, noon)
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
	s.presence.attack(1000, boar, slayer, someRand, noon)

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
