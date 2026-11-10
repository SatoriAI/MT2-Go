package game

import (
	"net"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/network"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/skill"
	"github.com/SatoriAI/MT2-Go/internal/world"
)

// shamanising is Rugor of warring as a level-10 shaman of INT 20 and DEX 10 with a fan in
// hand and every shaman attack at its first level; dogs dogs at home, one 250 off.
func shamanising(t *testing.T, dogs int) (*session, net.Conn) {
	t.Helper()
	s, client := warring(t, dogs)
	c := s.svc.record.state.all[1000].char
	c.Class, c.SkillGroup, c.Weapon = account.Shaman, "dragon", 7000
	c.Intelligence, c.Dexterity = 20, 10
	c.Skills = map[account.Skill]uint8{skill.FlyingTalisman: 1, skill.ShootingDragon: 1, skill.DragonRoar: 1,
		skill.LightningThrow: 1, skill.SummonLightning: 1, skill.LightningClaw: 1}
	return s, client
}

// Flying Talisman does its tooltip's damage, of her INT, level, hit rate and the fan's
// magic attack, 13 to 15: 70 + 40 + (400 + 65 to 75 + 50)·0.73·0.05, 128 to 129.
func TestFlyingTalismanDoesItsTooltipsDamage(t *testing.T) {
	s, client := shamanising(t, 1)
	dog := s.svc.monsters[0].p

	if err := s.onUseSkill(usedOn(skill.FlyingTalisman, dog.add.VID)); err != nil {
		t.Fatal(err)
	}

	if got := read(t, client, pointChangeSize); string(got) != string(manaSpent(38, 462)) { // 30 + 160 × 0.05
		t.Fatalf("told %x", got)
	}
	if got := dog.fight.damageBy[s.svc.record.state.all[1000]]; got < 128 || got > 129 {
		t.Fatalf("struck for %d, want 128 to 129", got)
	}
}

// Dragon Roar burns around her: it strikes every dog within 500, fifteen at most.
func TestDragonRoarStrikesAroundHer(t *testing.T) {
	s, client := shamanising(t, 3)

	if err := s.onUseSkill(used(skill.DragonRoar)); err != nil {
		t.Fatal(err)
	}

	read(t, client, pointChangeSize)
	if hurt, _ := struck(t, s, client); hurt != 4 {
		t.Fatalf("hurt %d, want the four within 500", hurt)
	}
}

// A burning dog is shown alight, and every two seconds loses a fifth of the burn's whole,
// told to the burner; after its fifth turn the fire goes out, shown to everyone.
func TestABurnTakesItsShareThenGoesOut(t *testing.T) {
	s, dog, client, _ := suraHits(t, 100)
	s.record.held(func(l locked) {
		l.burn(dog.p, s.record.state.all[1000], 7)
	})
	if got := read(t, client, updateSize); string(got) != string(protocol.EncodeCharacterUpdate(dog.p.info, dog.p.add, 1<<28)) {
		t.Fatalf("got %x, want the fire shown", got)
	}
	before := dog.p.fight.hp

	if !s.record.burnTick(dog.p, noon.Add(burnEvery)) || dog.p.fight.hp != before-7 {
		t.Fatalf("hp %d from %d", dog.p.fight.hp, before)
	}
	if got := read(t, client, damageSize); string(got) != string(protocol.EncodeDamage(dog.p.add.VID, protocol.DamageNormal, 7)) {
		t.Fatalf("told %x", got)
	}
	for turn := 2; turn < burnTurns; turn++ {
		if !s.record.burnTick(dog.p, noon.Add(time.Duration(turn)*burnEvery)) {
			t.Fatalf("out at turn %d", turn)
		}
	}
	if s.record.burnTick(dog.p, noon.Add(burnTurns*burnEvery)) || dog.p.fight.burning {
		t.Fatal("burns on past its fifth turn")
	}
}

// A burn's turns come from the world loop, all five of its whole in ten seconds, and the
// loop forgets it once it is out; a burn that takes the last hit points kills at the
// burner's hand.
func TestTheWorldLoopTicksBurnsAndTheyKill(t *testing.T) {
	s, dog, _, c := suraHits(t, 100)
	s.record.held(func(l locked) {
		l.burn(dog.p, s.record.state.all[1000], 3)
	})
	s.tick(noon)                        // the loop takes up the blow and the fire
	dog.chase = nil                     // so it neither bites nor walks
	s.healing, dog.healing = nil, false // nor heals
	before := dog.p.fight.hp
	for at := tick; at <= burnTurns*burnEvery+tick; at += tick {
		s.tick(noon.Add(at))
	}
	if got := before - dog.p.fight.hp; got != 15 || len(s.burnings) != 0 {
		t.Fatalf("lost %d, %d still burning", got, len(s.burnings))
	}

	dog.p.fight.hp = 2
	exp := c.Exp
	s.record.held(func(l locked) {
		l.burn(dog.p, s.record.state.all[1000], 3)
	})
	if s.record.burnTick(dog.p, noon.Add(time.Minute+burnEvery)) || !dog.p.dead() || c.Exp == exp {
		t.Fatal("the fire did not kill it at her hand")
	}
}

// Shooting Dragon sets some of what it leaves alive alight over many casts, at her INT
// 90 and level 20, but never a stone.
func TestShootingDragonSetsSomeAlightButNeverAStone(t *testing.T) {
	s := worldWith(t, world.Spawn{VNum: 8001, Count: 1, Spots: []world.Spot{{X: homeX, Y: homeY}}},
		world.Spawn{VNum: 101, X: homeX, Y: homeY, Count: 5})
	_, shaman := aPlayer(t, s, 1000, 9, "Rugor", 100)
	c := shaman.char
	c.Class, c.SP, c.Weapon, c.Intelligence = account.Shaman, 100000, 7000, 90
	c.Skills = map[account.Skill]uint8{skill.ShootingDragon: 20}
	for _, m := range s.monsters {
		m.p.fight.hp, m.p.fight.maxHP = 1000000, 1000000
	}
	stone := s.monsters[0]
	for range 100 {
		s.record.useSkill(1000, skill.ShootingDragon, stone.p.add.VID, noon, 0, s.rand, func() bool { return true })
	}
	alight := 0
	for _, m := range s.monsters[1:] {
		if m.p.fight.burning {
			alight++
		}
	}
	if alight == 0 || stone.p.fight.burning {
		t.Fatalf("%d of the dogs and wave alight, the stone %v", alight, stone.p.fight.burning)
	}
}

// Lightning Claw jumps from the dog picked to the nearest not yet struck within 600 of the
// last, and no further: of a line of dogs 500 apart, it strikes the first three and not
// the fourth, 1100 beyond the third, each jump 13 in a hundred weaker.
func TestLightningClawChainsFromEachToTheNext(t *testing.T) {
	s, client := shamanising(t, 4)
	for i, m := range s.svc.monsters[:4] {
		x := homeX + 2000 + int32(i)*500
		if i == 3 {
			x = homeX + 2000 + 2000
		}
		s.svc.record.held(func(l locked) {
			l.shift(m.p, protocol.Move{X: x, Y: homeY})
		})
	}
	drain(t, client)
	pl := s.svc.record.state.all[1000]
	pl.char.SP = 100000

	if err := s.onUseSkill(usedOn(skill.LightningClaw, s.svc.monsters[0].p.add.VID)); err != nil {
		t.Fatal(err)
	}
	drain(t, client)

	for i, m := range s.svc.monsters[:4] {
		if hurt := m.p.fight.damageBy[pl] > 0; hurt != (i < 3) {
			t.Errorf("dog %d hurt %v", i, hurt)
		}
	}
}

// Summon Lightning stuns at its own chance, (50 + 1000k/6)/10 percent: 58 in a thousand at
// its first level, where Stump's is 108.
func TestSummonLightningStunsAtItsOwnChance(t *testing.T) {
	p, _ := skill.Lookup(skill.SummonLightning)
	if got := p.StunChance(1); got != 58 {
		t.Fatalf("chance %d, want 58", got)
	}
}

// A shaman takes up Dragon at her teacher.
func TestAShamanTakesUpDragonAtHerTeacher(t *testing.T) {
	s := worldWith(t, world.Spawn{VNum: 20346, X: homeX, Y: homeY, Count: 1})
	kept := &keptSkills{learnt: map[account.Skill]uint8{}}
	s.skills = kept
	client, shaman := aPlayer(t, s, 1000, 9, "Gamma", 100)
	shaman.char.Class, shaman.char.Level = account.Shaman, 5
	read(t, client, addSize+infoSize)
	sess := &session{svc: s, conn: shaman.out.(*network.Conn), phase: protocol.PhaseGame,
		playing: playingAs(shaman)}

	if err := sess.onClick(vidBody(1)); err != nil {
		t.Fatal(err)
	}
	expectSaid(t, client, "Mogę nauczyć cię drogi Mocy Smoka. Czy chcesz nią podążać?[ENTER][QUESTION 1;Tak|2;Nie]")
}

// An attack that does not stun never does, however often it strikes: Sword Spin, over
// two hundred casts on a dog too tough to die.
func TestAnAttackThatDoesNotStunNeverStuns(t *testing.T) {
	s, _ := warring(t, 1)
	s.svc.record.state.all[1000].char.SP = 100000
	for range 200 {
		s.svc.record.useSkill(1000, skill.SwordSpin, 0, noon, 0, s.rand, func() bool { return true })
	}
	if s.svc.monsters[0].p.fight.stunned {
		t.Fatal("stunned by a spin")
	}
}

// Lightning Throw rolls its number from 5 to 15 times her INT, 100 to 300 at INT 20: 60 +
// 40 + (140 + 104 to 120 + 100 to 300)·0.73·0.05, 112 to 120.
func TestLightningThrowRollsByHerIntelligence(t *testing.T) {
	s, client := shamanising(t, 1)
	dog := s.svc.monsters[0].p

	if err := s.onUseSkill(usedOn(skill.LightningThrow, dog.add.VID)); err != nil {
		t.Fatal(err)
	}

	drain(t, client)
	if got := dog.fight.damageBy[s.svc.record.state.all[1000]]; got < 112 || got > 120 {
		t.Fatalf("struck for %d, want 112 to 120", got)
	}
}

// Only a burning skill burns: Flying Talisman, cast twenty times at its top level by a
// shaman of INT 250, who would burn a quarter of what Shooting Dragon struck, sets no dog
// alight.
func TestOnlyABurningSkillBurns(t *testing.T) {
	s, _ := shamanising(t, 4)
	c := s.svc.record.state.all[1000].char
	c.Intelligence, c.SP = 250, 100000
	c.Skills[skill.FlyingTalisman] = skill.Master
	dog := s.svc.monsters[0].p

	for i := range 20 {
		s.svc.now = func() time.Time { return noon.Add(time.Duration(i) * 10 * time.Second) }
		if err := s.onUseSkill(usedOn(skill.FlyingTalisman, dog.add.VID)); err != nil {
			t.Fatal(err)
		}
	}

	if len(s.svc.record.state.news.burned) != 0 {
		t.Fatalf("%d set alight", len(s.svc.record.state.news.burned))
	}
}
