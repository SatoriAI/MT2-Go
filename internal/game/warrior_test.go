package game

import (
	"context"
	"math/rand/v2"
	"net"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/mob"
	"github.com/SatoriAI/MT2-Go/internal/network"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/skill"
	"github.com/SatoriAI/MT2-Go/internal/world"
)

// warring is Rugor, a level-10 Body warrior of STR 30 with 500 mana and a sword in hand,
// every Body skill the game has at 1, 100 east of dogs dogs at home and one 250 further
// off, which leave nothing and are too tough to die; his client has read what entering
// showed.
func warring(t *testing.T, dogs int) (*session, net.Conn) {
	t.Helper()
	s := worldWith(t, world.Spawn{VNum: 101, X: homeX, Y: homeY, Count: dogs}, world.Spawn{VNum: 101, X: homeX + 350, Y: homeY, Count: 1})
	for _, m := range s.monsters {
		bare := *m.proto
		bare.Drops, bare.Yang = nil, mob.Yang{}
		m.proto = &bare
		m.p.fight.hp, m.p.fight.maxHP = 100000, 100000
	}
	s.now = func() time.Time { return noon }
	client, rugor := aPlayer(t, s, 1000, 9, "Rugor", 100)
	c := rugor.char
	c.Class, c.Level, c.SP, c.HP, c.SkillGroup, c.Weapon = account.Warrior, 10, 500, 500, "body", 10
	c.Strength = 30
	c.Skills = map[account.Skill]uint8{skill.ThreeWayCut: 1, skill.SwordSpin: 1, skill.Berserk: 1, skill.SwordAura: 1}
	points, _ := c.Points()
	rugor.add.MoveSpeed, rugor.add.AttackSpeed = points.MoveSpeed, points.AttackSpeed
	read(t, client, (dogs+1)*addSize)
	sess := &session{svc: s, conn: rugor.out.(*network.Conn), phase: protocol.PhaseGame, rand: rand.New(rand.NewPCG(5, 6)), //nolint:gosec // a fixed seed
		playing: &character{vid: 1000, Character: *c}}
	return sess, client
}

// struck is how many of the world's monsters the player has hurt, and the blows their
// client was told of, read.
func struck(t *testing.T, s *session, client net.Conn) (int, int) {
	t.Helper()
	pl := s.svc.presence.all[1000]
	hurt := 0
	for _, m := range s.svc.monsters {
		if m.p.fight.damageBy[pl] > 0 {
			hurt++
		}
	}
	got := drain(t, client)
	return hurt, len(got) / damageSize
}

// Three-Way Cut on the dog she picked costs its mana, then cuts it and the dogs within 200
// of it, five at most, each three times.
func TestThreeWayCutCutsFiveAroundItsTargetThriceEach(t *testing.T) {
	s, client := warring(t, 7)
	target := s.svc.monsters[0].p.add.VID

	if err := s.onUseSkill(usedOn(skill.ThreeWayCut, target)); err != nil {
		t.Fatal(err)
	}

	if got := read(t, client, pointChangeSize); string(got) != string(manaSpent(45, 455)) { // 40 + 100 × 0.05
		t.Fatalf("told %x", got)
	}
	if hurt, blows := struck(t, s, client); hurt != 5 || blows != 15 {
		t.Fatalf("hurt %d with %d blows, want 5 with 15", hurt, blows)
	}
	if s.svc.monsters[0].p.fight.damageBy[s.svc.presence.all[1000]] == 0 {
		t.Fatal("the target was spared")
	}
}

// Three-Way Cut needs the dog she picked in reach, and a sword or a two-handed weapon:
// with nothing picked, or a fan in hand, it is not used and costs nothing.
func TestThreeWayCutNeedsATargetAndABlade(t *testing.T) {
	s, client := warring(t, 1)
	if err := s.onUseSkill(usedOn(skill.ThreeWayCut, 0)); err != nil {
		t.Fatal(err)
	}
	s.svc.presence.all[1000].char.Weapon = 7000
	if err := s.onUseSkill(usedOn(skill.ThreeWayCut, s.svc.monsters[0].p.add.VID)); err != nil {
		t.Fatal(err)
	}

	expectNothing(t, client)
	if c := s.svc.presence.all[1000].char; c.SP != 500 {
		t.Fatalf("mana %d", c.SP)
	}
}

// A glaive, two-handed, does for Three-Way Cut as a sword does.
func TestThreeWayCutCutsWithAGlaive(t *testing.T) {
	s, client := warring(t, 1)
	s.svc.presence.all[1000].char.Weapon = 3000

	if err := s.onUseSkill(usedOn(skill.ThreeWayCut, s.svc.monsters[0].p.add.VID)); err != nil {
		t.Fatal(err)
	}

	read(t, client, pointChangeSize)
	if hurt, blows := struck(t, s, client); hurt != 1 || blows != 3 {
		t.Fatalf("hurt %d with %d blows", hurt, blows)
	}
}

// Sword Spin strikes every dog within 200 of her once, and not the one 250 off.
func TestSwordSpinStrikesAroundHer(t *testing.T) {
	s, client := warring(t, 3)

	if err := s.onUseSkill(used(skill.SwordSpin)); err != nil {
		t.Fatal(err)
	}

	if got := read(t, client, pointChangeSize); string(got) != string(manaSpent(56, 444)) { // 50 + 130 × 0.05
		t.Fatalf("told %x", got)
	}
	if hurt, blows := struck(t, s, client); hurt != 3 || blows != 3 {
		t.Fatalf("hurt %d with %d blows, want the three beside her", hurt, blows)
	}
}

// Sword Spin strikes at most twelve, the client's number of victims.
func TestSwordSpinStrikesTwelveAtMost(t *testing.T) {
	s, client := warring(t, 14)

	if err := s.onUseSkill(used(skill.SwordSpin)); err != nil {
		t.Fatal(err)
	}

	read(t, client, pointChangeSize)
	if hurt, _ := struck(t, s, client); hurt != 12 {
		t.Fatalf("hurt %d, want 12", hurt)
	}
}

// Berserk makes her strike and move faster, by 2 and 1 at its first level: she is told
// both speeds, she and those who see her see its effect and pace, and her blows may come
// that much sooner; when it ends, both are back.
func TestBerserkSpeedsHerUpUntilItEnds(t *testing.T) {
	s, client := warring(t, 1)
	pl := s.svc.presence.all[1000]
	walk, strike := pl.add.MoveSpeed, pl.add.AttackSpeed

	if err := s.onUseSkill(used(skill.Berserk)); err != nil {
		t.Fatal(err)
	}

	faster := pl.add
	faster.MoveSpeed, faster.AttackSpeed = walk+1, strike+2
	want := string(manaSpent(57, 443)) + // 50 + 140 × 0.05
		string(protocol.EncodeCharacterUpdate(pl.info, faster, 1<<14)) +
		string(protocol.EncodePointChange(1000, protocol.PointAttackSpeed, 0, int32(strike+2))) +
		string(protocol.EncodePointChange(1000, protocol.PointMoveSpeed, 0, int32(walk+1)))
	if got := read(t, client, len(want)); string(got) != want {
		t.Fatalf("got  %x\nwant %x", got, want)
	}
	if got := s.svc.presence.attackSpeedOf(1000); got != strike+2 {
		t.Fatalf("blows timed at %d", got)
	}

	s.svc.tick(noon)
	s.svc.tick(noon.Add(skill.Proto{Lasts: skill.Scaled{Base: 60, PerPower: 90}}.LastsAt(1)))

	if pl.add.MoveSpeed != walk || pl.add.AttackSpeed != strike || affectsOf(pl.char) != 0 {
		t.Fatalf("after: %+v, affects %x", pl.add, affectsOf(pl.char))
	}
}

// Sword Aura adds (100 + STR)·k to her attack, 6 at its first level with STR 30, and shows
// its effect; with the sword taken out of hand, it ends.
func TestSwordAuraRaisesHerAttackWhileSheHoldsABlade(t *testing.T) {
	s, client := warring(t, 1)
	pl := s.svc.presence.all[1000]

	if err := s.onUseSkill(used(skill.SwordAura)); err != nil {
		t.Fatal(err)
	}

	read(t, client, pointChangeSize)
	if got := read(t, client, updateSize); string(got) != string(protocol.EncodeCharacterUpdate(pl.info, pl.add, 1<<15)) {
		t.Fatalf("got %x", got)
	}
	if got := attackerOf(pl.char).Bonus; got != 6 {
		t.Fatalf("attack bonus %d, want 6", got)
	}

	s.svc.presence.dress(1000, outfit{})

	if _, running := pl.char.Buffs[skill.SwordAura]; running || attackerOf(pl.char).Bonus != 0 {
		t.Fatal("the aura runs on without a blade")
	}
}

// Each group has its own skill window: its first place is Three-Way Cut to a Body warrior,
// Spirit Strike to a Mental one, Finger Strike to a Weaponry sura and Repetitive Shot to
// an Archery ninja, whose fourth is Feather Walk; with no group chosen there is none.
func TestTheSkillWindowsPlacesAreTheGroupsOwn(t *testing.T) {
	for group, want := range map[account.SkillGroup]account.Skill{"body": skill.ThreeWayCut, "mental": skill.SpiritStrike,
		"weaponry": skill.FingerStrike, "archery": skill.RepetitiveShot} {
		if got, ok := skillByWindowSlot(group, 1); !ok || got != want {
			t.Errorf("%s: %q, want %q", group, got, want)
		}
	}
	if got, ok := skillByWindowSlot("archery", 4); !ok || got != skill.FeatherWalk {
		t.Errorf("archery's fourth: %q", got)
	}
	if got, ok := skillByWindowSlot("", 1); ok {
		t.Errorf("no group: %q", got)
	}
}

// A warrior of level 5 takes up Body at its teacher, as a sura takes up Weaponry.
func TestAWarriorTakesUpBodyAtItsTeacher(t *testing.T) {
	s := worldWith(t, world.Spawn{VNum: 20340, X: homeX, Y: homeY, Count: 1})
	kept := &keptSkills{learnt: map[account.Skill]uint8{}}
	s.skills = kept
	client, rugor := aPlayer(t, s, 1000, 9, "Rugor", 100)
	rugor.char.Class, rugor.char.Level = account.Warrior, 5
	read(t, client, addSize+infoSize)
	sess := &session{svc: s, conn: rugor.out.(*network.Conn), phase: protocol.PhaseGame,
		playing: &character{vid: 1000, Character: *rugor.char}}

	if err := sess.onClick(vidBody(1)); err != nil {
		t.Fatal(err)
	}
	expectSaid(t, client, "Mogę nauczyć cię drogi Mocy Ciała. Czy chcesz nią podążać?[ENTER][QUESTION 1;Tak|2;Nie]")
	for range 2 {
		if err := sess.onScriptAnswer(context.Background(), answered(0)); err != nil {
			t.Fatal(err)
		}
	}

	if kept.group != "body" || rugor.char.SkillGroup != "body" {
		t.Fatalf("kept %q, in play %q", kept.group, rugor.char.SkillGroup)
	}
}

// With more than twelve in reach, Sword Spin strikes the nearest: of twelve dogs 100 from
// her and two 190 off, the two are spared, as is the dog out of reach.
func TestSwordSpinStrikesTheNearestFirst(t *testing.T) {
	s, client := warring(t, 14)
	spared := map[*present]bool{s.svc.monsters[14].p: true}
	for _, m := range s.svc.monsters[:2] {
		s.svc.presence.moveLocked(m.p, protocol.Move{X: homeX - 90, Y: homeY})
		spared[m.p] = true
	}

	if err := s.onUseSkill(used(skill.SwordSpin)); err != nil {
		t.Fatal(err)
	}

	read(t, client, pointChangeSize)
	drain(t, client)
	for _, m := range s.svc.monsters {
		if hurt := m.p.fight.damageBy[s.svc.presence.all[1000]] > 0; hurt == spared[m.p] {
			t.Errorf("the dog at %d: hurt %v", m.p.add.X, hurt)
		}
	}
}

// Berserk raised as it runs speeds her up at once: at its second level it adds 3 to the
// attack speed, where it added 2, and she is told so.
func TestBerserkRaisedAsItRunsSpeedsHerUpAtOnce(t *testing.T) {
	s, client := warring(t, 1)
	s.svc.skills = &keptSkills{group: "body", learnt: s.svc.presence.all[1000].char.Skills}
	pl := s.svc.presence.all[1000]
	strike := pl.add.AttackSpeed
	if err := s.onUseSkill(used(skill.Berserk)); err != nil {
		t.Fatal(err)
	}
	drain(t, client)
	pl.char.Level = 11 // a point to raise it with

	if err := s.onCommand(context.Background(), "skillup 3"); err != nil {
		t.Fatal(err)
	}

	read(t, client, updateSize)
	if point, _, value := pointChange(t, client, 1000); point != protocol.PointAttackSpeed || value != int32(strike+3) {
		t.Fatalf("told point %d at %d, want attack speed %d", point, value, strike+3)
	}
	if got := s.svc.presence.attackSpeedOf(1000); got != strike+3 {
		t.Fatalf("blows timed at %d", got)
	}
}

// Taking up another group ends the buffs running: Berserk's speeds are gone with it.
func TestTakingUpAGroupEndsTheBuffsRunning(t *testing.T) {
	s, client := warring(t, 1)
	pl := s.svc.presence.all[1000]
	strike := pl.add.AttackSpeed
	if err := s.onUseSkill(used(skill.Berserk)); err != nil {
		t.Fatal(err)
	}
	drain(t, client)

	s.svc.presence.chooseGroup(1000, "mental")

	if len(pl.char.Buffs) != 0 || pl.add.AttackSpeed != strike || affectsOf(pl.char) != 0 {
		t.Fatalf("buffs %v, attack speed %d", pl.char.Buffs, pl.add.AttackSpeed)
	}
}
