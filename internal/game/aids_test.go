package game

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/skill"
)

// rolled is a generator whose first roll of 0 to 99 is r.
func rolled(r int) func(int) bool { return func(got int) bool { return got == r } }

// Rada Pustelnika held gives the next book read two and a half times its chance, never
// past certain: a roll of 45 raises an M9, at 20 in 100 alone, and one of 99 an M1, at 70.
// The read spends it, and its icon goes.
func TestRadaRaisesTheNextBookReadTwoAndAHalfTimes(t *testing.T) {
	for _, c := range []struct{ level, roll uint8 }{{28, 45}, {skill.Master, 99}} {
		book := item.Item{ID: 5, VNum: swirlBook, Count: 1, Cell: 0}
		s, kept, client := reading(t, map[account.Skill]uint8{skill.DragonSwirl: c.level}, book, rolling(rolled(int(c.roll))))
		s.playing.Aids = account.Aids{account.ReadBonus}

		usedFirst(t, s)

		got := packets(t, drain(t, client))
		if kept.learnt[skill.DragonSwirl] != c.level+1 || !slices.Equal(kept.spent, account.Aids{account.ReadBonus}) ||
			len(s.playing.Aids) != 0 || !holds(got, aidGone(account.ReadBonus)) {
			t.Errorf("at %d rolling %d: kept %d, spent %v, holds %v, told %x", c.level, c.roll, kept.learnt[skill.DragonSwirl], kept.spent, s.playing.Aids, got)
		}
	}
}

// A read Rada Pustelnika fails spends it all the same.
func TestRadaIsSpentOnAFailedRead(t *testing.T) {
	book := item.Item{ID: 5, VNum: swirlBook, Count: 1, Cell: 0}
	s, kept, _ := reading(t, map[account.Skill]uint8{skill.DragonSwirl: 28}, book, rolling(rolled(50)))
	s.playing.Aids = account.Aids{account.ReadBonus}

	usedFirst(t, s)

	if kept.learnt[skill.DragonSwirl] != 28 || !slices.Equal(kept.spent, account.Aids{account.ReadBonus}) || len(s.playing.Aids) != 0 {
		t.Fatalf("kept %d, spent %v, holds %v", kept.learnt[skill.DragonSwirl], kept.spent, s.playing.Aids)
	}
}

// Rada Pustelnika leaves a soul stone's chance as it is, and is kept: G2 at 40 in 100
// fails on a roll of 45.
func TestRadaLeavesASoulStoneAsItIs(t *testing.T) {
	s, kept, client := soulsAt(t, rolling(rolled(45)))
	s.playing.Aids = account.Aids{account.ReadBonus}
	usedFirst(t, s)
	drain(t, client)

	if err := s.onScriptAnswer(context.Background(), answered(1)); err != nil {
		t.Fatal(err)
	}

	if kept.learnt[skill.EnchantedBlade] != 31 || len(kept.spent) != 0 || !s.playing.Aids.Has(account.ReadBonus) {
		t.Fatalf("kept %d, spent %v, holds %v", kept.learnt[skill.EnchantedBlade], kept.spent, s.playing.Aids)
	}
}

// Zwój Egzorcyzmu held lets a book read its wait holds up go ahead, spent by it, with a
// line saying so; the read waits its 4 hours again after.
func TestZwojLetsAReadItsWaitHoldsUpGoAhead(t *testing.T) {
	book := item.Item{ID: 5, VNum: swirlBook, Count: 1, Cell: 0}
	s, kept, client := reading(t, map[account.Skill]uint8{skill.DragonSwirl: skill.Master}, book, lucky)
	s.playing.SkillReads[skill.DragonSwirl] = noon.Add(time.Hour)
	s.playing.Aids = account.Aids{account.NoReadWait}

	usedFirst(t, s)

	got := packets(t, drain(t, client))
	if kept.learnt[skill.DragonSwirl] != 21 || !kept.reads[skill.DragonSwirl].Equal(noon.Add(skill.ReadWait)) ||
		!slices.Equal(kept.spent, account.Aids{account.NoReadWait}) || len(s.playing.Aids) != 0 ||
		!holds(got, aidGone(account.NoReadWait), infoLine("Zwój Egzorcyzmu pozwolił ci ćwiczyć od razu.")) {
		t.Fatalf("kept %v, %v, spent %v, told %x", kept.learnt, kept.reads, kept.spent, got)
	}
}

// Zwój Egzorcyzmu is kept by a read no wait holds up.
func TestZwojIsKeptWhenNoWaitHoldsTheReadUp(t *testing.T) {
	book := item.Item{ID: 5, VNum: swirlBook, Count: 1, Cell: 0}
	s, kept, _ := reading(t, map[account.Skill]uint8{skill.DragonSwirl: skill.Master}, book, lucky)
	s.playing.Aids = account.Aids{account.NoReadWait}

	usedFirst(t, s)

	if kept.learnt[skill.DragonSwirl] != 21 || len(kept.spent) != 0 || !s.playing.Aids.Has(account.NoReadWait) {
		t.Fatalf("kept %d, spent %v, holds %v", kept.learnt[skill.DragonSwirl], kept.spent, s.playing.Aids)
	}
}

// Zwój Egzorcyzmu held puts a skill still waiting among those a soul stone offers, and
// training it spends it.
func TestZwojLetsASoulStoneTrainASkillStillWaiting(t *testing.T) {
	s, kept, client := soulsAt(t, lucky)
	s.playing.Aids = account.Aids{account.NoReadWait}

	usedFirst(t, s)
	expectSaid(t, client, "Którą umiejętność chcesz ćwiczyć z Kamieniem Duchowym?[ENTER][QUESTION 1;Smoczy Wir (M10)|2;Czarowane Ostrze (G2)|3;Czarowana Zbroja (G1)|4;Anuluj]")
	if err := s.onScriptAnswer(context.Background(), answered(2)); err != nil {
		t.Fatal(err)
	}

	if kept.learnt[skill.EnchantedArmour] != 31 || !slices.Equal(kept.spent, account.Aids{account.NoReadWait}) {
		t.Fatalf("kept %v, spent %v", kept.learnt, kept.spent)
	}
}

// A helper used is held: kept, the item used up, and its icon shown, forever as the
// client draws it.
func TestAHelperUsedIsHeldAndShown(t *testing.T) {
	advice := item.Item{ID: 7, VNum: item.HermitsAdvice, Count: 1, Cell: 0}
	s, kept, client := reading(t, map[account.Skill]uint8{}, advice, lucky)

	usedFirst(t, s)

	got := packets(t, drain(t, client))
	shown := protocol.EncodeAffectAdd(protocol.Affect{Type: 512, Duration: protocol.AffectForever})
	if !holds(got, itemCleared(advice), shown) || !slices.Equal(kept.aids, account.Aids{account.ReadBonus}) ||
		len(kept.left) != 1 || kept.left[0].Count != 0 || !s.playing.Aids.Has(account.ReadBonus) || len(s.inventory) != 0 {
		t.Fatalf("told %x, kept %v, left %+v, holds %v", got, kept.aids, kept.left, s.playing.Aids)
	}
}

// A second of a helper held is refused with a line and kept. One the store holds but the
// world record did not is taken from the store, its icon shown.
func TestASecondHelperIsRefusedAndKept(t *testing.T) {
	for name, c := range map[string]struct {
		held  func(*session, *keptSkills)
		shown int
	}{
		"held":       {func(s *session, _ *keptSkills) { s.playing.Aids = account.Aids{account.NoReadWait} }, 0},
		"kept, only": {func(_ *session, k *keptSkills) { k.aids = account.Aids{account.NoReadWait} }, 1},
	} {
		scroll := item.Item{ID: 7, VNum: item.ExorcismScroll, Count: 1, Cell: 0}
		s, kept, client := reading(t, map[account.Skill]uint8{}, scroll, lucky)
		c.held(s, kept)

		usedFirst(t, s)

		got := packets(t, drain(t, client))
		if len(got) != 1+c.shown || !holds(got, infoLine(aidHeld)) || len(kept.left) != 0 || len(s.inventory) != 1 ||
			!s.playing.Aids.Has(account.NoReadWait) || (c.shown == 1 && !holds(got, aidShown(account.NoReadWait))) {
			t.Errorf("%s: told %x, left %+v, carries %+v, holds %v", name, got, kept.left, s.inventory, s.playing.Aids)
		}
	}
}

// A helper the store cannot keep leaves nothing held, and the item shown as it was.
func TestAHelperNotKeptIsNotHeld(t *testing.T) {
	scroll := item.Item{ID: 7, VNum: item.ExorcismScroll, Count: 1, Cell: 0}
	s, kept, client := reading(t, map[account.Skill]uint8{}, scroll, lucky)
	kept.fail = errors.New("the database is away")

	usedFirst(t, s)

	if got := packets(t, drain(t, client)); len(got) != 1 || !holds(got, itemSet(scroll)) || len(s.playing.Aids) != 0 || len(s.inventory) != 1 {
		t.Fatalf("told %x, holds %v, carries %+v", got, s.playing.Aids, s.inventory)
	}
}

// Both helpers held on a book read its wait holds up are spent by it, both icons going,
// with Zwój's line.
func TestBothHelpersAreSpentOnAWaitingBookRead(t *testing.T) {
	book := item.Item{ID: 5, VNum: swirlBook, Count: 1, Cell: 0}
	s, kept, client := reading(t, map[account.Skill]uint8{skill.DragonSwirl: 28}, book, rolling(rolled(45)))
	s.playing.SkillReads[skill.DragonSwirl] = noon.Add(time.Hour)
	s.playing.Aids = account.Aids{account.ReadBonus, account.NoReadWait}

	usedFirst(t, s)

	got := packets(t, drain(t, client))
	if kept.learnt[skill.DragonSwirl] != 29 || len(kept.spent) != 2 || len(s.playing.Aids) != 0 ||
		!holds(got, aidGone(account.ReadBonus), aidGone(account.NoReadWait), infoLine("Zwój Egzorcyzmu pozwolił ci ćwiczyć od razu.")) {
		t.Fatalf("kept %d, spent %v, holds %v, told %x", kept.learnt[skill.DragonSwirl], kept.spent, s.playing.Aids, got)
	}
}

// A book read refused for its wait, with no Zwój held, leaves Rada Pustelnika held.
func TestRadaIsKeptByAReadRefusedForItsWait(t *testing.T) {
	book := item.Item{ID: 5, VNum: swirlBook, Count: 1, Cell: 0}
	s, kept, _ := reading(t, map[account.Skill]uint8{skill.DragonSwirl: 28}, book, lucky)
	s.playing.SkillReads[skill.DragonSwirl] = noon.Add(time.Hour)
	s.playing.Aids = account.Aids{account.ReadBonus}

	usedFirst(t, s)

	if len(kept.used) != 0 || !s.playing.Aids.Has(account.ReadBonus) || len(s.inventory) != 1 {
		t.Fatalf("used %v, holds %v", kept.used, s.playing.Aids)
	}
}

// A soul stone's read no wait holds up leaves Zwój Egzorcyzmu held.
func TestZwojIsKeptByASoulStoneWithNoWait(t *testing.T) {
	s, kept, client := soulsAt(t, lucky)
	s.playing.Aids = account.Aids{account.NoReadWait}
	usedFirst(t, s)
	drain(t, client)

	if err := s.onScriptAnswer(context.Background(), answered(0)); err != nil {
		t.Fatal(err)
	}

	if kept.learnt[skill.DragonSwirl] != skill.GrandMaster || len(kept.spent) != 0 || !s.playing.Aids.Has(account.NoReadWait) {
		t.Fatalf("kept %v, spent %v, holds %v", kept.learnt, kept.spent, s.playing.Aids)
	}
}

// The aids held are shown as their icons on entering the world.
func TestTheAidsHeldAreShownOnEntering(t *testing.T) {
	conn, client := watcher(t)
	entering := &session{svc: worldWith(t), conn: conn, phase: protocol.PhaseLoading, playing: &character{vid: 2000,
		Character: account.Character{Name: "Kira", HP: 1, X: homeX, Y: homeY, Aids: account.Aids{account.ReadBonus, account.NoReadWait}}}}

	if err := entering.onEnterGame(); err != nil {
		t.Fatal(err)
	}

	if got := packets(t, drain(t, client)); !holds(got, aidShown(account.ReadBonus), aidShown(account.NoReadWait)) {
		t.Fatalf("shown %x", got)
	}
}
