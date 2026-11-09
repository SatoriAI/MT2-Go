package game

import (
	"bytes"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/skill"
)

// archering is archerAmong, 1500 east of the dogs, on Archery with every one of its skills
// at level and mana to spare.
func archering(t *testing.T, dogs int, level uint8) *session {
	t.Helper()
	s, _, client, _ := archerAmong(t, 1500, 100, dogs)
	c := s.svc.presence.all[1000].char
	c.SkillGroup, c.SP = "archery", 1000
	c.Skills = map[account.Skill]uint8{skill.RepetitiveShot: level, skill.ArrowShower: level, skill.FireArrow: level,
		skill.FeatherWalk: level, skill.PoisonArrow: level}
	drain(t, client)
	return s
}

func dogVIDs(s *session) []uint32 {
	vids := make([]uint32, len(s.svc.monsters))
	for i, m := range s.svc.monsters {
		vids[i] = m.p.add.VID
	}
	return vids
}

// loosed lets the skill's next arrow go.
func loosed(t *testing.T, s *session, sk account.Skill) {
	t.Helper()
	if err := s.onShoot([]byte{skillVNums[sk]}); err != nil {
		t.Fatal(err)
	}
}

// arrowsLeft is how many arrows she wears.
func arrowsLeft(s *session) uint8 {
	arrows, _ := s.inventory.WornIn(item.Arrow)
	return arrows.Count
}

// hurtEach is what each dog has taken from her.
func hurtEach(s *session) []int {
	pl := s.svc.presence.all[1000]
	took := make([]int, len(s.svc.monsters))
	for i, m := range s.svc.monsters {
		took[i] = m.p.fight.damageBy[pl]
	}
	return took
}

// Repetitive Shot at its first level costs 46 mana and lets 2 + 6k = 2 arrows go at the dog
// she picked, each landing as the client shoots it and using an arrow up; a third is
// dropped.
func TestRepetitiveShotLetsItsArrowsGoAtTheOnePicked(t *testing.T) {
	s := archering(t, 1, 1)
	dog := dogVIDs(s)[0]

	if err := s.onUseSkill(usedOn(skill.RepetitiveShot, dog)); err != nil {
		t.Fatal(err)
	}
	if sp := s.svc.presence.all[1000].char.SP; sp != 1000-46 || hurt(s) != 0 {
		t.Fatalf("SP %d, the dog took %d before an arrow flew", sp, hurt(s))
	}
	loosed(t, s, skill.RepetitiveShot)
	first := hurt(s)
	loosed(t, s, skill.RepetitiveShot)
	second := hurt(s) - first
	loosed(t, s, skill.RepetitiveShot)

	if first == 0 || second == 0 || hurt(s) != first+second || arrowsLeft(s) != 98 {
		t.Fatalf("arrows took %d and %d, %d in all, %d arrows left", first, second, hurt(s), arrowsLeft(s))
	}
}

// Repetitive Shot's arrows share its 100 + 20 for each arrow + 80k percent: at its first
// level two arrows of (104 + 40) / 2 = 72 percent each, at 20, five of (140 + 100) / 5 = 48.
func TestRepetitiveShotsArrowsShareItsDamage(t *testing.T) {
	proto, _ := skill.Lookup(skill.RepetitiveShot)
	for level, want := range map[uint8]volley{1: {left: 2, percent: 72}, 20: {left: 5, percent: 48}} {
		if v := loose(skill.RepetitiveShot, proto, level, 7, nil, noon); v.left != want.left || v.percent != want.percent {
			t.Errorf("at %d: %d arrows of %d percent", level, v.left, v.percent)
		}
	}
}

// Arrow Shower lets an arrow go at the dog she picked and one at each further dog her client
// aimed at, in turn, up to its 2 + 6k arrows: two at the first level, the third dog spared.
func TestArrowShowerSendsEachArrowAtTheNextAimedAt(t *testing.T) {
	s := archering(t, 3, 1)
	dogs := dogVIDs(s)
	for _, d := range []uint32{dogs[1], dogs[0], dogs[1], dogs[2]} {
		if err := s.onAddFlyTarget(flyAt(d, homeX, homeY)); err != nil {
			t.Fatal(err)
		}
	}

	if err := s.onUseSkill(usedOn(skill.ArrowShower, dogs[0])); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		loosed(t, s, skill.ArrowShower)
	}

	if took := hurtEach(s); took[0] == 0 || took[1] == 0 || took[2] != 0 || arrowsLeft(s) != 98 {
		t.Fatalf("the dogs took %v, %d arrows left", took, arrowsLeft(s))
	}
}

// Fire Arrow lets one arrow go, which strikes the dog she picked and the others within 300
// of it for 60 in a hundred of its damage, each less than the one picked takes.
func TestFireArrowSplashesTheOthersForLess(t *testing.T) {
	s := archering(t, 3, 1)
	dogs := dogVIDs(s)

	if err := s.onUseSkill(usedOn(skill.FireArrow, dogs[0])); err != nil {
		t.Fatal(err)
	}
	loosed(t, s, skill.FireArrow)
	loosed(t, s, skill.FireArrow)

	took := hurtEach(s)
	if took[1] == 0 || took[2] == 0 || took[1] >= took[0] || took[2] >= took[0] || arrowsLeft(s) != 99 {
		t.Fatalf("the dogs took %v, %d arrows left", took, arrowsLeft(s))
	}
}

// Poison Arrow at level 20 poisons some of the dogs it strikes, at its 40 percent.
func TestPoisonArrowPoisonsSome(t *testing.T) {
	s := archering(t, 12, 20)

	if err := s.onUseSkill(usedOn(skill.PoisonArrow, dogVIDs(s)[0])); err != nil {
		t.Fatal(err)
	}
	loosed(t, s, skill.PoisonArrow)

	poisoned := 0
	for _, m := range s.svc.monsters {
		if m.p.fight.poisoned {
			poisoned++
		}
	}
	if poisoned == 0 || poisoned == 12 {
		t.Fatalf("%d of 12 poisoned", poisoned)
	}
}

// An arrow skill's arrows are dropped, using no arrow, when it was not used, when the
// other skill was, or once 4 s have passed since.
func TestASkillsArrowsAreDroppedOutsideItsVolley(t *testing.T) {
	cases := map[string]func(t *testing.T, s *session){
		"not used": func(t *testing.T, s *session) { loosed(t, s, skill.FireArrow) },
		"another used": func(t *testing.T, s *session) {
			if err := s.onUseSkill(usedOn(skill.PoisonArrow, dogVIDs(s)[0])); err != nil {
				t.Fatal(err)
			}
			loosed(t, s, skill.FireArrow)
		},
		"too late": func(t *testing.T, s *session) {
			if err := s.onUseSkill(usedOn(skill.FireArrow, dogVIDs(s)[0])); err != nil {
				t.Fatal(err)
			}
			s.svc.now = func() time.Time { return noon.Add(4*time.Second + time.Millisecond) }
			loosed(t, s, skill.FireArrow)
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			s := archering(t, 1, 1)
			c(t, s)
			if hurt(s) != 0 || arrowsLeft(s) != 100 {
				t.Fatalf("the dog took %d, %d arrows left", hurt(s), arrowsLeft(s))
			}
		})
	}
}

// An arrow skill is not used, and costs nothing, without arrows worn or a bow in hand.
func TestAnArrowSkillNeedsArrowsAndABow(t *testing.T) {
	for name, change := range map[string]func(s *session){
		"no arrows": func(s *session) { s.inventory = s.inventory[:1]; s.svc.presence.all[1000].char.Arrow = 0 },
		"no bow":    func(s *session) { s.svc.presence.all[1000].char.Weapon = 1000 },
	} {
		s := archering(t, 1, 1)
		change(s)
		if err := s.onUseSkill(usedOn(skill.FireArrow, dogVIDs(s)[0])); err != nil {
			t.Fatal(err)
		}
		loosed(t, s, skill.FireArrow)
		if sp := s.svc.presence.all[1000].char.SP; sp != 1000 || hurt(s) != 0 {
			t.Fatalf("%s: SP %d, the dog took %d", name, sp, hurt(s))
		}
	}
}

// Feather Walk adds 60k to her move speed, 3 at its first level, with the client's
// affect bit 17 shown to her, for 15 + 30k seconds.
func TestFeatherWalkQuickensHerStep(t *testing.T) {
	s, _, client, _ := archerAmong(t, 1500, 100, 1)
	c := s.svc.presence.all[1000].char
	c.SkillGroup, c.SP, c.Skills = "archery", 1000, map[account.Skill]uint8{skill.FeatherWalk: 1}
	before := moveSpeed(c)
	drain(t, client)

	if err := s.onUseSkill(used(skill.FeatherWalk)); err != nil {
		t.Fatal(err)
	}

	pl := s.svc.presence.all[1000]
	if got := drain(t, client); pl.add.MoveSpeed != before+3 || affectsOf(c)&(1<<17) == 0 ||
		!bytes.Contains(got, protocol.EncodePointChange(1000, protocol.PointMoveSpeed, 0, int32(before+3))) {
		t.Fatalf("speed %d from %d, affects %x, told %x", pl.add.MoveSpeed, before, affectsOf(c), got)
	}
	if until := c.Buffs[skill.FeatherWalk]; !until.Equal(noon.Add(16 * time.Second)) {
		t.Fatalf("runs until %v", until)
	}
}

// At level 20 Arrow Shower's five arrows go once at each dog aimed at, the one aimed at
// twice counted once: three dogs, three arrows.
func TestArrowShowerShootsEachDogAimedAtOnce(t *testing.T) {
	s := archering(t, 3, 20)
	dogs := dogVIDs(s)
	for _, d := range []uint32{dogs[1], dogs[0], dogs[1], dogs[2]} {
		if err := s.onAddFlyTarget(flyAt(d, homeX, homeY)); err != nil {
			t.Fatal(err)
		}
	}

	if err := s.onUseSkill(usedOn(skill.ArrowShower, dogs[0])); err != nil {
		t.Fatal(err)
	}
	for range 5 {
		loosed(t, s, skill.ArrowShower)
	}

	if took := hurtEach(s); took[0] == 0 || took[1] == 0 || took[2] == 0 || arrowsLeft(s) != 97 {
		t.Fatalf("the dogs took %v, %d arrows left", took, arrowsLeft(s))
	}
}

// A point on the ground the client aimed an arrow at is no target: Arrow Shower's second
// arrow goes at the dog aimed at after it.
func TestArrowShowerPassesOverPointsOnTheGround(t *testing.T) {
	s := archering(t, 2, 1)
	dogs := dogVIDs(s)
	for _, d := range []uint32{0, dogs[1]} {
		if err := s.onAddFlyTarget(flyAt(d, homeX, homeY)); err != nil {
			t.Fatal(err)
		}
	}

	if err := s.onUseSkill(usedOn(skill.ArrowShower, dogs[0])); err != nil {
		t.Fatal(err)
	}
	loosed(t, s, skill.ArrowShower)
	loosed(t, s, skill.ArrowShower)

	if took := hurtEach(s); took[0] == 0 || took[1] == 0 {
		t.Fatalf("the dogs took %v", took)
	}
}

// The dogs aimed at for one Arrow Shower are not aimed at by the next: with none aimed at
// anew, it goes at the one picked alone.
func TestArrowShowersAimsAreNotCarriedOver(t *testing.T) {
	s := archering(t, 2, 1)
	dogs := dogVIDs(s)
	if err := s.onAddFlyTarget(flyAt(dogs[1], homeX, homeY)); err != nil {
		t.Fatal(err)
	}
	if err := s.onUseSkill(usedOn(skill.ArrowShower, dogs[0])); err != nil {
		t.Fatal(err)
	}
	s.svc.now = func() time.Time { return noon.Add(20 * time.Second) }
	if err := s.onUseSkill(usedOn(skill.ArrowShower, dogs[0])); err != nil {
		t.Fatal(err)
	}

	loosed(t, s, skill.ArrowShower)
	loosed(t, s, skill.ArrowShower)

	if took := hurtEach(s); took[0] == 0 || took[1] != 0 || arrowsLeft(s) != 99 {
		t.Fatalf("the dogs took %v, %d arrows left", took, arrowsLeft(s))
	}
}

// An arrow skill's arrow lands only with arrows still worn and the bow still in hand when
// it is let go.
func TestASkillsArrowNeedsArrowsAndABowWhenShot(t *testing.T) {
	for name, change := range map[string]func(s *session){
		"arrows taken off": func(s *session) { s.inventory = s.inventory[:1]; s.svc.presence.all[1000].char.Arrow = 0 },
		"a dagger taken":   func(s *session) { s.svc.presence.all[1000].char.Weapon = 1000 },
	} {
		s := archering(t, 1, 1)
		if err := s.onUseSkill(usedOn(skill.FireArrow, dogVIDs(s)[0])); err != nil {
			t.Fatal(err)
		}
		change(s)
		loosed(t, s, skill.FireArrow)
		if hurt(s) != 0 {
			t.Fatalf("%s: the dog took %d", name, hurt(s))
		}
	}
}

// Dogs aimed at for an Arrow Shower that was refused, too soon after the last, are not
// aimed at by the next: its arrows go at the dogs aimed at for it.
func TestARefusedArrowShowersAimsAreForgotten(t *testing.T) {
	s := archering(t, 3, 1)
	dogs := dogVIDs(s)
	s.svc.cooldowns.used(s.playing.ID, skill.ArrowShower, noon.Add(time.Second))
	for range maxFlyAdds {
		if err := s.onAddFlyTarget(flyAt(dogs[1], homeX, homeY)); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.onUseSkill(usedOn(skill.ArrowShower, dogs[0])); err != nil {
		t.Fatal(err)
	}
	s.svc.now = func() time.Time { return noon.Add(2 * time.Second) }
	if err := s.onAddFlyTarget(flyAt(dogs[2], homeX, homeY)); err != nil {
		t.Fatal(err)
	}

	if err := s.onUseSkill(usedOn(skill.ArrowShower, dogs[0])); err != nil {
		t.Fatal(err)
	}
	loosed(t, s, skill.ArrowShower)
	loosed(t, s, skill.ArrowShower)

	if took := hurtEach(s); took[0] == 0 || took[1] != 0 || took[2] == 0 {
		t.Fatalf("the dogs took %v", took)
	}
}

// Change character forgets what the archer aimed at and the arrows her skill had left, so
// the next character cannot let them go.
func TestChangeCharacterForgetsTheVolley(t *testing.T) {
	s := archering(t, 2, 20)
	dogs := dogVIDs(s)
	if err := s.onAddFlyTarget(flyAt(dogs[1], homeX, homeY)); err != nil {
		t.Fatal(err)
	}
	if err := s.onUseSkill(usedOn(skill.RepetitiveShot, dogs[0])); err != nil {
		t.Fatal(err)
	}
	if err := s.onFlyTarget(flyAt(dogs[0], homeX, homeY)); err != nil {
		t.Fatal(err)
	}
	if err := s.onAddFlyTarget(flyAt(dogs[1], homeX, homeY)); err != nil {
		t.Fatal(err)
	}
	s.phase, s.claim = protocol.PhaseLoading, &claim{character: s.playing.ID, done: make(chan struct{})}

	s.leaveWorld()

	if s.volley.left != 0 || s.flyAdds != nil || s.flyTarget != 0 {
		t.Fatalf("left %d arrows, aims %v, target %d", s.volley.left, s.flyAdds, s.flyTarget)
	}
}

// A plain shot waits its gap after a skill's arrow: one at once after Fire Arrow's is
// dropped.
func TestAPlainShotWaitsAfterASkillsArrow(t *testing.T) {
	s := archering(t, 1, 1)
	dog := dogVIDs(s)[0]
	if err := s.onUseSkill(usedOn(skill.FireArrow, dog)); err != nil {
		t.Fatal(err)
	}
	loosed(t, s, skill.FireArrow)
	took := hurt(s)

	shot(t, s, 0)

	if hurt(s) != took || arrowsLeft(s) != 99 {
		t.Fatalf("the dog took %d, then %d; %d arrows left", took, hurt(s), arrowsLeft(s))
	}
}
