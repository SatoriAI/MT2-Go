package game

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/network"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/skill"
	"github.com/SatoriAI/MT2-Go/internal/world"
)

// keptSkills stands in for the store's skills, and fails every write with fail set.
type keptSkills struct {
	group  account.SkillGroup
	learnt map[account.Skill]uint8
	reads  map[account.Skill]time.Time
	used   []int64 // the book or stone each training used up
	spent  account.Aids
	aids   account.Aids // the aids held, which TakeAid adds to
	left   []item.Item  // what TakeAid left of each item used
	fail   error
}

func (k *keptSkills) Learnt(context.Context, uint32) (map[account.Skill]uint8, map[account.Skill]time.Time, error) {
	return k.learnt, k.reads, k.fail
}

func (k *keptSkills) Train(_ context.Context, _ uint32, r account.Read) error {
	if k.fail != nil {
		return k.fail
	}
	if k.learnt[r.Skill] != r.From {
		return errors.New("trained from another level")
	}
	if k.reads == nil {
		k.reads = map[account.Skill]time.Time{}
	}
	k.learnt[r.Skill], k.reads[r.Skill] = r.To, r.Next
	k.used = append(k.used, r.Used)
	k.spent = append(k.spent, r.Spent...)
	return nil
}

func (k *keptSkills) TakeAid(_ context.Context, _ uint32, aid account.Aid, left item.Item) error {
	if k.fail != nil {
		return k.fail
	}
	if k.aids.Has(aid) {
		return account.ErrAidHeld
	}
	k.aids, k.left = append(k.aids, aid), append(k.left, left)
	return nil
}

func (k *keptSkills) Choose(_ context.Context, _ uint32, group account.SkillGroup) error {
	if k.fail == nil {
		k.group, k.learnt = group, map[account.Skill]uint8{}
	}
	return k.fail
}

func (k *keptSkills) Raise(_ context.Context, _ uint32, s account.Skill, level uint8) error {
	if k.fail == nil {
		k.learnt[s] = level
	}
	return k.fail
}

// training is Hendra, a sura of level and skill group, learnt as given, beside the
// Weaponry teacher (20304) and with the Black Magic one (20305) beside him; it returns
// both teachers' vids.
func training(t *testing.T, class account.Class, level uint8, group account.SkillGroup,
	learnt map[account.Skill]uint8) (*session, *keptSkills, net.Conn, uint32, uint32) {
	t.Helper()
	s := worldWith(t, world.Spawn{VNum: 20304, X: homeX, Y: homeY, Count: 1}, world.Spawn{VNum: 20305, X: homeX + 100, Y: homeY, Count: 1})
	if learnt == nil {
		learnt = map[account.Skill]uint8{}
	}
	kept := &keptSkills{group: group, learnt: learnt}
	s.skills = kept
	client, hendra := aPlayer(t, s, 1000, 7, "Hendra", 100)
	hendra.char.Class, hendra.char.Level, hendra.char.SkillGroup, hendra.char.Skills = class, level, group, learnt
	read(t, client, 2*(addSize+infoSize)) // the two teachers
	sess := &session{svc: s, conn: hendra.out.(*network.Conn), phase: protocol.PhaseGame,
		playing: playingAs(hendra)}
	return sess, kept, client, 1, 2
}

// expectSaid reads an NPC's dialog of exactly text.
func expectSaid(t *testing.T, c net.Conn, text string) {
	t.Helper()
	want := script(text)
	if got := read(t, c, len(want)); string(got) != string(want) {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}

func answered(button uint8) []byte { return []byte{button} }

// A sura without a group is offered the teacher's path, asked whether she is sure, and on
// yes takes it: kept, then the client told the group, the levels after it, and her points.
func TestASuraTakesUpTheTeachersGroup(t *testing.T) {
	s, kept, client, weaponry, _ := training(t, account.Sura, 6, "", nil)

	if err := s.onClick(vidBody(weaponry)); err != nil {
		t.Fatal(err)
	}
	expectSaid(t, client, "Mogę nauczyć cię drogi Mocy Broni. Czy chcesz nią podążać?[ENTER][QUESTION 1;Tak|2;Nie]")
	if err := s.onScriptAnswer(context.Background(), answered(0)); err != nil {
		t.Fatal(err)
	}
	expectSaid(t, client, "Czy na pewno? Wybór drogi Mocy Broni to poważna decyzja.[ENTER][QUESTION 1;Tak|2;Nie]")
	if err := s.onScriptAnswer(context.Background(), answered(0)); err != nil {
		t.Fatal(err)
	}

	want := string(protocol.EncodeChangeSkillGroup(1)) + string(protocol.EncodeSkillLevels(protocol.SkillLevels{})) +
		string(protocol.EncodePointChange(1000, protocol.PointSkillPoints, 0, 2)) +
		string(script("Od dziś podążasz drogą Mocy Broni.[ENTER][DONE]"))
	if got := read(t, client, len(want)); string(got) != want {
		t.Fatalf("got  %x\nwant %x", got, want)
	}
	if kept.group != "weaponry" || s.svc.record.state.all[1000].char.SkillGroup != "weaponry" || s.talking != nil {
		t.Fatalf("kept %q, in play %q", kept.group, s.svc.record.state.all[1000].char.SkillGroup)
	}
}

// No, the escape key, or an answer to nothing asked ends the talk and takes nothing up.
func TestATeachersOfferDeclinedTakesNothingUp(t *testing.T) {
	for name, answers := range map[string][]uint8{
		"no to the offer":        {1},
		"no when asked if sure":  {0, 1},
		"the dialog's own close": {254},
		"an answer to nothing":   {},
	} {
		s, kept, client, weaponry, _ := training(t, account.Sura, 6, "", nil)
		if name != "an answer to nothing" {
			if err := s.onClick(vidBody(weaponry)); err != nil {
				t.Fatal(err)
			}
			read(t, client, len(script("Mogę nauczyć cię drogi Mocy Broni. Czy chcesz nią podążać?[ENTER][QUESTION 1;Tak|2;Nie]")))
		}
		for i, a := range answers {
			if i == 1 {
				read(t, client, len(script("Czy na pewno? Wybór drogi Mocy Broni to poważna decyzja.[ENTER][QUESTION 1;Tak|2;Nie]")))
			}
			if err := s.onScriptAnswer(context.Background(), answered(a)); err != nil {
				t.Fatal(err)
			}
		}
		if err := s.onScriptAnswer(context.Background(), answered(0)); err != nil { // a stale yes
			t.Fatal(err)
		}

		expectNothing(t, client)
		if kept.group != "" || s.svc.record.state.all[1000].char.SkillGroup != "" || s.talking != nil {
			t.Errorf("%s: took up %q", name, kept.group)
		}
	}
}

// A teacher tells, and offers nothing to, the wrong class, the too young, one already on
// their path, and one of the other path from level 20.
func TestATeacherTellsWhyNot(t *testing.T) {
	cases := []struct {
		name  string
		class account.Class
		level uint8
		group account.SkillGroup
		said  string
	}{
		{"a warrior", account.Warrior, 10, "", "Nie uczę twojej klasy."},
		{"below level 5", account.Sura, 4, "", "Wróć, gdy osiągniesz 5 poziom."},
		{"already Weaponry", account.Sura, 10, "weaponry", "Ćwicz dalej, a twoja droga Mocy Broni zaprowadzi cię daleko."},
		{"Black Magic at 20", account.Sura, 20, "black_magic", "Twoja droga jest już wybrana."},
	}
	for _, c := range cases {
		s, _, client, weaponry, _ := training(t, c.class, c.level, c.group, nil)

		if err := s.onClick(vidBody(weaponry)); err != nil {
			t.Fatal(err)
		}

		expectSaid(t, client, c.said+"[ENTER][DONE]")
		if s.talking != nil {
			t.Errorf("%s: asked something", c.name)
		}
	}
}

// One of the other path below level 20 may change, forgetting what she learnt and getting
// every point back.
func TestASuraBelowTwentyChangesPathAndGetsHerPointsBack(t *testing.T) {
	s, kept, client, _, blackMagic := training(t, account.Sura, 10, "weaponry", map[account.Skill]uint8{skill.DragonSwirl: 4})

	if err := s.onClick(vidBody(blackMagic)); err != nil {
		t.Fatal(err)
	}
	expectSaid(t, client, "Chcesz porzucić swoją drogę i uczyć się drogi Czarnej Magii? Zapomnisz to, czego się nauczyłeś, ale odzyskasz wszystkie punkty.[ENTER][QUESTION 1;Tak|2;Nie]")
	for range 2 {
		if err := s.onScriptAnswer(context.Background(), answered(0)); err != nil {
			t.Fatal(err)
		}
	}

	read(t, client, len(script("Czy na pewno? Wybór drogi Czarnej Magii to poważna decyzja.[ENTER][QUESTION 1;Tak|2;Nie]")))
	if got := read(t, client, 2+1531+pointChangeSize); string(got[2+1531:]) != string(protocol.EncodePointChange(1000, protocol.PointSkillPoints, 0, 6)) ||
		got[1] != 2 {
		t.Fatalf("told group %d, points %x", got[1], got[2+1531:])
	}
	if c := s.svc.record.state.all[1000].char; kept.group != "black_magic" || len(c.Skills) != 0 || len(kept.learnt) != 0 {
		t.Fatalf("kept %q %v, in play %v", kept.group, kept.learnt, c.Skills)
	}
}

// A group that cannot be kept is not taken up.
func TestAGroupThatCannotBeKeptIsNotTakenUp(t *testing.T) {
	s, kept, client, weaponry, _ := training(t, account.Sura, 6, "", nil)
	kept.fail = errors.New("the database is gone")
	if err := s.onClick(vidBody(weaponry)); err != nil {
		t.Fatal(err)
	}
	read(t, client, len(script("Mogę nauczyć cię drogi Mocy Broni. Czy chcesz nią podążać?[ENTER][QUESTION 1;Tak|2;Nie]")))
	if err := s.onScriptAnswer(context.Background(), answered(0)); err != nil {
		t.Fatal(err)
	}
	read(t, client, len(script("Czy na pewno? Wybór drogi Mocy Broni to poważna decyzja.[ENTER][QUESTION 1;Tak|2;Nie]")))

	if err := s.onScriptAnswer(context.Background(), answered(0)); err != nil {
		t.Fatal(err)
	}

	expectNothing(t, client)
	if s.svc.record.state.all[1000].char.SkillGroup != "" {
		t.Fatal("taken up in play")
	}
}

// The + button raises a skill of her path a level, kept, and she is shown the levels and
// the points left.
func TestASkillIsRaisedWithAPoint(t *testing.T) {
	s, kept, client, _, _ := training(t, account.Sura, 6, "weaponry", map[account.Skill]uint8{skill.DragonSwirl: 1})

	if err := s.onCommand(context.Background(), "skillup 62"); err != nil {
		t.Fatal(err)
	}

	var levels protocol.SkillLevels
	levels[62] = protocol.SkillLevel{Level: 2}
	want := string(protocol.EncodeSkillLevels(levels)) + string(protocol.EncodePointChange(1000, protocol.PointSkillPoints, -1, 0))
	if got := read(t, client, len(want)); string(got) != want || kept.learnt[skill.DragonSwirl] != 2 ||
		s.svc.record.state.all[1000].char.Skills[skill.DragonSwirl] != 2 {
		t.Fatalf("got %x, kept %v", got[len(got)-pointChangeSize:], kept.learnt)
	}
}

// A skill is not raised with no point free, at its last level, of another group, with no
// group, or if it cannot be kept; one the game does not have yet is not raised, and she is
// told.
func TestASkillThatCannotBeRaisedIsNot(t *testing.T) {
	cases := []struct {
		name   string
		level  uint8
		group  account.SkillGroup
		learnt map[account.Skill]uint8
		fail   bool
	}{
		{"no point free", 6, "weaponry", map[account.Skill]uint8{skill.DragonSwirl: 2}, false},
		{"at its last level", 30, "weaponry", map[account.Skill]uint8{skill.DragonSwirl: skill.Master}, false},
		{"of the other group", 10, "black_magic", nil, false},
		{"with no group", 10, "", nil, false},
		{"not kept", 10, "weaponry", nil, true},
	}
	for _, c := range cases {
		s, kept, client, _, _ := training(t, account.Sura, c.level, c.group, c.learnt)
		if c.fail {
			kept.fail = errors.New("the database is gone")
		}
		before := s.svc.record.state.all[1000].char.Skills[skill.DragonSwirl]

		if err := s.onCommand(context.Background(), "skillup 62"); err != nil {
			t.Fatal(err)
		}

		expectNothing(t, client)
		if s.svc.record.state.all[1000].char.Skills[skill.DragonSwirl] != before {
			t.Errorf("%s: raised", c.name)
		}
	}

	s, _, client, _, _ := training(t, account.Sura, 10, "weaponry", nil)
	if err := s.onCommand(context.Background(), "skillup 64"); err != nil {
		t.Fatal(err)
	}
	line := textToClient("Tej umiejętności nie można jeszcze rozwijać.")
	if got := read(t, client, chatHeaderSize+len(line)+1); string(got) != string(protocol.EncodeChat(protocol.ChatInfo, 0, 0, line)) {
		t.Fatalf("told %q", got)
	}
}

// A level up past 5 with a group tells her her new skill points; without one, it does not.
func TestALevelUpTellsTheSkillPointsOfOneWithAGroup(t *testing.T) {
	s, _, client, _, _ := training(t, account.Sura, 6, "weaponry", nil)
	p := s.svc.record.state.all[1000]
	var level uint8
	s.svc.record.held(func(l locked) {
		l.gainExp(p, 1_000_000)
		level = p.char.Level
	})

	got := read(t, client, 10*pointChangeSize) // the experience, eight points of the level, then the skill points
	want := protocol.EncodePointChange(1000, protocol.PointSkillPoints, 0, int32(level)-skill.FirstLevel+1)
	if string(got[9*pointChangeSize:]) != string(want) {
		t.Fatalf("got %x\nwant %x", got[9*pointChangeSize:], want)
	}
}

// Without a group, a level up says nothing of skill points.
func TestALevelUpWithoutAGroupSaysNothingOfSkillPoints(t *testing.T) {
	s, _, client, _, _ := training(t, account.Sura, 6, "", nil)
	p := s.svc.record.state.all[1000]
	s.svc.record.held(func(l locked) {
		l.gainExp(p, 1_000_000)
	})

	read(t, client, 9*pointChangeSize)
	expectNothing(t, client)
}

// The skill window shows the points of a character that has chosen a group, and none
// before, as they are for its skills.
func TestSkillPointsAreShownOnceAGroupIsChosen(t *testing.T) {
	c := &character{Character: account.Character{Class: account.Sura, Level: 10}}
	if got := pointsOf(c, 0).SkillPoints; got != 0 {
		t.Fatalf("no group: %d points", got)
	}
	c.SkillGroup = "weaponry"
	if got := pointsOf(c, 0).SkillPoints; got != 6 {
		t.Fatalf("Weaponry at 10: %d points", got)
	}
}

// A sura is offered a path from level 5 exactly, and a change of path up to level 19.
func TestATeacherOffersFromLevelFiveAndAChangeUpToNineteen(t *testing.T) {
	young, _, youngClient, weaponry, _ := training(t, account.Sura, 5, "", nil)
	if err := young.onClick(vidBody(weaponry)); err != nil {
		t.Fatal(err)
	}
	expectSaid(t, youngClient, "Mogę nauczyć cię drogi Mocy Broni. Czy chcesz nią podążać?[ENTER][QUESTION 1;Tak|2;Nie]")

	older, _, olderClient, _, blackMagic := training(t, account.Sura, 19, "weaponry", nil)
	if err := older.onClick(vidBody(blackMagic)); err != nil {
		t.Fatal(err)
	}
	expectSaid(t, olderClient, "Chcesz porzucić swoją drogę i uczyć się drogi Czarnej Magii? Zapomnisz to, czego się nauczyłeś, ale odzyskasz wszystkie punkty.[ENTER][QUESTION 1;Tak|2;Nie]")
}

// An offer kept open while she reached level 20 is not taken: her yes is told no, as a
// click would be now.
func TestAnOfferKeptOpenPastTwentyIsNotTaken(t *testing.T) {
	s, kept, client, _, blackMagic := training(t, account.Sura, 19, "weaponry", nil)
	if err := s.onClick(vidBody(blackMagic)); err != nil {
		t.Fatal(err)
	}
	read(t, client, len(script("Chcesz porzucić swoją drogę i uczyć się drogi Czarnej Magii? Zapomnisz to, czego się nauczyłeś, ale odzyskasz wszystkie punkty.[ENTER][QUESTION 1;Tak|2;Nie]")))
	s.svc.record.held(func(l locked) {
		s.svc.record.state.all[1000].char.Level = 20
	})

	if err := s.onScriptAnswer(context.Background(), answered(0)); err != nil {
		t.Fatal(err)
	}

	expectSaid(t, client, "Twoja droga jest już wybrana.[ENTER][DONE]")
	if kept.group != "weaponry" || s.talking != nil {
		t.Fatalf("kept %q", kept.group)
	}
}

// A yes from out of the teacher's reach takes nothing up.
func TestAYesFromOutOfReachTakesNothingUp(t *testing.T) {
	s, kept, client, weaponry, _ := training(t, account.Sura, 6, "", nil)
	if err := s.onClick(vidBody(weaponry)); err != nil {
		t.Fatal(err)
	}
	read(t, client, len(script("Mogę nauczyć cię drogi Mocy Broni. Czy chcesz nią podążać?[ENTER][QUESTION 1;Tak|2;Nie]")))
	s.svc.record.held(func(l locked) {
		s.svc.record.state.all[1000].add.X += 2 * npcReach
	})

	if err := s.onScriptAnswer(context.Background(), answered(0)); err != nil {
		t.Fatal(err)
	}

	expectNothing(t, client)
	if kept.group != "" || s.talking != nil {
		t.Fatalf("took up %q", kept.group)
	}
}

// The client is told a casting speed of 100, at which it counts a skill's cooldown as its
// table says: at 0 it would wait twice as long as this server.
func TestTheClientIsToldTheCastingSpeedCooldownsAreCountedAt(t *testing.T) {
	if got := pointsOf(&character{}, 0).CastingSpeed; got != 100 {
		t.Fatalf("casting speed %d", got)
	}
	if got := binary.LittleEndian.Uint32(protocol.EncodePoints(pointsOf(&character{}, 0))[1+4*protocol.PointCastingSpeed:]); got != 100 {
		t.Fatalf("sent %d at %d", got, protocol.PointCastingSpeed)
	}
}

// Enchanted Armour raised while it runs adds at its new level at once: she is shown its
// effect again and told her defence with it, (5 + 30) × 0.06 is 2, one more than at its
// first level, then the levels and points.
func TestTheArmourRaisedWhileItRunsTellsHerDefence(t *testing.T) {
	s, _, client, _, _ := training(t, account.Sura, 10, "weaponry", map[account.Skill]uint8{skill.EnchantedArmour: 1})
	c := s.svc.record.state.all[1000].char
	c.Intelligence = 5
	c.Buffs = map[account.Skill]time.Time{skill.EnchantedArmour: noon.Add(time.Minute)}
	before := defence(c)

	if err := s.onCommand(context.Background(), "skillup 65"); err != nil {
		t.Fatal(err)
	}

	read(t, client, updateSize)
	if got := read(t, client, pointChangeSize); string(got) != string(defenceTold(before+1)) {
		t.Fatalf("told %x, defence before %d", got, before)
	}
	read(t, client, 1531+pointChangeSize)
}

// A skill raised with no defence buff running tells no defence: the blade raised as it
// runs is only shown again.
func TestASkillRaisedTellsNoDefenceWithoutTheArmour(t *testing.T) {
	s, _, client, _, _ := training(t, account.Sura, 10, "weaponry", nil)
	s.svc.record.state.all[1000].char.Buffs = map[account.Skill]time.Time{skill.EnchantedBlade: noon.Add(time.Minute)}

	if err := s.onCommand(context.Background(), "skillup 63"); err != nil {
		t.Fatal(err)
	}

	read(t, client, updateSize+1531+pointChangeSize)
	expectNothing(t, client)
}
