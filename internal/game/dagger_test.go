package game

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/network"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/skill"
	"github.com/SatoriAI/MT2-Go/internal/world"
)

// stabbing is Rugor of warring as a Dagger ninja with a dagger in hand and every Dagger
// skill at its first level.
func stabbing(t *testing.T, dogs int) (*session, net.Conn) {
	t.Helper()
	s, client := warring(t, dogs)
	c := s.svc.presence.all[1000].char
	c.Class, c.SkillGroup, c.Weapon = account.Ninja, "dagger", 1000
	c.Skills = map[account.Skill]uint8{skill.Ambush: 1, skill.FastAttack: 1, skill.RollingDagger: 1}
	return s, client
}

// Ambush on a dog facing away from her, she standing behind it, strikes half as hard
// again as on one facing her.
func TestAmbushStrikesHarderFromBehind(t *testing.T) {
	struckFacing := func(rotation float32) int {
		s, client := stabbing(t, 1)
		dog := s.svc.monsters[0].p
		dog.add.Rotation = rotation // she stands east of it
		if err := s.onUseSkill(usedOn(skill.Ambush, dog.add.VID)); err != nil {
			t.Fatal(err)
		}
		drain(t, client)
		return dog.fight.damageBy[s.svc.presence.all[1000]]
	}
	facing, behind := struckFacing(90), struckFacing(270)
	if facing == 0 || behind < facing*14/10 || behind > facing*16/10 {
		t.Fatalf("struck %d facing her, %d from behind", facing, behind)
	}
}

// Ambush needs a dagger or a sword in hand: with a glaive it is not used.
func TestAmbushNeedsADaggerOrASword(t *testing.T) {
	s, client := stabbing(t, 1)
	s.svc.presence.all[1000].char.Weapon = 3000
	if err := s.onUseSkill(usedOn(skill.Ambush, s.svc.monsters[0].p.add.VID)); err != nil {
		t.Fatal(err)
	}
	expectNothing(t, client)

	s.svc.presence.all[1000].char.Weapon = 10
	if err := s.onUseSkill(usedOn(skill.Ambush, s.svc.monsters[0].p.add.VID)); err != nil {
		t.Fatal(err)
	}
	read(t, client, pointChangeSize)
}

// Fast Attack on the dog she picked 700 off moves her to 270 from it, as a neighbour sees, and
// strikes it; one picked beyond 800 is out of its reach.
func TestFastAttackDashesToItsTarget(t *testing.T) {
	s, client := stabbing(t, 1)
	kira, _ := aPlayer(t, s.svc, 1001, 8, "Kira", 0)
	read(t, client, addSize+infoSize) // Kira
	drain(t, kira)
	dog := s.svc.monsters[0]
	s.svc.presence.mu.Lock()
	s.svc.presence.moveLocked(dog.p, protocol.Move{X: homeX + 700, Y: homeY})
	s.svc.presence.mu.Unlock()
	drain(t, client)
	drain(t, kira)

	if err := s.onUseSkill(usedOn(skill.FastAttack, dog.p.add.VID)); err != nil {
		t.Fatal(err)
	}

	pl := s.svc.presence.all[1000]
	if pl.add.X != homeX+700-dashStop || pl.add.Y != homeY || dog.p.fight.damageBy[pl] == 0 {
		t.Fatalf("at (%d, %d), struck %d", pl.add.X, pl.add.Y, dog.p.fight.damageBy[pl])
	}
	if got := drain(t, kira); !bytes.Contains(got, []byte{byte(protocol.ServerMove)}) {
		t.Fatalf("Kira got %x, want her move", got)
	}

	s.svc.presence.mu.Lock()
	s.svc.presence.moveLocked(dog.p, protocol.Move{X: pl.add.X + 900, Y: homeY})
	s.svc.presence.mu.Unlock()
	drain(t, client)
	s.svc.cooldowns = cooldowns{}
	if err := s.onUseSkill(usedOn(skill.FastAttack, dog.p.add.VID)); err != nil {
		t.Fatal(err)
	}
	expectNothing(t, client)
}

// Rolling Dagger strikes every dog within 250 of her, and not one 280 off.
func TestRollingDaggerStrikesAroundHer(t *testing.T) {
	s, client := stabbing(t, 3)
	s.svc.presence.mu.Lock()
	s.svc.presence.moveLocked(s.svc.monsters[3].p, protocol.Move{X: homeX + 380, Y: homeY})
	s.svc.presence.mu.Unlock()
	drain(t, client)

	if err := s.onUseSkill(used(skill.RollingDagger)); err != nil {
		t.Fatal(err)
	}

	if got := read(t, client, pointChangeSize); string(got) != string(manaSpent(57, 443)) { // 50 + 140 × 0.05
		t.Fatalf("told %x", got)
	}
	if hurt, _ := struck(t, s, client); hurt != 3 {
		t.Fatalf("hurt %d, want the three beside her", hurt)
	}
}

// Rolling Dagger strikes twelve at most.
func TestRollingDaggerStrikesTwelveAtMost(t *testing.T) {
	s, client := stabbing(t, 14)

	if err := s.onUseSkill(used(skill.RollingDagger)); err != nil {
		t.Fatal(err)
	}

	read(t, client, pointChangeSize)
	if hurt, _ := struck(t, s, client); hurt != 12 {
		t.Fatalf("hurt %d, want 12", hurt)
	}
}

// Rolling Dagger poisons some of the dogs beside a stone, over many casts, but never the
// stone.
func TestRollingDaggerPoisonsSomeButNeverAStone(t *testing.T) {
	s := worldWith(t, world.Spawn{VNum: 8001, Count: 1, Spots: []world.Spot{{X: homeX, Y: homeY}}},
		world.Spawn{VNum: 101, X: homeX, Y: homeY, Count: 5})
	_, ninja := aPlayer(t, s, 1000, 9, "Rugor", 100)
	c := ninja.char
	c.Class, c.SP, c.Weapon, c.Skills = account.Ninja, 100000, 1000, map[account.Skill]uint8{skill.RollingDagger: 20}
	for _, m := range s.monsters {
		m.p.fight.hp, m.p.fight.maxHP = 1000000, 1000000
	}
	for range 200 {
		s.presence.useSkill(1000, skill.RollingDagger, 0, noon, 0, s.rand, func() bool { return true })
	}
	stone, poisoned := s.monsters[0], 0
	for _, m := range s.monsters[1:] {
		if m.p.fight.poisoned {
			poisoned++
		}
	}
	if poisoned == 0 || stone.p.fight.poisoned {
		t.Fatalf("%d of the dogs and wave poisoned, the stone %v", poisoned, stone.p.fight.poisoned)
	}
}

// poisoned is a dog of aDog poisoned by her at noon, with her client's news of it read.
func poisoned(t *testing.T) (*Service, *monster, net.Conn) {
	t.Helper()
	s, dog, client, _ := suraHits(t, 100)
	s.presence.mu.Lock()
	s.presence.poison(dog.p, s.presence.all[1000], noon)
	s.presence.mu.Unlock()
	read(t, client, updateSize)
	s.tick(noon) // the loop takes it up, and the dog bites back
	drain(t, client)
	return s, dog, client
}

// A poisoned dog is shown poisoned, and every two seconds loses 1% of its hit points, at
// least one, told to the poisoner as poison; the poison never makes it fight harder.
func TestAPoisonTakesItsShareEveryTwoSeconds(t *testing.T) {
	s, dog, client := poisoned(t)
	if mobAffects(dog.p) != 1<<3 { // the client's AFFECT_POISON
		t.Fatalf("affects %x", mobAffects(dog.p))
	}
	before := dog.p.fight.hp

	s.presence.poisonTick(dog.p, noon.Add(poisonEvery))

	if got := dog.p.fight.hp; got != before-1 {
		t.Fatalf("hp %d, from %d", got, before)
	}
	if got := read(t, client, damageSize); string(got) != string(protocol.EncodeDamage(dog.p.add.VID, 2, 1)) { // the client's DAMAGE_POISON
		t.Fatalf("told %x", got)
	}
	if len(s.presence.provoked) != 0 {
		t.Fatal("the poison provoked it")
	}
}

// A poison ends after thirty seconds, shown to everyone; a second one before then runs
// thirty seconds from it instead.
func TestAPoisonEndsAfterThirtySecondsOrIsRenewed(t *testing.T) {
	s, dog, client := poisoned(t)
	s.presence.mu.Lock()
	s.presence.poison(dog.p, s.presence.all[1000], noon.Add(10*time.Second))
	s.presence.mu.Unlock()

	if !s.presence.poisonTick(dog.p, noon.Add(poisonFor)) {
		t.Fatal("ended at thirty seconds of the first, though renewed")
	}
	drain(t, client)
	if s.presence.poisonTick(dog.p, noon.Add(10*time.Second+poisonFor)) || dog.p.fight.poisoned {
		t.Fatal("ran on past the second's thirty seconds")
	}
	if got := read(t, client, updateSize); string(got) != string(protocol.EncodeCharacterUpdate(dog.p.info, dog.p.add, 0)) {
		t.Fatalf("got %x, want the poison ended", got)
	}
}

// A poison that takes a dog's last hit points kills it at the poisoner's hand: she is
// given its experience, and it lies dead, no longer poisoned.
func TestAPoisonKillsAtThePoisonersHand(t *testing.T) {
	s, dog, _ := poisoned(t)
	dog.p.fight.hp = 1
	c := s.presence.all[1000].char
	exp := c.Exp

	if s.presence.poisonTick(dog.p, noon.Add(poisonEvery)) {
		t.Fatal("the poison runs on after its death")
	}

	if !dog.p.dead() || dog.p.fight.poisoned || c.Exp == exp || len(s.presence.fallen) != 1 {
		t.Fatalf("dead %v, poisoned %v, exp %d from %d", dog.p.dead(), dog.p.fight.poisoned, c.Exp, exp)
	}
}

// A poison ends once its poisoner has left the world.
func TestAPoisonEndsWithItsPoisonerGone(t *testing.T) {
	s, dog, _ := poisoned(t)
	s.presence.leave(1000)

	if s.presence.poisonTick(dog.p, noon.Add(poisonEvery)) || dog.p.fight.poisoned {
		t.Fatal("it runs on for nobody")
	}
}

// The world loop ticks the poison every two seconds: by ten seconds the dog has lost five
// hit points, and the loop holds it no more once it ends.
func TestTheWorldLoopTicksPoisons(t *testing.T) {
	s, dog, _ := poisoned(t)
	dog.chase = nil // so it neither bites nor walks
	s.healing, dog.healing = nil, false
	before := dog.p.fight.hp

	for at := tick; at <= 10*time.Second; at += tick {
		s.tick(noon.Add(at))
	}
	if got := before - dog.p.fight.hp; got != 5 {
		t.Fatalf("lost %d", got)
	}
	s.tick(noon.Add(poisonFor + poisonEvery))
	if len(s.poisons) != 0 {
		t.Fatalf("%d still held", len(s.poisons))
	}
}

// A ninja takes up Dagger at its teacher.
func TestANinjaTakesUpDaggerAtItsTeacher(t *testing.T) {
	s := worldWith(t, world.Spawn{VNum: 20342, X: homeX, Y: homeY, Count: 1})
	kept := &keptSkills{learnt: map[account.Skill]uint8{}}
	s.skills = kept
	client, ninja := aPlayer(t, s, 1000, 9, "Sheya", 100)
	ninja.char.Class, ninja.char.Level = account.Ninja, 5
	read(t, client, addSize+infoSize)
	sess := &session{svc: s, conn: ninja.out.(*network.Conn), phase: protocol.PhaseGame,
		playing: &character{vid: 1000, Character: *ninja.char}}

	if err := sess.onClick(vidBody(1)); err != nil {
		t.Fatal(err)
	}
	expectSaid(t, client, "Mogę nauczyć cię drogi Mocy Ostrza. Czy chcesz nią podążać?[ENTER][QUESTION 1;Tak|2;Nie]")
}

// Fast Attack stamps the dash with the client clock, as every move the server makes, and
// moves a ninja nearer than 270 back out to it, as the original's warp does.
func TestFastAttackTimesItsDashAndBacksTheNearOut(t *testing.T) {
	s, client := stabbing(t, 1)
	kira, _ := aPlayer(t, s.svc, 1001, 8, "Kira", 0)
	read(t, client, addSize+infoSize)
	drain(t, kira)
	dog := s.svc.monsters[0]
	s.svc.presence.mu.Lock()
	s.svc.presence.moveLocked(dog.p, protocol.Move{X: homeX + 700, Y: homeY})
	s.svc.presence.mu.Unlock()
	drain(t, client)
	drain(t, kira)

	s.svc.presence.useSkill(1000, skill.FastAttack, dog.p.add.VID, noon, 4242, s.rand, func() bool { return true })

	got := drain(t, kira)
	i := bytes.IndexByte(got, byte(protocol.ServerMove))
	if i < 0 || binary.LittleEndian.Uint32(got[i+16:]) != 4242 {
		t.Fatalf("Kira got %x, want the dash at 4242", got)
	}
	pl := s.svc.presence.all[1000]
	s.svc.presence.mu.Lock()
	s.svc.presence.moveLocked(pl, protocol.Move{X: dog.p.add.X - 200, Y: homeY})
	s.svc.presence.mu.Unlock()
	s.svc.presence.useSkill(1000, skill.FastAttack, dog.p.add.VID, noon, 4243, s.rand, func() bool { return true })
	if pl.add.X != dog.p.add.X-dashStop {
		t.Fatalf("at %d, want %d", pl.add.X, dog.p.add.X-dashStop)
	}
}
