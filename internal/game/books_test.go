package game

import (
	"context"
	"errors"
	"maps"
	"math"
	"math/rand/v2"
	"net"
	"slices"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/skill"
)

// always is a source of one number over and over: 1<<56 makes every roll of IntN 0, the
// best, and MaxUint64 the highest, the worst. 0 would not do: IntN rejects it and draws
// again, for ever.
type always uint64

func (a always) Uint64() uint64 { return uint64(a) }

var (
	lucky   = rand.New(always(1 << 56))        //nolint:gosec // a roll fixed for a test, no secret
	unlucky = rand.New(always(math.MaxUint64)) //nolint:gosec // the same
)

const swirlBook = 50462 // Instr. Smoczy Wir

// reading is Hendra, a Weaponry sura of level 40 with learnt as given and nothing waiting,
// carrying it at cell 0, at noon; her rolls come from roll.
func reading(t *testing.T, learnt map[account.Skill]uint8, it item.Item, roll *rand.Rand) (*session, *keptSkills, net.Conn) {
	t.Helper()
	s, kept, client, _, _ := training(t, account.Sura, 40, "weaponry", learnt)
	s.svc.now = func() time.Time { return noon }
	// The world record's own skills, apart from what the store keeps.
	s.playing.Skills, s.playing.SkillReads = maps.Clone(learnt), map[account.Skill]time.Time{}
	s.inventory, s.rand = item.Inventory{it}, roll
	return s, kept, client
}

func usedFirst(t *testing.T, s *session) {
	t.Helper()
	if err := s.onItemUse(context.Background(), []byte{1, 0, 0}); err != nil {
		t.Fatal(err)
	}
}

func levelsWith(sk account.Skill, level uint8) protocol.SkillLevels {
	var levels protocol.SkillLevels
	levels[skillVNums[sk]] = protocol.SkillLevel{Grade: skill.Grade(level), Level: level}
	return levels
}

// A book read raises a Master at M1 a level, to M2: kept with its next read 4 hours on, the
// book used up, and she is shown the book gone, her skills and a line.
func TestABookRaisesAMasterALevel(t *testing.T) {
	book := item.Item{ID: 5, VNum: swirlBook, Count: 1, Cell: 0}
	s, kept, client := reading(t, map[account.Skill]uint8{skill.DragonSwirl: skill.Master}, book, lucky)

	usedFirst(t, s)

	got := packets(t, drain(t, client))
	if !holds(got, itemCleared(book), protocol.EncodeSkillLevels(levelsWith(skill.DragonSwirl, 21)),
		infoLine("Udało się! Umiejętność osiągnęła poziom M2.")) {
		t.Fatalf("told %x", got)
	}
	if kept.learnt[skill.DragonSwirl] != 21 || !kept.reads[skill.DragonSwirl].Equal(noon.Add(4*time.Hour)) ||
		!slices.Equal(kept.used, []int64{5}) || len(s.inventory) != 0 ||
		s.svc.presence.all[1000].char.Skills[skill.DragonSwirl] != 21 {
		t.Fatalf("kept %v, %v, used %+v; holds %+v", kept.learnt, kept.reads, kept.used, s.inventory)
	}
}

// A read that fails uses the book up all the same, and the skill stays where it was, to be
// read again only 4 hours on.
func TestAFailedReadUsesTheBookUp(t *testing.T) {
	book := item.Item{ID: 5, VNum: swirlBook, Count: 1, Cell: 0}
	s, kept, client := reading(t, map[account.Skill]uint8{skill.DragonSwirl: 28}, book, unlucky)

	usedFirst(t, s)

	got := packets(t, drain(t, client))
	if len(got) != 2 || !holds(got, itemCleared(book), infoLine("Nie udało się. Księga przepadła.")) {
		t.Fatalf("told %x", got)
	}
	if kept.learnt[skill.DragonSwirl] != 28 || !kept.reads[skill.DragonSwirl].Equal(noon.Add(4*time.Hour)) || len(s.inventory) != 0 {
		t.Fatalf("kept %v, %v; holds %+v", kept.learnt, kept.reads, s.inventory)
	}
}

// A book is refused, and kept, for a skill below M1 or at M10, for a skill not hers, and
// while the skill's wait is not over, each with a line saying why.
func TestABookIsRefusedWhereItDoesNotTrain(t *testing.T) {
	cases := map[string]struct {
		learnt map[account.Skill]uint8
		book   uint32
		wait   time.Duration
		why    string
	}{
		"below M1":        {map[account.Skill]uint8{skill.DragonSwirl: 17}, swirlBook, 0, "Ta umiejętność musi najpierw zostać Mistrzem."},
		"at M10":          {map[account.Skill]uint8{skill.DragonSwirl: 29}, swirlBook, 0, "Tej umiejętności nie rozwiniesz już księgą."},
		"not learnt":      {map[account.Skill]uint8{}, swirlBook, 0, "Nie znasz tej umiejętności."},
		"another group":   {map[account.Skill]uint8{skill.DragonSwirl: 20}, 50476, 0, "Nie znasz tej umiejętności."},
		"still to wait":   {map[account.Skill]uint8{skill.DragonSwirl: 20}, swirlBook, 89*time.Minute + 20*time.Second, "Kolejną naukę możesz podjąć za 1 godz. 30 min."},
		"seconds to wait": {map[account.Skill]uint8{skill.DragonSwirl: 20}, swirlBook, 20 * time.Second, "Kolejną naukę możesz podjąć za 0 godz. 1 min."},
	}
	for name, c := range cases {
		book := item.Item{ID: 5, VNum: c.book, Count: 1, Cell: 0}
		s, kept, client := reading(t, c.learnt, book, lucky)
		s.playing.SkillReads[skill.DragonSwirl] = noon.Add(c.wait)

		usedFirst(t, s)

		if got := packets(t, drain(t, client)); len(got) != 1 || !holds(got, infoLine(c.why)) || len(kept.used) != 0 || len(s.inventory) != 1 {
			t.Errorf("%s: told %x, used %+v", name, got, kept.used)
		}
	}
}

// The eighteenth point makes a skill at 17 a Master, M1, at 20, for one point; one at 18
// or 19, raised before Masters came, too: for nothing, or giving a point back, as a Master
// took 18. A step that costs nothing is taken with no point free; at level 22 the point
// back only makes good the one a skill at 19 overspent, so none is shown.
func TestTheNextPointAfterSeventeenMakesAMaster(t *testing.T) {
	for _, c := range []struct {
		level, from uint8
		change      int32
	}{{40, 17, -1}, {40, 18, 0}, {40, 19, 1}, {22, 18, 0}, {22, 19, 0}} {
		s, kept, client, _, _ := training(t, account.Sura, c.level, "weaponry", map[account.Skill]uint8{skill.DragonSwirl: c.from})

		if err := s.onCommand(context.Background(), "skillup 62"); err != nil {
			t.Fatal(err)
		}

		left := int32(skill.Points(c.level, map[account.Skill]uint8{skill.DragonSwirl: 20})) //nolint:gosec // 18 points
		if got := packets(t, drain(t, client)); !holds(got, protocol.EncodeSkillLevels(levelsWith(skill.DragonSwirl, 20)),
			protocol.EncodePointChange(1000, protocol.PointSkillPoints, c.change, left)) || kept.learnt[skill.DragonSwirl] != 20 {
			t.Errorf("level %d from %d: told %x, kept %v", c.level, c.from, got, kept.learnt)
		}
	}
}

// soulsAt is Hendra with a soul stone at cell 0 and her Weaponry at M1, M10, G2 and G1,
// the last still waiting.
func soulsAt(t *testing.T, roll *rand.Rand) (*session, *keptSkills, net.Conn) {
	t.Helper()
	stone := item.Item{ID: 6, VNum: item.SoulStone, Count: 1, Cell: 0}
	s, kept, client := reading(t, map[account.Skill]uint8{skill.FingerStrike: 20, skill.DragonSwirl: 29,
		skill.EnchantedBlade: 31, skill.EnchantedArmour: 30}, stone, roll)
	s.playing.SkillReads[skill.EnchantedArmour] = noon.Add(time.Hour)
	return s, kept, client
}

// A soul stone asks which skill to train, of those it raises now, in the skill window's
// order, with a cancel last; the one picked rises, the stone used up.
func TestASoulStoneTrainsTheSkillPicked(t *testing.T) {
	s, kept, client := soulsAt(t, lucky)

	usedFirst(t, s)
	expectSaid(t, client, "Którą umiejętność chcesz ćwiczyć z Kamieniem Duchowym?[ENTER][QUESTION 1;Smoczy Wir (M10)|2;Czarowane Ostrze (G2)|3;Anuluj]")
	if err := s.onScriptAnswer(context.Background(), answered(1)); err != nil {
		t.Fatal(err)
	}

	got := packets(t, drain(t, client))
	if !holds(got, itemCleared(item.Item{Cell: 0}), infoLine("Udało się! Umiejętność osiągnęła poziom G3.")) ||
		kept.learnt[skill.EnchantedBlade] != 32 || kept.learnt[skill.DragonSwirl] != 29 || len(s.inventory) != 0 {
		t.Fatalf("told %x, kept %v", got, kept.learnt)
	}
}

// An M10 picked becomes a Grand Master, G1, on success.
func TestASoulStoneMakesAnM10AGrandMaster(t *testing.T) {
	s, kept, client := soulsAt(t, lucky)
	usedFirst(t, s)
	drain(t, client)

	if err := s.onScriptAnswer(context.Background(), answered(0)); err != nil {
		t.Fatal(err)
	}

	got := packets(t, drain(t, client))
	shown := skillLevels(s.svc.presence.all[1000].char.Skills)
	if !holds(got, protocol.EncodeSkillLevels(shown), infoLine("Udało się! Umiejętność osiągnęła poziom G1.")) ||
		kept.learnt[skill.DragonSwirl] != skill.GrandMaster || shown[62] != (protocol.SkillLevel{Grade: skill.GrandMasterGrade, Level: 30}) {
		t.Fatalf("told %x, kept %v, shown %+v", got, kept.learnt, shown[62])
	}
}

// A soul stone moved to another cell after the question is still the one asked about.
func TestASoulStoneMovedIsStillUsed(t *testing.T) {
	s, kept, client := soulsAt(t, lucky)
	usedFirst(t, s)
	drain(t, client)
	s.inventory[0].Cell = 17

	if err := s.onScriptAnswer(context.Background(), answered(0)); err != nil {
		t.Fatal(err)
	}

	if got := packets(t, drain(t, client)); !holds(got, itemCleared(item.Item{Cell: 17})) || !slices.Equal(kept.used, []int64{6}) {
		t.Fatalf("told %x, used %v", got, kept.used)
	}
}

// A read the store cannot keep changes nothing: the book stays in the bag, and nothing is
// said, as the client moved nothing itself.
func TestAReadThatCannotBeKeptKeepsTheBook(t *testing.T) {
	book := item.Item{ID: 5, VNum: swirlBook, Count: 1, Cell: 0}
	s, kept, client := reading(t, map[account.Skill]uint8{skill.DragonSwirl: skill.Master}, book, lucky)
	kept.fail = errors.New("the database is gone")

	usedFirst(t, s)

	if got := drain(t, client); len(got) != 0 || len(s.inventory) != 1 || s.svc.presence.all[1000].char.Skills[skill.DragonSwirl] != skill.Master {
		t.Fatalf("told %x, holds %+v", got, s.inventory)
	}
}

// Cancelling the soul stone's question, or picking after the stone left the bag, uses
// nothing.
func TestASoulStoneNotUsedIsKept(t *testing.T) {
	for name, before := range map[string]func(s *session){
		"cancelled":     func(*session) {},
		"stone dropped": func(s *session) { s.inventory = nil },
	} {
		s, kept, client := soulsAt(t, lucky)
		usedFirst(t, s)
		drain(t, client)
		before(s)
		answer := uint8(2)
		if name == "stone dropped" {
			answer = 0
		}

		if err := s.onScriptAnswer(context.Background(), answered(answer)); err != nil {
			t.Fatal(err)
		}

		if got := drain(t, client); len(got) != 0 || len(kept.used) != 0 {
			t.Errorf("%s: told %x, used %+v", name, got, kept.used)
		}
	}
}

// With no skill a soul stone raises, she is told it takes an M10; with only some still
// waiting, how long until the soonest is ready.
func TestASoulStoneWithNothingReadySaysWhy(t *testing.T) {
	stone := item.Item{ID: 6, VNum: item.SoulStone, Count: 1, Cell: 0}
	s, _, client := reading(t, map[account.Skill]uint8{skill.DragonSwirl: 28}, stone, lucky)
	usedFirst(t, s)
	if got := packets(t, drain(t, client)); !holds(got, infoLine("Żadna twoja umiejętność nie jest gotowa na Kamień Duchowy: musi być co najmniej M10.")) {
		t.Fatalf("told %x", got)
	}

	s, _, client = reading(t, map[account.Skill]uint8{skill.DragonSwirl: 29, skill.EnchantedBlade: 33}, stone, lucky)
	s.playing.SkillReads[skill.DragonSwirl] = noon.Add(3 * time.Hour)
	s.playing.SkillReads[skill.EnchantedBlade] = noon.Add(2 * time.Hour)
	usedFirst(t, s)
	if got := packets(t, drain(t, client)); !holds(got, infoLine("Kolejną naukę możesz podjąć za 2 godz. 0 min.")) || s.talking != nil {
		t.Fatalf("told %x", got)
	}
}
