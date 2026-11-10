package game

import (
	"bytes"
	"math/rand/v2"
	"net"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/skill"
)

// supporting is Rugor of shamanising with every buff and the heal at their first level, and Kira,
// vid 1001, a warrior of her kingdom 300 from her, whose client has read what entering
// showed.
func supporting(t *testing.T) (*session, net.Conn, *present, net.Conn) {
	t.Helper()
	s, client := shamanising(t, 1)
	c := s.svc.presence.all[1000].char
	for _, sk := range []account.Skill{skill.Blessing, skill.Reflection, skill.DragonsHelp, skill.Heal, skill.Swiftness, skill.AttackUp} {
		c.Skills[sk] = 1
	}
	kiraClient, kira := aPlayer(t, s.svc, 1001, 8, "Kira", 400)
	kira.char.Class, kira.char.Level, kira.char.HP = account.Warrior, 10, 1
	drain(t, client)
	drain(t, kiraClient)
	return s, client, kira, kiraClient
}

// Blessing on Kira, picked within its 800, runs on her at the shaman's level of it and
// INT, shown on her, and not on the shaman.
func TestBlessingLandsOnThePlayerPicked(t *testing.T) {
	s, _, kira, kiraClient := supporting(t)

	if err := s.onUseSkill(usedOn(skill.Blessing, 1001)); err != nil {
		t.Fatal(err)
	}

	if _, running := kira.char.Buffs[skill.Blessing]; !running || kira.char.BuffBy[skill.Blessing] != (account.BuffCaster{Level: 1, Intelligence: 20}) {
		t.Fatalf("Kira's buffs %v by %v", kira.char.Buffs, kira.char.BuffBy)
	}
	if _, running := s.svc.presence.all[1000].char.Buffs[skill.Blessing]; running {
		t.Fatal("it ran on the shaman too")
	}
	if got := drain(t, kiraClient); !bytes.Contains(got, protocol.EncodeCharacterUpdate(kira.info, kira.add, 1<<22)) {
		t.Fatalf("Kira got %x, want the blessing shown", got)
	}
}

// With nothing picked, a player of another kingdom picked, or one beyond its range, the
// buff lands on the shaman herself.
func TestABuffLandsOnTheCasterWithNoAllyInReach(t *testing.T) {
	for name, set := range map[string]func(s *session, kira *present) uint32{
		"nothing picked":   func(*session, *present) uint32 { return 0 },
		"another kingdom":  func(_ *session, kira *present) uint32 { kira.info.Empire = 9; return 1001 },
		"beyond its range": func(s *session, kira *present) uint32 { kira.add.X = homeX + 2000; return 1001 },
	} {
		s, _, kira, _ := supporting(t)
		if err := s.onUseSkill(usedOn(skill.Blessing, set(s, kira))); err != nil {
			t.Fatal(err)
		}
		if _, running := s.svc.presence.all[1000].char.Buffs[skill.Blessing]; !running {
			t.Errorf("%s: not on the shaman", name)
		}
		if _, running := kira.char.Buffs[skill.Blessing]; running {
			t.Errorf("%s: on Kira", name)
		}
	}
}

// bittenWith is the damage of the dog's first bite on a sura running the buff cast by,
// none for none, with the same roll each time.
func bittenWith(t *testing.T, sk account.Skill, by *account.BuffCaster) int32 {
	t.Helper()
	s, _, client, c := suraHits(t, 100)
	if by != nil {
		c.Buffs = map[account.Skill]time.Time{sk: noon.Add(time.Minute)}
		c.BuffBy = map[account.Skill]account.BuffCaster{sk: *by}
	}
	s.rand = rand.New(rand.NewPCG(9, 9)) //nolint:gosec // the same roll for each
	c.HP = 10000
	s.tick(noon)
	got := drain(t, client)
	d := protocol.NewReader(got[moveSize+1:])
	_, _, damage := d.U32(), d.U8(), d.I32()
	return damage
}

// Blessing cuts every bite by its percent, (0.3·iq + 5)(2k + 0.5)/(k + 1.5), after armour:
// 24 in a hundred from a shaman of INT 90 at level 20.
func TestBlessingCutsEveryBite(t *testing.T) {
	plain, blessed := bittenWith(t, "", nil), bittenWith(t, skill.Blessing, &account.BuffCaster{Level: 20, Intelligence: 90})
	if blessed != plain*76/100 || plain == 0 {
		t.Fatalf("bitten for %d blessed, %d not", blessed, plain)
	}
}

// However strong, a resistance cuts a bite by half at most: a blessing from INT 255 would
// cut 61 in a hundred, and cuts 50.
func TestAResistanceCutsABiteByHalfAtMost(t *testing.T) {
	plain, blessed := bittenWith(t, "", nil), bittenWith(t, skill.Blessing, &account.BuffCaster{Level: 20, Intelligence: 255})
	if blessed != plain*50/100 || plain == 0 {
		t.Fatalf("bitten for %d blessed, %d not", blessed, plain)
	}
}

// Reflection sends back 5 + (0.3·iq + 5)·k percent of every bite to the biter: 21 in a
// hundred from INT 90 at level 20, struck at the bitten one's hand.
func TestReflectionSendsABiteBack(t *testing.T) {
	s, dog, client, c := suraHits(t, 100)
	c.Buffs = map[account.Skill]time.Time{skill.Reflection: noon.Add(time.Minute)}
	c.BuffBy = map[account.Skill]account.BuffCaster{skill.Reflection: {Level: 20, Intelligence: 90}}
	c.HP = 10000
	pl := s.presence.all[1000]
	before := dog.p.fight.damageBy[pl]

	s.tick(noon)

	got := drain(t, client)
	d := protocol.NewReader(got[moveSize+1:])
	_, _, bite := d.U32(), d.U8(), d.I32()
	if back := dog.p.fight.damageBy[pl] - before; back != int(bite)*21/100 || back == 0 {
		t.Fatalf("sent back %d of a bite of %d", back, bite)
	}
}

// Dragon's Help gives her blows a chance to strike twice as hard, drawn as critical: over
// many blows some are, at INT 255's 61 in a hundred.
func TestDragonsHelpMakesSomeBlowsCritical(t *testing.T) {
	s, dog, client, c := suraHits(t, 100)
	c.Buffs = map[account.Skill]time.Time{skill.DragonsHelp: noon.Add(time.Hour)}
	c.BuffBy = map[account.Skill]account.BuffCaster{skill.DragonsHelp: {Level: 20, Intelligence: 255}}
	dog.p.fight.hp, dog.p.fight.maxHP = 1000000, 1000000
	critical := 0
	for i := range 40 {
		s.presence.attack(1000, dog.p.add.VID, 0, someRand, noon.Add(time.Duration(i)*time.Second))
		d := protocol.NewReader(read(t, client, damageSize)[1:])
		if _, flag, _ := d.U32(), d.U8(), d.I32(); flag == 1<<5 { // the client's DAMAGE_CRITICAL
			critical++
		}
	}
	if critical == 0 || critical == 40 {
		t.Fatalf("%d of 40 critical", critical)
	}
}

// Heal gives Kira 200 + 4·lv + (10·iq + 6·MTK + 600 to 800)·k hit points: 283 to 294 from
// a level-10 shaman of INT 20 with her fan at level 1, told to her.
func TestHealGivesThePlayerPickedHitPoints(t *testing.T) {
	s, _, kira, kiraClient := supporting(t)

	if err := s.onUseSkill(usedOn(skill.Heal, 1001)); err != nil {
		t.Fatal(err)
	}

	if got := kira.char.HP - 1; got < 283 || got > 294 {
		t.Fatalf("healed %d, want 283 to 294", got)
	}
	if point, _, value := pointChange(t, kiraClient, 1001); point != protocol.PointHP || value != int32(kira.char.HP) { //nolint:gosec // small
		t.Fatalf("told point %d at %d", point, value)
	}
}

// Swiftness raises her casting speed by 3 + 33k, to 104 at level 1, told to her, and her
// cooldowns shorten as the client counts them, by 10000/104 = 96 hundredths: Flying
// Talisman's 7 s becomes 6.72, so it is refused at 6.71 and may be cast again at 6.72.
func TestSwiftnessShortensCooldowns(t *testing.T) {
	s, client, _, _ := supporting(t)
	c := s.svc.presence.all[1000].char
	if err := s.onUseSkill(used(skill.Swiftness)); err != nil {
		t.Fatal(err)
	}
	if got := castingSpeedOf(c); got != 104 {
		t.Fatalf("casting speed %d", got)
	}
	if got := drain(t, client); !bytes.Contains(got, protocol.EncodePointChange(1000, protocol.PointCastingSpeed, 0, 104)) {
		t.Fatalf("got %x, want the casting speed told", got)
	}
	dog := s.svc.monsters[0].p.add.VID
	if err := s.onUseSkill(usedOn(skill.FlyingTalisman, dog)); err != nil {
		t.Fatal(err)
	}
	drain(t, client)
	for _, try := range []struct {
		at   time.Duration
		cast bool
	}{{6710 * time.Millisecond, false}, {6720 * time.Millisecond, true}} { // in order: a cast starts the cooldown again
		at, cast := try.at, try.cast
		s.svc.now = func() time.Time { return noon.Add(at) }
		sp := c.SP
		if err := s.onUseSkill(usedOn(skill.FlyingTalisman, dog)); err != nil {
			t.Fatal(err)
		}
		if got := c.SP != sp; got != cast {
			t.Fatalf("cast at %v: %v, want %v", at, got, cast)
		}
	}
}

// A cooldown is fixed when the skill is used, as the client fixes its own: Flying Talisman
// cast at Swiftness's 104 is ready at 6.72 s even when Swiftness has ended since.
func TestACooldownKeepsTheSpeedItWasCastAt(t *testing.T) {
	s, client, _, _ := supporting(t)
	c := s.svc.presence.all[1000].char
	if err := s.onUseSkill(used(skill.Swiftness)); err != nil {
		t.Fatal(err)
	}
	dog := s.svc.monsters[0].p.add.VID
	if err := s.onUseSkill(usedOn(skill.FlyingTalisman, dog)); err != nil {
		t.Fatal(err)
	}
	s.svc.presence.mu.Lock()
	s.svc.presence.endBuffs(s.svc.presence.all[1000])
	s.svc.presence.mu.Unlock()
	drain(t, client)
	s.svc.now = func() time.Time { return noon.Add(6720 * time.Millisecond) }
	sp := c.SP

	if err := s.onUseSkill(usedOn(skill.FlyingTalisman, dog)); err != nil {
		t.Fatal(err)
	}
	if c.SP == sp {
		t.Fatal("cooling down at 6.72 s after Swiftness ended")
	}
}

// Attack Up adds 5 + (0.2·iq + 15)·k to the attack of whoever it lands on: 5 from INT 20
// at level 1, 21 from INT 90 at 20 (21.5).
func TestAttackUpAddsToTheAttack(t *testing.T) {
	for want, by := range map[uint16]account.BuffCaster{5: {Level: 1, Intelligence: 20}, 21: {Level: 20, Intelligence: 90}} {
		c := &account.Character{Buffs: map[account.Skill]time.Time{skill.AttackUp: noon}, BuffBy: map[account.Skill]account.BuffCaster{skill.AttackUp: by}}
		if got := attackerOf(c).Bonus; got != want {
			t.Errorf("by %+v: %d, want %d", by, got, want)
		}
	}
}
