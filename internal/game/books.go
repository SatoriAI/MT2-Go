package game

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/skill"
)

// chance is how likely a book or a soul stone raises a skill at a level, in hundredths, and
// whether it raises one at that level at all.
type chance func(level uint8) (uint16, bool)

// readBook trains the skill the book teaches: at M1 to M9, and once its wait is over, the
// skill may rise a level, and the book is used up either way. Anything else is refused
// with a line saying why, and the book is kept.
func (s *session) readBook(ctx context.Context, book item.Item, sk account.Skill) error {
	level, why := s.svc.presence.trainable(s.playing.vid, sk, s.svc.now(), skill.BookChance)
	switch why {
	case "":
		return s.train(ctx, book, sk, level, skill.BookChance, "Księga przepadła.")
	case notYetTrainable:
		if level < skill.Master {
			why = "Ta umiejętność musi najpierw zostać Mistrzem."
		} else {
			why = "Tej umiejętności nie rozwiniesz już księgą."
		}
	}
	return s.conn.Send(infoLine(why))
}

// notYetTrainable is trainable's answer for a skill at a level the book or stone does not
// raise; its caller says which.
const notYetTrainable = "level"

// offerSoulStone asks which skill to train with the soul stone, of those it raises now:
// an M10, or a Grand Master below G10, whose wait is over. With none, the player is told
// why, and the stone is kept.
func (s *session) offerSoulStone(stone item.Item) error {
	ready, wait := s.svc.presence.soulReady(s.playing.vid, s.svc.now())
	if len(ready) == 0 {
		if wait > 0 {
			return s.conn.Send(infoLine(waitLine(wait)))
		}
		return s.conn.Send(infoLine("Żadna twoja umiejętność nie jest gotowa na Kamień Duchowy: musi być co najmniej M10."))
	}
	var options []string
	for i, r := range ready {
		book, _ := item.BookOf(r.skill)
		options = append(options, fmt.Sprintf("%d;%s (%s)", i+1, strings.TrimPrefix(book.Name, "Instr. "), gradeName(r.level)))
	}
	options = append(options, fmt.Sprintf("%d;Anuluj", len(ready)+1))
	s.talking = &talk{stone: stone.ID}
	for _, r := range ready {
		s.talking.skills = append(s.talking.skills, r.skill)
	}
	return s.conn.Send(script("Którą umiejętność chcesz ćwiczyć z Kamieniem Duchowym?[ENTER][QUESTION " + strings.Join(options, "|") + "]"))
}

// useSoulStone trains the skill the player picked with the stone they were asked about,
// if they still carry it and the skill is still ready; the last answer cancels.
func (s *session) useSoulStone(ctx context.Context, t *talk, answer uint8) error {
	stone, held := s.inventory.ByID(t.stone)
	if int(answer) >= len(t.skills) || !held {
		return nil
	}
	sk := t.skills[answer]
	level, why := s.svc.presence.trainable(s.playing.vid, sk, s.svc.now(), skill.SoulChance)
	if why != "" {
		s.conn.Log().Debug("soul stone refused", "skill", sk, "why", why)
		return nil
	}
	return s.train(ctx, stone, sk, level, skill.SoulChance, "Kamień Duchowy przepadł.")
}

// train rolls the chance that the book or stone raises the skill from level, keeps the
// read, the skill's next wait and the book or stone used up, as neither stacks, then
// tells the player: the item gone, their skills on a rise, and a line. A write that fails
// changes nothing, the client having moved nothing itself.
func (s *session) train(ctx context.Context, used item.Item, sk account.Skill, level uint8, odds chance, lost string) error {
	now := s.svc.now()
	percent, _ := odds(level)
	to := level
	if s.rand.IntN(100) < int(percent) {
		to = level + 1
	}
	next := now.Add(skill.ReadWait)
	if err := s.svc.skills.Train(ctx, s.playing.ID, sk, level, to, next, used.ID); err != nil {
		s.conn.Log().Error("training not kept", "skill", sk, "item", used.ID, "err", err)
		return nil
	}
	s.inventory = slices.DeleteFunc(s.inventory, func(held item.Item) bool { return held.ID == used.ID })
	levels := s.svc.presence.trained(s.playing.vid, sk, to, next)
	packets := append([][]byte{itemCleared(used)}, s.followTaskbar()...)
	line := "Nie udało się. " + lost
	if to > level {
		packets = append(packets, protocol.EncodeSkillLevels(levels))
		line = "Udało się! Umiejętność osiągnęła poziom " + gradeName(to) + "."
	}
	s.conn.Log().Info("skill trained", "skill", sk, "from", level, "to", to, "item", used.VNum)
	return s.conn.Send(append(packets, infoLine(line))...)
}

// gradeName is how the client labels a Master's or a Grand Master's level: M1 to M10, G1
// to G10.
func gradeName(level uint8) string {
	if level >= skill.GrandMaster {
		return fmt.Sprintf("G%d", level-skill.GrandMaster+1)
	}
	return fmt.Sprintf("M%d", level-skill.Master+1)
}

// waitLine tells the player how long before they may train a skill again, in whole
// minutes rounded up, so a wait of seconds is never told as none.
func waitLine(wait time.Duration) string {
	wait = (wait + time.Minute - 1).Truncate(time.Minute)
	return fmt.Sprintf("Kolejną naukę możesz podjąć za %d godz. %d min.", int(wait.Hours()), int(wait.Minutes())%60)
}

// trainable is the level of the skill, if the player may train it now with what odds
// raise: they learnt it, which only a skill of their group can be, it is at a level the
// odds raise, and its wait is over. Else it says why not: a line for the player, or
// notYetTrainable for a level the odds do not raise, with the level, for the caller to
// word.
func (w *presence) trainable(vid uint32, sk account.Skill, now time.Time, odds chance) (uint8, string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	c := w.all[vid].char
	level, learnt := c.Skills[sk]
	switch _, raises := odds(level); {
	case !learnt:
		return 0, "Nie znasz tej umiejętności."
	case !raises:
		return level, notYetTrainable
	case now.Before(c.SkillReads[sk]):
		return level, waitLine(c.SkillReads[sk].Sub(now))
	}
	return level, ""
}

// readySkill is a skill a soul stone may train now, at its level.
type readySkill struct {
	skill account.Skill
	level uint8
}

// soulReady is the player's skills a soul stone raises now, in the order the skill window
// shows them, and, with none, how long until the soonest one it raises is out of its wait;
// 0 when it raises none.
func (w *presence) soulReady(vid uint32, now time.Time) ([]readySkill, time.Duration) {
	w.mu.Lock()
	defer w.mu.Unlock()
	c := w.all[vid].char
	var ready []readySkill
	var soonest time.Duration
	for sk, level := range c.Skills {
		if _, raises := skill.SoulChance(level); !raises {
			continue
		}
		if wait := c.SkillReads[sk].Sub(now); wait > 0 {
			if soonest == 0 || wait < soonest {
				soonest = wait
			}
			continue
		}
		ready = append(ready, readySkill{sk, level})
	}
	slices.SortFunc(ready, func(a, b readySkill) int { return cmp.Compare(skillVNums[a.skill], skillVNums[b.skill]) })
	return ready, soonest
}

// trained puts the skill at the level a read left it and its next wait, and returns
// every skill's level for the client. A self-buff of it running adds at its new level.
func (w *presence) trained(vid uint32, sk account.Skill, level uint8, next time.Time) protocol.SkillLevels {
	w.mu.Lock()
	defer w.mu.Unlock()
	pl := w.all[vid]
	c := pl.char
	c.Skills[sk], c.SkillReads[sk] = level, next
	if _, running := c.Buffs[sk]; running {
		w.showAffects(pl, sk)
	}
	return skillLevels(c.Skills)
}
