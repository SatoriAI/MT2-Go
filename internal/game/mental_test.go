package game

import (
	"math/rand/v2"
	"net"
	"slices"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/skill"
	"github.com/SatoriAI/MT2-Go/internal/world"
)

// thinking is Rugor of warring, but a Mental warrior with CON 10 and every Mental skill
// at level, facing west, towards the dogs.
func thinking(t *testing.T, dogs int, level uint8) (*session, net.Conn) {
	t.Helper()
	s, client := warring(t, dogs)
	c := s.svc.record.state.all[1000].char
	c.SkillGroup, c.Vitality = "mental", 10
	c.Skills = map[account.Skill]uint8{skill.SpiritStrike: level, skill.Bash: level, skill.Stump: level,
		skill.StrongBody: level, skill.SwordStrike: level}
	s.svc.record.state.all[1000].add.Rotation = 270
	return s, client
}

// Spirit Strike on the dog she picked strikes it and up to three more within 100 of it.
func TestSpiritStrikeStrikesItsTargetAndThreeBeside(t *testing.T) {
	s, client := thinking(t, 6, 1)

	if err := s.onUseSkill(usedOn(skill.SpiritStrike, s.svc.monsters[0].p.add.VID)); err != nil {
		t.Fatal(err)
	}

	if got := read(t, client, pointChangeSize); string(got) != string(manaSpent(66, 434)) { // 60 + 120 × 0.05
		t.Fatalf("told %x", got)
	}
	if hurt, _ := struck(t, s, client); hurt != 4 {
		t.Fatalf("hurt %d, want 4", hurt)
	}
}

// Bash strikes up to eight within 200 in front of her: facing the dogs, eight of ten;
// facing away, none, for its mana all the same.
func TestBashStrikesOnlyInFrontOfHer(t *testing.T) {
	s, client := thinking(t, 10, 1)

	if err := s.onUseSkill(used(skill.Bash)); err != nil {
		t.Fatal(err)
	}

	read(t, client, pointChangeSize)
	if hurt, _ := struck(t, s, client); hurt != 8 {
		t.Fatalf("facing them, hurt %d, want 8", hurt)
	}

	away, awayClient := thinking(t, 10, 1)
	away.svc.record.state.all[1000].add.Rotation = 90
	if err := away.onUseSkill(used(skill.Bash)); err != nil {
		t.Fatal(err)
	}
	read(t, awayClient, pointChangeSize)
	if hurt, _ := struck(t, away, awayClient); hurt != 0 {
		t.Fatalf("facing away, hurt %d", hurt)
	}
}

// A stunned dog stands and bites nothing for two seconds, its stun shown to her, then
// bites again, its stun gone for everyone.
func TestAStunnedMonsterNeitherMovesNorBitesForTwoSeconds(t *testing.T) {
	s, dog, client, c := suraHits(t, 100)
	s.record.held(func(l locked) {
		l.stun(dog.p, noon)
	})
	stunned := dog.p.add
	if got := read(t, client, updateSize); string(got) != string(protocol.EncodeCharacterUpdate(dog.p.info, stunned, 1<<5)) {
		t.Fatalf("got %x, want the stun shown", got)
	}

	s.tick(noon)
	s.tick(noon.Add(time.Second))
	expectNothing(t, client)

	s.tick(noon.Add(stunFor))
	if got := read(t, client, updateSize); string(got) != string(protocol.EncodeCharacterUpdate(dog.p.info, stunned, 0)) {
		t.Fatalf("got %x, want the stun ended", got)
	}
	expectBite(t, client, dog, c)
}

// A second stun holds it two seconds from then, not two more on top.
func TestAStunAgainHoldsItTwoSecondsFromThen(t *testing.T) {
	s, dog, client, _ := suraHits(t, 100)
	s.record.held(func(l locked) {
		l.stun(dog.p, noon)
		l.stun(dog.p, noon.Add(time.Second))
	})
	read(t, client, updateSize)
	expectNothing(t, client) // shown once

	if got := dog.p.fight.stunnedUntil; !got.Equal(noon.Add(time.Second + stunFor)) {
		t.Fatalf("stunned until %v", got)
	}
}

// A stunned monster whose fight ends goes home awake, its stun gone for everyone.
func TestAStunnedMonsterGoesHomeAwake(t *testing.T) {
	s, dog, client, _ := suraHits(t, 100)
	s.record.held(func(l locked) {
		l.stun(dog.p, noon)
	})
	read(t, client, updateSize)
	s.record.state.all[1000].char.HP = 0 // she lies dead, so it gives up

	s.tick(noon)

	if dog.p.fight.stunned || mobAffects(dog.p) != 0 {
		t.Fatal("it went home stunned")
	}
}

// Stump, at level 20, stuns some of the ten dogs around her that it leaves alive, and
// never a Metin stone, however often.
func TestStumpStunsSomeAroundHerButNeverAStone(t *testing.T) {
	s, _ := thinking(t, 10, 20)
	if err := s.onUseSkill(used(skill.Stump)); err != nil {
		t.Fatal(err)
	}
	stunned := 0
	for _, m := range s.svc.monsters {
		if m.p.fight.stunned {
			stunned++
		}
	}
	if stunned == 0 || stunned == 10 {
		t.Fatalf("%d of 10 stunned at 18%%", stunned)
	}

	stones := worldWith(t, world.Spawn{VNum: 8001, Count: 1, Spots: []world.Spot{{X: homeX, Y: homeY}}})
	_, rugor := aPlayer(t, stones, 1000, 9, "Rugor", 100)
	rugor.char.Class, rugor.char.SP, rugor.char.Skills = account.Warrior, 100000, map[account.Skill]uint8{skill.Stump: 20}
	stone := stones.monsters[0]
	for range 50 {
		stones.record.useSkill(1000, skill.Stump, 0, noon, 0, stones.rand, func() bool { return true })
	}
	if stone.p.fight.stunned {
		t.Fatal("the stone was stunned")
	}
}

// Strong Body raises her defence by 7 + (40 + 0.2·STR + 0.4·CON)·k, 32 at level 20 with
// STR 30 and CON 10, and slows her by 1 + 9k, 5, showing its effect.
func TestStrongBodyRaisesHerDefenceAndSlowsHer(t *testing.T) {
	s, client := thinking(t, 1, 20)
	pl := s.svc.record.state.all[1000]
	before, walk := defence(pl.char), pl.add.MoveSpeed

	if err := s.onUseSkill(used(skill.StrongBody)); err != nil {
		t.Fatal(err)
	}

	slower := pl.add
	slower.MoveSpeed = walk - 5
	want := string(manaSpent(190, 310)) + // 80 + 220 × 0.5
		string(protocol.EncodeCharacterUpdate(pl.info, slower, 1<<16)) +
		string(protocol.EncodePointChange(1000, protocol.PointDefence, 0, int32(before+32))) + //nolint:gosec // small
		string(protocol.EncodePointChange(1000, protocol.PointAttackSpeed, 0, int32(pl.add.AttackSpeed))) +
		string(protocol.EncodePointChange(1000, protocol.PointMoveSpeed, 0, int32(walk-5)))
	if got := read(t, client, len(want)); string(got) != want {
		t.Fatalf("got  %x\nwant %x", got, want)
	}
}

// Sword Strike lands on the dog she picked, 1100 off, and the dogs within 200 of it.
func TestSwordStrikeLandsOnThePickedFarOff(t *testing.T) {
	s, client := thinking(t, 2, 1)
	far := s.svc.monsters[2]
	s.svc.record.held(func(l locked) {
		l.shift(far.p, protocol.Move{X: homeX + 1200, Y: homeY})
	})

	if err := s.onUseSkill(usedOn(skill.SwordStrike, far.p.add.VID)); err != nil {
		t.Fatal(err)
	}

	read(t, client, pointChangeSize)
	if hurt, _ := struck(t, s, client); hurt != 1 || far.p.fight.damageBy[s.svc.record.state.all[1000]] == 0 {
		t.Fatalf("hurt %d, the far dog spared", hurt)
	}
}

// With nothing picked, Sword Strike lands on the nearest dog in front of her; with none
// in front, it is not used and costs nothing.
func TestSwordStrikeFindsTheNearestInFront(t *testing.T) {
	s, client := thinking(t, 2, 1)
	if err := s.onUseSkill(used(skill.SwordStrike)); err != nil {
		t.Fatal(err)
	}
	read(t, client, pointChangeSize)
	if hurt, _ := struck(t, s, client); hurt != 2 {
		t.Fatalf("hurt %d, want the two at home", hurt)
	}

	away, awayClient := thinking(t, 2, 1)
	away.svc.record.state.all[1000].add.Rotation = 90
	far := away.svc.monsters[2] // 250 east, in front of her now
	away.svc.record.held(func(l locked) {
		l.shift(far.p, protocol.Move{X: homeX - 2000, Y: homeY})
	})
	drain(t, awayClient)
	if err := away.onUseSkill(used(skill.SwordStrike)); err != nil {
		t.Fatal(err)
	}
	if got := drain(t, awayClient); len(got) != 0 || away.svc.record.state.all[1000].char.SP != 500 {
		t.Fatalf("used facing nothing: %x", got)
	}
}

// Berserk makes her take (5 + 10k)% more of every bite: 5% more at its first level.
func TestBerserkMakesHerTakeMoreOfEveryBite(t *testing.T) {
	bitten := func(berserk bool) int32 {
		s, _, client, c := suraHits(t, 100)
		if berserk {
			c.Skills = map[account.Skill]uint8{skill.Berserk: 1}
			c.Buffs = map[account.Skill]time.Time{skill.Berserk: noon.Add(time.Minute)}
		}
		s.rand = rand.New(rand.NewPCG(9, 9)) //nolint:gosec // the same roll for both
		c.HP = 10000
		s.tick(noon)
		got := packets(t, drain(t, client))
		i := slices.IndexFunc(got, func(p []byte) bool { return protocol.ServerHeader(p[0]) == protocol.ServerDamage })
		if i < 0 {
			t.Fatal("no bite told")
		}
		d := protocol.NewReader(got[i][1:])
		_, _, damage := d.U32(), d.U8(), d.I32()
		return damage
	}
	plain, more := bitten(false), bitten(true)
	if more != plain*105/100 || plain == 0 {
		t.Fatalf("bitten for %d berserking, %d not", more, plain)
	}
}

// A dog killed while stunned lies dead, not stunned, for anyone who comes to see it.
func TestAMonsterKilledStunnedIsNotStunnedDead(t *testing.T) {
	s, dog, client, _ := suraHits(t, 100)
	pl := s.record.state.all[1000]
	s.record.held(func(l locked) {
		l.stun(dog.p, noon)
		l.strike(pl, dog.p, dog.p.fight.hp, noon)
	})
	drain(t, client)

	if mobAffects(dog.p) != 0 {
		t.Fatal("the body is stunned")
	}
}

// A stunned dog comes to as its stun ends, not at its next half-second step: stunned a
// quarter second into a step, it bites at two and a quarter seconds.
func TestAStunEndsOnTime(t *testing.T) {
	s, dog, client, c := suraHits(t, 100)
	s.tick(noon)
	expectBite(t, client, dog, c)
	s.record.held(func(l locked) {
		l.stun(dog.p, noon.Add(250*time.Millisecond))
	})
	read(t, client, updateSize)

	for at := 500 * time.Millisecond; at < 2250*time.Millisecond; at += tick {
		s.tick(noon.Add(at)) // the loop's ticks while stunned
	}
	expectNothing(t, client)
	s.tick(noon.Add(2250 * time.Millisecond))

	read(t, client, updateSize) // awake
	expectBite(t, client, dog, c)
}

// Strength raised under Strong Body raises her defence with it, and she is told: from STR
// 39 to 40 at level 20 with CON 10, its 32 becomes 33.
func TestStrengthRaisedUnderStrongBodyTellsHerDefence(t *testing.T) {
	s, client := thinking(t, 1, 20)
	c := s.svc.record.state.all[1000].char
	c.Strength, c.StatPoints = 39, 1
	if err := s.onUseSkill(used(skill.StrongBody)); err != nil {
		t.Fatal(err)
	}
	drain(t, client)
	before := defence(c)

	s.svc.record.spendStatPoint(1000, "st")

	read(t, client, 2*pointChangeSize) // the stat, the points left
	if got := read(t, client, pointChangeSize); string(got) != string(defenceTold(before+1)) {
		t.Fatalf("told %x, defence before %d", got, before)
	}
}
