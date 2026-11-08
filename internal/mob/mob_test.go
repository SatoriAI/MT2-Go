package mob

import (
	"maps"
	"slices"
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/world"
)

// The client stands a mob's animation still at speed 0, so every kind the world spawns
// needs a proto with real speeds.
func TestEveryKindTheWorldSpawnsIsKnownAndMoves(t *testing.T) {
	for _, s := range world.Spawns() {
		p, ok := Lookup(s.VNum)
		if !ok {
			t.Errorf("%d is spawned but has no proto", s.VNum)
			continue
		}
		if p.VNum != s.VNum || p.MoveSpeed == 0 || p.AttackSpeed == 0 || p.Name == "" {
			t.Errorf("%d: %+v", s.VNum, p)
		}
	}
}

func TestOnlyMonstersWander(t *testing.T) {
	if p, _ := Lookup(101); !p.Wanders() {
		t.Error("a wild dog stands still")
	}
	if p, _ := Lookup(9001); p.Wanders() {
		t.Error("the weapon dealer wanders off")
	}
}

// Every monster and stone the world spawns can be fought: it has hit points and a level;
// NPCs cannot be.
func TestEveryMonsterCanBeFoughtAndNoNPC(t *testing.T) {
	for _, s := range world.Spawns() {
		p, _ := Lookup(s.VNum)
		if p.Fightable() != (p.Kind != NPC) || (p.Fightable() && (p.MaxHP == 0 || p.Level == 0)) {
			t.Errorf("%d: %+v", s.VNum, p)
		}
	}
}

// Every monster can bite: dice that run upwards and a reach. Stones never bite.
func TestEveryMonsterCanBite(t *testing.T) {
	for _, s := range world.Spawns() {
		if p, _ := Lookup(s.VNum); p.Kind == Monster && (p.DamageMin == 0 || p.DamageMax < p.DamageMin || p.AttackRange == 0) {
			t.Errorf("%d: dice %d to %d, reach %d", s.VNum, p.DamageMin, p.DamageMax, p.AttackRange)
		}
	}
}

// Every drop is an item the game has, in a count one item of it holds, with a chance that
// is one: above none and at most certain.
func TestEveryDropIsAnItemTheGameHas(t *testing.T) {
	for vnum, p := range protos {
		for _, d := range p.Drops {
			kind, ok := item.Lookup(d.VNum)
			if !ok || d.Count < 1 || d.Count > kind.Stack || d.Chance < 1 || d.Chance > 10000 {
				t.Errorf("%d drops %+v (%v)", vnum, d, ok)
			}
		}
		for _, pick := range p.Picks {
			for _, v := range pick.From {
				kind, ok := item.Lookup(v)
				if !ok || pick.Count < 1 || pick.Count > kind.Stack || pick.Chance < 1 || pick.Chance > 10000 {
					t.Errorf("%d picks %d of %+v (%v)", vnum, v, pick, ok)
				}
			}
		}
	}
}

// Every monster's yang is a chance in 10,000 of an amount from its least to its most, and
// every one of the first villages leaves some.
func TestEveryMonstersYangIsAChanceOfARange(t *testing.T) {
	for vnum, p := range protos {
		y := p.Yang
		if y.Chance > 10000 || y.Min > y.Max || (p.Kind != NPC) != (y.Chance > 0 && y.Min > 0) {
			t.Errorf("%d leaves %+v", vnum, y)
		}
	}
}

// shopCells and shopRow are the client's shop window: forty cells, five to a row.
const shopCells, shopRow = 40, 5

// Every ware is a kind the game has, in a count one item holds, at a price that fits the
// client's u32, in the shop window with every cell it stands in free of the others; only
// the three traders sell.
func TestEveryWareFitsTheShopWindow(t *testing.T) {
	for vnum, p := range protos {
		if (len(p.Sells) > 0) != (vnum == 9001 || vnum == 9002 || vnum == 9003) {
			t.Errorf("%d sells %v", vnum, p.Sells)
		}
		taken := map[int]bool{}
		for _, w := range p.Sells {
			kind, ok := item.Lookup(w.VNum)
			if !ok || w.Count < 1 || w.Count > kind.Stack || uint64(kind.Price)*uint64(w.Count) > 1<<32-1 {
				t.Errorf("%d sells %+v", vnum, w)
				continue
			}
			for row := range int(kind.Size) {
				cell := int(w.Cell) + row*shopRow
				if cell >= shopCells || taken[cell] {
					t.Errorf("%d: %+v runs off the window or onto another ware at %d", vnum, w, cell)
				}
				taken[cell] = true
			}
		}
	}
}

// Wolves leave a wolf's claw and boars a boar's tooth, each 5 times in 100: the materials
// later upgrades take.
func TestWolvesAndBoarsLeaveTheMaterialsOfUpgrades(t *testing.T) {
	for vnum, material := range map[uint16]uint32{102: 30028, 103: 30028, 104: 30028, 105: 30028, 108: 30004, 109: 30004} {
		p, _ := Lookup(vnum)
		if !slices.Contains(p.Drops, Drop{VNum: material, Count: 1, Chance: 500}) {
			t.Errorf("%d leaves %+v", vnum, p.Drops)
		}
	}
	if dog, _ := Lookup(101); slices.ContainsFunc(dog.Drops, func(d Drop) bool { return d.VNum == 30028 || d.VNum == 30004 }) {
		t.Errorf("a dog leaves %+v", dog.Drops)
	}
}

// The blacksmith is the one NPC who upgrades.
func TestOnlyTheBlacksmithUpgrades(t *testing.T) {
	for vnum, p := range protos {
		if p.Upgrades != (vnum == 20016) || (p.Upgrades && p.Kind != NPC) {
			t.Errorf("%d: %+v", vnum, p)
		}
	}
}

// The armour trader sells the four armours along the top row, on the third row below them
// the four helmets and the shoes, and the shield under those.
func TestTheArmourTraderSellsHelmetsShoesAndTheShieldBelowTheArmours(t *testing.T) {
	trader, _ := Lookup(9002)
	cells := map[uint32]uint8{}
	for _, w := range trader.Sells {
		cells[w.VNum] = w.Cell
	}
	want := map[uint32]uint8{11200: 0, 11400: 1, 11600: 2, 11800: 3, 12200: 10, 12340: 11, 12480: 12, 12620: 13, 15000: 14, 13000: 15}
	if !maps.Equal(cells, want) {
		t.Fatalf("sells %v", cells)
	}
}

// Each stone calls out monsters there are, of no higher level than its own, and comes back
// after a while; only stones call waves, and they never wander.
func TestEveryStoneCallsWavesOfMonstersNoStrongerThanItself(t *testing.T) {
	for vnum, p := range protos {
		if (p.Kind == Stone) != (len(p.Wave) > 0) || (p.Kind == Stone) != (p.Respawn > 0) || (p.Kind == Stone && p.Wanders()) {
			t.Errorf("%d: %+v", vnum, p)
		}
		for _, w := range p.Wave {
			called, ok := Lookup(w)
			if !ok || called.Kind != Monster || called.Level > p.Level {
				t.Errorf("%d calls %d: %+v", vnum, w, called)
			}
		}
	}
}

// A stone always leaves potions and materials, each of one kind, and may leave a piece of
// the first gear; the last also, rarely, one at +3.
func TestAStoneLeavesPotionsMaterialsAndMaybeGear(t *testing.T) {
	cases := map[uint16][]Pick{
		8001: {{potionKinds, 3, 10000}, {materials, 2, 10000}, {firstGear, 1, 1500}},
		8002: {{potionKinds, 4, 10000}, {materials, 3, 10000}, {firstGear, 1, 2500}},
		8003: {{potionKinds, 5, 10000}, {materials, 4, 10000}, {firstGear, 1, 3500}, {firstGearP3, 1, 500}},
	}
	for vnum, want := range cases {
		p, _ := Lookup(vnum)
		if !slices.EqualFunc(p.Picks, want, func(a, b Pick) bool {
			return slices.Equal(a.From, b.From) && a.Count == b.Count && a.Chance == b.Chance
		}) || p.Yang.Chance != 10000 {
			t.Errorf("%d leaves %+v and %+v", vnum, p.Picks, p.Yang)
		}
	}
	for i, v := range firstGearP3 {
		if v != firstGear[i]+3 {
			t.Errorf("%d at +3 is %d", firstGear[i], v)
		}
	}
}

// Every teacher teaches one of the skill groups there are, and each first village's
// warrior teachers the warrior's two.
func TestEveryTeacherTeachesAGroupThereIs(t *testing.T) {
	groups := map[account.SkillGroup]bool{}
	for _, c := range []account.Class{account.Warrior, account.Ninja, account.Sura, account.Shaman} {
		for _, g := range c.SkillGroups() {
			groups[g] = true
		}
	}
	for vnum, p := range protos {
		if p.Teaches != "" && (!groups[p.Teaches] || p.Kind != NPC) {
			t.Errorf("%d teaches %q", vnum, p.Teaches)
		}
	}
	for vnum, want := range map[uint16]account.SkillGroup{20300: "body", 20301: "mental", 20320: "body", 20321: "mental", 20340: "body", 20341: "mental"} {
		if p, _ := Lookup(vnum); p.Teaches != want {
			t.Errorf("%d teaches %q, want %q", vnum, p.Teaches, want)
		}
	}
}
