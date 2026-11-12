package game

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/mob"
	"github.com/SatoriAI/MT2-Go/internal/world"
)

// atFortyFive rolls 4500 in 10,000 every time: past a chance of 30%, within one doubled.
var atFortyFive = rand.New(always(uint64(4500*(1<<32)/10000+1) << 32)) //nolint:gosec // a roll fixed for a test

// dogRolling is a dog that leaves a red potion 30 times in 100, a skill book as often, a
// soul stone (steady) as often, and yang as often, rolled at 4500.
func dogRolling(t *testing.T) (*Service, *monster) {
	t.Helper()
	s := worldWith(t, world.Spawn{VNum: 101, X: homeX, Y: homeY, Count: 1})
	m := s.monsters[0]
	rolled := *m.proto
	rolled.Drops = []mob.Drop{{VNum: 27001, Count: 1, Chance: 3000}}
	rolled.Picks = []mob.Pick{{From: []uint32{swirlBook}, Count: 1, Chance: 3000},
		{From: []uint32{item.SoulStone}, Count: 1, Chance: 3000, Steady: true}}
	rolled.Yang = mob.Yang{Min: 10, Max: 10, Chance: 3000}
	m.proto = &rolled
	s.rand = atFortyFive
	return s, m
}

// fallen is the kinds that fell, yang as 1.
func fallen(s *Service) map[uint32]bool {
	kinds := map[uint32]bool{}
	for _, it := range s.record.state.ground {
		kinds[max(it.vnum, min(it.gold, 1))] = true
	}
	return kinds
}

// The roll of 45 misses the potion's 30% for an owner without the gloves.
func TestWithoutTheGlovesADropFallsAtItsOwnChance(t *testing.T) {
	s, m := dogRolling(t)

	s.dropFrom(m, death{owner: owner{7, "Hendra"}}, noon)

	if len(s.record.state.ground) != 0 {
		t.Fatalf("fell %v", fallen(s))
	}
}

// For an owner wearing them, the potion's and the book's chances are 60%, and they fall;
// the steady pick and the yang stay at 30%, and do not.
func TestTheGlovesDoubleAnItemsChanceButNotYangOrTheirOwn(t *testing.T) {
	s, m := dogRolling(t)

	s.dropFrom(m, death{owner: owner{7, "Hendra"}, lucky: true}, noon)

	if got := fallen(s); len(got) != 2 || !got[27001] || !got[swirlBook] {
		t.Fatalf("fell %v, want the potion and the book", got)
	}
}

// A chance doubled stays certain at most: one of 60% doubled falls, as one of 100% does.
func TestADoubledChanceIsCappedAtCertain(t *testing.T) {
	if dropChance(6000, true) != 10000 || dropChance(10000, true) != 10000 || dropChance(3000, false) != 3000 {
		t.Fatal("not capped at 10,000")
	}
}

// The gloves bring luck while their time runs, and no more once it is spent.
func TestOnlyGlovesStillRunningBringLuck(t *testing.T) {
	_, hendra := aPlayer(t, worldWith(t), 1000, 7, "Hendra", 0)
	gloves := item.Item{ID: 5, VNum: item.ThiefGloves, Count: 1, Worn: item.Unique1}

	hendra.timed = &wearing{it: gloves, since: noon}
	running := wearsLuck(hendra)
	hendra.timed.spent = true
	spent := wearsLuck(hendra)
	hendra.timed = nil

	if !running || spent || wearsLuck(hendra) {
		t.Fatalf("running %v, spent %v, none %v", running, spent, wearsLuck(hendra))
	}
}

// The luck is the owner's: Hendra, who took most of the dog's hit points, wears the gloves,
// so the dog Kira finished dies lucky; worn by Kira alone, it does not.
func TestTheLuckIsTheOwnersNotTheKillers(t *testing.T) {
	for _, gloved := range []string{"Hendra", "Kira"} {
		s, m := dogRolling(t)
		_, hendra := aPlayer(t, s, 1000, 7, "Hendra", 20000)
		_, kira := aPlayer(t, s, 1001, 8, "Kira", 40000)
		wearer := map[string]*present{"Hendra": hendra, "Kira": kira}[gloved]
		wearer.timed = &wearing{it: item.Item{ID: 5, VNum: item.ThiefGloves}, since: noon}
		m.p.fight.damageBy = map[*present]int{hendra: 50, kira: 10}

		s.record.held(func(l locked) { l.kill(kira, m.p, noon.Add(time.Second)) })

		if got := s.record.state.news.fallen[0].lucky; got != (gloved == "Hendra") {
			t.Errorf("worn by %s: lucky %v", gloved, got)
		}
	}
}
