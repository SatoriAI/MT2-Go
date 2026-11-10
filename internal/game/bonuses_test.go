package game

import (
	"bytes"
	"encoding/binary"
	"math/rand/v2"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/combat"
	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/mob"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/skill"
)

// seeded is a generator that rolls the same every time it is made.
func seeded() *rand.Rand { return rand.New(rand.NewPCG(3, 3)) } //nolint:gosec // a test's fixed seed

// blowWith is what one of the sura's blows on the dog takes with totals worn, from the same
// dice every time, and the flags it was told with; the dog is made too tough to fall.
func blowWith(t *testing.T, totals account.Totals, animal bool) (int, uint8) {
	t.Helper()
	s, dog, client, c := suraHits(t, 100)
	c.Totals, dog.p.fight.animal = totals, animal
	dog.p.fight.hp, dog.p.fight.maxHP = 1_000_000, 1_000_000
	s.record.held(func(l locked) { l.land(l.all[1000], dog.p, seeded(), noon) })
	d := protocol.NewReader(read(t, client, damageSize)[1:])
	_, flag, _ := d.U32(), d.U8(), d.I32()
	return 1_000_000 - dog.p.fight.hp, flag
}

// A piercing blow is drawn with the piercing effect on the sura and a critical one with the
// critical effect, each for her and a player who sees her; a plain blow with neither.
func TestPiercingAndCriticalBlowsAreDrawnOnTheAttacker(t *testing.T) {
	for name, c := range map[string]struct {
		totals account.Totals
		want   []protocol.Effect
	}{
		"plain":    {account.Totals{}, nil},
		"piercing": {account.Totals{Pierce: 100}, []protocol.Effect{protocol.EffectPiercing}},
		"critical": {account.Totals{Critical: 100}, []protocol.Effect{protocol.EffectCritical}},
		"both":     {account.Totals{Pierce: 100, Critical: 100}, []protocol.Effect{protocol.EffectPiercing, protocol.EffectCritical}},
	} {
		s, dog, client, sura := suraHits(t, 100)
		watcher, _ := aPlayer(t, s, 1001, 8, "Kira", 200)
		drain(t, client)
		drain(t, watcher)
		sura.Totals, dog.p.fight.hp = c.totals, 1_000_000
		s.record.held(func(l locked) { l.land(l.all[1000], dog.p, seeded(), noon) })

		var drawn [][]byte
		for _, e := range c.want {
			drawn = append(drawn, protocol.EncodeSpecialEffect(e, 1000))
		}
		if got := packets(t, drain(t, client))[1:]; !slices.EqualFunc(got, drawn, bytes.Equal) {
			t.Errorf("%s: after the damage the sura is sent %x, want %x", name, got, drawn)
		}
		if got := packets(t, drain(t, watcher)); !slices.EqualFunc(got, drawn, bytes.Equal) {
			t.Errorf("%s: Kira is sent %x, want %x", name, got, drawn)
		}
	}
}

// Average damage raises a blow by its percent, and strength against animals raises one on
// an animal, but not on anything else.
func TestAverageDamageAndStrengthAgainstAnimalsRaiseABlow(t *testing.T) {
	plain, flag := blowWith(t, account.Totals{}, true)
	if flag != protocol.DamageNormal {
		t.Fatalf("a plain blow told as %d", flag)
	}
	for name, c := range map[string]struct {
		totals account.Totals
		animal bool
		want   int
	}{
		"average 20":           {account.Totals{Average: 20}, true, plain * 120 / 100},
		"animals 10":           {account.Totals{Animals: 10}, true, plain * 110 / 100},
		"animals on no animal": {account.Totals{Animals: 10}, false, plain},
		"both":                 {account.Totals{Average: 20, Animals: 10}, true, plain * 120 * 110 / 10000},
		"average taken off":    {account.Totals{Average: -15}, true, plain * 85 / 100},
	} {
		if got, _ := blowWith(t, c.totals, c.animal); got != c.want {
			t.Errorf("%s: took %d, want %d of %d", name, got, c.want, plain)
		}
	}
}

// A piercing blow goes through the dog's defence, told with the client's piercing flag; a
// critical one from a bonus is twice the blow, told as critical, and one both is both.
func TestPiercingAndCriticalBlowsFromBonuses(t *testing.T) {
	plain, _ := blowWith(t, account.Totals{}, true)
	_, _, _, c := suraHits(t, 100)
	r := seeded()
	r.IntN(100) // the piercing roll
	through := combat.Damage(attackerOf(c), combat.Defender{}, r)

	if got, flag := blowWith(t, account.Totals{Pierce: 100}, true); got != through || flag != protocol.DamagePiercing {
		t.Errorf("piercing: took %d told %d, want %d through its defence", got, flag, through)
	}
	if got, flag := blowWith(t, account.Totals{Critical: 100}, true); got != 2*plain || flag != protocol.DamageCritical {
		t.Errorf("critical: took %d told %d, want %d", got, flag, 2*plain)
	}
	if got, flag := blowWith(t, account.Totals{Pierce: 100, Critical: 100}, true); got != 2*through ||
		flag != protocol.DamagePiercing|protocol.DamageCritical {
		t.Errorf("both: took %d told %d, want %d", got, flag, 2*through)
	}
}

// Skill damage raises a skill's blow, and average damage leaves it as it is.
func TestSkillDamageRaisesASkillAndAverageDamageDoesNot(t *testing.T) {
	skilled := func(totals account.Totals) int {
		s, dog, _, c := suraHits(t, 100)
		c.Totals = totals
		dog.p.fight.hp, dog.p.fight.maxHP = 1_000_000, 1_000_000
		proto, _ := skill.Lookup(skill.DragonSwirl)
		s.record.held(func(l locked) { l.splash(l.all[1000], dog.p, proto, 300, 1, noon, seeded()) })
		return 1_000_000 - dog.p.fight.hp
	}
	plain := skilled(account.Totals{})
	if got := skilled(account.Totals{Average: 30}); got != plain {
		t.Errorf("average 30: took %d, want %d", got, plain)
	}
	if got := skilled(account.Totals{Skill: 20}); got != plain*120/100 {
		t.Errorf("skill 20: took %d, want %d", got, plain*120/100)
	}
}

// Resistance takes its percent off a bite.
func TestResistanceTakesOffABite(t *testing.T) {
	bitten := func(resist int) uint32 {
		s, dog, _, c := suraHits(t, 100)
		c.Totals.Resist, c.HP = resist, 1_000_000
		dog.chase = &chase{} // as the loop sets it on her
		s.record.held(func(l locked) { l.bite(dog, l.all[1000], noon, 0, seeded()) })
		return 1_000_000 - c.HP
	}
	plain := bitten(0)
	if got := bitten(15); got != plain*85/100 {
		t.Fatalf("resist 15: took %d, want %d of %d", got, plain*85/100, plain)
	}
}

// What is worn sums its bonuses into the totals, capped; hit points' maximum rises with
// them and falls when they come off, the hit points cut to it, and she is told both.
func TestWhatIsWornSetsTheTotalsAndHitPoints(t *testing.T) {
	inv := item.Inventory{
		{ID: 1, VNum: 290, Count: 1, Worn: item.Weapon, Bonuses: item.Bonuses{{Kind: account.BonusCritical, Value: 8}, {Kind: account.BonusAverage, Value: 30}, {Kind: account.BonusSkill, Value: -15}}},
		{ID: 2, VNum: 15100, Count: 1, Worn: item.Feet, Bonuses: item.Bonuses{{Kind: account.BonusCritical, Value: 8}, {Kind: account.BonusMaxHP, Value: 400}}},
		{ID: 3, VNum: 12220, Count: 1, Worn: item.Head, Bonuses: item.Bonuses{{Kind: account.BonusAnimals, Value: 10}}},
		{ID: 4, VNum: 11240, Count: 1, Cell: 0, Bonuses: item.Bonuses{{Kind: account.BonusMaxHP, Value: 400}}}, // in the bag
	}
	if got := outfitOf(inv).totals; got != (account.Totals{MaxHP: 400, Critical: 16, Animals: 10, Average: 30, Skill: -15}) {
		t.Fatalf("totals %+v", got)
	}

	s, _, client, c := suraHits(t, 100)
	points, _ := c.Points()
	s.record.dress(1000, outfit{totals: account.Totals{MaxHP: 400}})
	if got := packets(t, drain(t, client)); !holds(got, pointTold(1000, protocol.PointMaxHP, 0, points.MaxHP+400)) {
		t.Fatalf("told %x, want max HP %d", got, points.MaxHP+400)
	}
	c.HP = points.MaxHP + 400

	s.record.dress(1000, outfit{})
	if got := packets(t, drain(t, client)); c.HP != points.MaxHP ||
		!holds(got, pointTold(1000, protocol.PointMaxHP, 0, points.MaxHP), pointTold(1000, protocol.PointHP, 0, points.MaxHP)) {
		t.Fatalf("HP %d, told %x, want %d", c.HP, got, points.MaxHP)
	}
}

// A piece of gear picked up keeps its bonuses, kept and shown in its slots.
func TestGearPickedUpKeepsItsBonuses(t *testing.T) {
	s, kept, client := pickingUp(t, 5)
	bonuses := item.Bonuses{{Kind: account.BonusMaxHP, Value: 180}}
	s.svc.record.drop(s.svc.record.state.all[1000], []*groundItem{{vid: 5001, x: homeX, y: homeY, vnum: 11240, count: 1,
		bonuses: bonuses, owner: owner{7, "Hendra"}, dropped: noon}})
	drain(t, client)

	if err := s.onItemPickup(binary.LittleEndian.AppendUint32(nil, 5001)); err != nil {
		t.Fatal(err)
	}

	got := packets(t, drain(t, client))
	if len(kept.added) != 1 || !reflect.DeepEqual(kept.added[0].Bonuses, bonuses) ||
		!holds(got, itemSet(item.Item{ID: 101, VNum: 11240, Count: 1, Cell: 1, Bonuses: bonuses})) {
		t.Fatalf("kept %+v, told %x", kept.added, got)
	}
}

// Gear a player drops lies with its bonuses.
func TestGearDroppedLiesWithItsBonuses(t *testing.T) {
	s, _, _, _ := dropping(t)
	bonuses := item.Bonuses{{Kind: account.BonusResist, Value: 3}}
	s.inventory = append(s.inventory, item.Item{ID: 3, VNum: 11240, Count: 1, Cell: 5, Bonuses: bonuses})

	if err := s.onItemDrop(dropped(1, 5, 0, 1)); err != nil {
		t.Fatal(err)
	}

	if it := theOneLying(t, s); !reflect.DeepEqual(it.bonuses, bonuses) {
		t.Fatalf("lies with %+v", it.bonuses)
	}
}

// A stone's gear rolls bonuses as it falls, and a monster's does not.
func TestAStonesGearRollsBonusesAndAMonstersDoesNot(t *testing.T) {
	s, stone, _, _ := aStone(t, 100)
	dog := *stone
	dogProto := *stone.proto
	dogProto.Kind = "monster"
	dog.proto = &dogProto
	s.rand = seeded()

	for range 200 {
		it := &groundItem{vnum: 11240}
		if s.rollBonuses(&dog, it); len(it.bonuses) != 0 {
			t.Fatalf("a monster's armour rolled %+v", it.bonuses)
		}
		it = &groundItem{vnum: 12220}
		if s.rollBonuses(stone, it); len(it.bonuses) == 0 {
			t.Fatal("a stone's helmet rolled none")
		}
	}
}

// A remarkable roll is told to everyone on the map with its finder's name, once the gear
// lies on the ground to be seen.
func TestARemarkableDropIsToldOnceItLies(t *testing.T) {
	s, stone, _, client := aStone(t, 100)
	helmets := *stone.proto
	helmets.Drops, helmets.Picks, helmets.Yang = []mob.Drop{{VNum: 12220, Count: 1, Chance: 10000}}, nil, mob.Yang{}
	stone.proto = &helmets
	s.rand = seeded()

	remarkable := false
	for i := 0; i < 200 && !remarkable; i++ {
		s.dropFrom(stone, death{owner: owner{7, "Hendra"}}, noon)
		for _, it := range s.record.state.ground {
			remarkable = remarkable || it.bonuses.Remarkable()
		}
	}
	if !remarkable {
		t.Fatal("200 helmets, none remarkable")
	}
	got := packets(t, drain(t, client))
	told := slices.IndexFunc(got, func(p []byte) bool {
		return bytes.Equal(p, notice("Hendra zdobywa Żelazny Hełm z niezwykłymi bonusami!"))
	})
	laid := -1
	for i, p := range got {
		if p[0] == byte(protocol.ServerGroundAdd) {
			laid = i
		}
	}
	if told < 0 || told < laid {
		t.Fatalf("told at %d, the helmet laid at %d: %x", told, laid, got)
	}
}

// /bonusy lists the totals of what she wears in the client's words, or says there are none.
func TestBonusyListsWhatSheWears(t *testing.T) {
	got := bonusLines(account.Totals{Average: 20, Skill: -10, MaxHP: 400})
	want := []string{"Bonusy tego, co nosisz:", "Średnie Obrażenia: +20%", "Obrażenie Umiejętności: -10%", "Maks. PŻ: +400"}
	if len(got) != len(want) {
		t.Fatalf("%d lines, want %d", len(got), len(want))
	}
	for i, line := range want {
		if string(got[i]) != string(infoLine(line)) {
			t.Errorf("line %d: %x, want %q", i, got[i], line)
		}
	}
	if none := bonusLines(account.Totals{}); len(none) != 1 || !strings.Contains(string(none[0]), "bonusami") {
		t.Errorf("none: %q", none)
	}
}

// Each bonus reaches the client as its own APPLY_ type, so its tooltip line is the right
// one: Średnie 72, Umiejętności 71, and the rest.
func TestEachBonusIsSentAsTheClientsType(t *testing.T) {
	got := bonusSlots(item.Bonuses{{Kind: account.BonusAverage, Value: 20}, {Kind: account.BonusSkill, Value: -10},
		{Kind: account.BonusMaxHP, Value: 400}, {Kind: account.BonusAttackSpeed, Value: 3}, {Kind: account.BonusCritical, Value: 5},
		{Kind: account.BonusPierce, Value: 2}, {Kind: account.BonusAnimals, Value: 8}})
	want := protocol.Bonuses{{Type: 72, Value: 20}, {Type: 71, Value: -10}, {Type: 1, Value: 400}, {Type: 7, Value: 3},
		{Type: 15, Value: 5}, {Type: 16, Value: 2}, {Type: 18, Value: 8}}
	if got != want {
		t.Fatalf("slots %v, want %v", got, want)
	}
	if got := bonusSlots(item.Bonuses{{Kind: account.BonusResist, Value: 4}}); got[0] != (protocol.Bonus{Type: 74, Value: 4}) {
		t.Fatalf("resistance %v", got[0])
	}
}

// Gear offered in a trade shows its bonuses to both sides.
func TestGearOfferedShowsItsBonusesToBoth(t *testing.T) {
	_, hendra, kira := started(t)
	hendra.s.inventory = append(hendra.s.inventory, item.Item{ID: 5, VNum: 11240, Count: 1, Cell: 20,
		Bonuses: item.Bonuses{{Kind: account.BonusMaxHP, Value: 400}}})

	hendra.do(t, exchanging(protocol.ExchangeAdd, 0, 0, 20))

	armour := protocol.ExchangeNews{Sub: protocol.ExchangeAdded, Mine: true, Arg: 11240, Shown: 0, Count: 1,
		Bonuses: protocol.Bonuses{{Type: 1, Value: 400}}}
	if got := hendra.told(t); !holds(got, exNews(armour)) {
		t.Fatalf("Hendra told %x", got)
	}
	armour.Mine = false
	if got := kira.told(t); !holds(got, exNews(armour)) {
		t.Fatalf("Kira told %x", got)
	}
}

// A bonus raises a blow that hurts and never makes a harmless one hurt, and a total of
// none leaves the damage as rolled; average and animals are taken in one sum, so small
// bonuses are not rounded away.
func TestRaisedDamage(t *testing.T) {
	animal, other := &present{fight: &fight{animal: true}}, &present{fight: &fight{}}
	for _, c := range []struct {
		damage, percent int
		v               *present
		animals, want   int
	}{
		{0, 30, animal, 25, 0},
		{1, -15, animal, 0, 1},
		{7, 0, animal, 0, 7},
		{7, 0, other, 25, 7},
		{10, 5, animal, 5, 11},
	} {
		if got := raised(c.damage, c.percent, c.v, c.animals); got != c.want {
			t.Errorf("%d raised by %d, animals %d: %d, want %d", c.damage, c.percent, c.animals, got, c.want)
		}
	}
}

// An upgrade at the blacksmith keeps the sword's bonuses, held and told.
func TestAnUpgradeKeepsTheBonuses(t *testing.T) {
	bonuses := item.Bonuses{{Kind: account.BonusCritical, Value: 3}}
	s, kept, client, smith := atTheSmith(t, 500, item.Inventory{{ID: 1, VNum: 10, Count: 1, Cell: 3, Bonuses: bonuses}}, anyRoll)
	giveAndRead(t, s, client, smith)

	if err := s.onRefine(t.Context(), refined(3, protocol.RefineNoQuestion)); err != nil {
		t.Fatal(err)
	}

	upgraded := item.Item{ID: 1, VNum: 11, Count: 1, Cell: 3, Bonuses: bonuses}
	if got := read(t, client, itemSetSize); string(got) != string(itemSet(upgraded)) {
		t.Fatalf("told %x", got)
	}
	if it, _ := s.inventory.ByID(1); !reflect.DeepEqual(kept.upgraded, upgraded) || !reflect.DeepEqual(it, upgraded) {
		t.Fatalf("kept %+v, holds %+v", kept.upgraded, it)
	}
}

// A sword traded goes with its bonuses: kept so and shown so to the side it comes to.
func TestATradedItemKeepsItsBonuses(t *testing.T) {
	bonuses := item.Bonuses{{Kind: account.BonusAnimals, Value: 6}}
	store, hendra, kira := traders(t)
	hendra.s.inventory[0].Bonuses = bonuses
	hendra.do(t, exchanging(protocol.ExchangeStart, 1001, 0, 0))
	hendra.do(t, exchanging(protocol.ExchangeAdd, 0, 0, 0))
	kira.at(acceptAfter)
	kira.do(t, exchanging(protocol.ExchangeAccept, 0, 0, 0))
	drain(t, kira.client)
	hendra.do(t, exchanging(protocol.ExchangeAccept, 0, 0, 0))

	got := item.Item{ID: 1, VNum: 10, Count: 1, Cell: 0, Bonuses: bonuses}
	if !reflect.DeepEqual(store.kept[0].Items, []item.Item{got}) {
		t.Fatalf("kept %+v", store.kept)
	}
	if told := kira.told(t); !holds(told, itemSet(got)) {
		t.Fatalf("Kira told %x", told)
	}
}
