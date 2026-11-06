package game

import (
	"context"
	"slices"
	"strconv"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/mob"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/skill"
)

// Skills is where the game port keeps a character's skill group and the skills it has
// learnt.
type Skills interface {
	Learnt(ctx context.Context, characterID uint32) (map[account.Skill]uint8, error)
	Choose(ctx context.Context, characterID uint32, group account.SkillGroup) error
	Raise(ctx context.Context, characterID uint32, s account.Skill) (uint8, error)
}

// changeBefore is the level a character may still change its skill group below, giving
// back every point; from it on, the path is kept.
const changeBefore = 20

// paths are how a teacher names the skill group they teach.
var paths = map[account.SkillGroup]string{
	"weaponry":    "Mocy Broni",
	"black_magic": "Czarnej Magii",
}

// talk is what a teacher asked the player, which their next answer in the dialog answers:
// whether to take up the teacher's group, then whether they are sure.
type talk struct {
	teacher    uint32 // the teacher's vid
	confirming bool
}

// teach answers a click on a class teacher. A sura of level 5 or more without a group is
// offered the teacher's, and one of the other group below level 20 is offered a change;
// anyone else is told why not.
func (s *session) teach(vid uint32, teacher mob.Proto) error {
	g := teacher.Teaches
	if why, refused := s.whyNot(g); refused {
		return s.tell(why)
	}
	s.talking = &talk{teacher: vid}
	if _, group := s.svc.presence.trainee(s.playing.vid); group != "" {
		return s.ask("Chcesz porzucić swoją drogę i uczyć się drogi " + paths[g] + "? Zapomnisz to, czego się nauczyłeś, ale odzyskasz wszystkie punkty.")
	}
	return s.ask("Mogę nauczyć cię drogi " + paths[g] + ". Czy chcesz nią podążać?")
}

// whyNot is what a teacher of group tells the player instead of an offer, if they may not
// take it up now: of another class, below level 5, on that path already, or on the other
// from level 20.
func (s *session) whyNot(g account.SkillGroup) (string, bool) {
	level, group := s.svc.presence.trainee(s.playing.vid)
	groups := s.playing.Class.SkillGroups() // the class never changes in play
	switch {
	case !slices.Contains(groups[:], g):
		return "Nie uczę twojej klasy.", true
	case level < skill.FirstLevel:
		return "Wróć, gdy osiągniesz 5 poziom.", true
	case group == g:
		return "Ćwicz dalej, a twoja droga " + paths[g] + " zaprowadzi cię daleko.", true
	case group != "" && level >= changeBefore:
		return "Twoja droga jest już wybrana.", true
	}
	return "", false
}

// onScriptAnswer carries the player's answer to what a teacher asked: yes to the offer
// asks whether they are sure, and yes to that takes up the group. Each yes is taken only
// while the teacher is still in reach and the player still may: an offer kept open while
// they levelled past 20, or walked off, is told no again. Any other answer, or one to
// nothing asked, ends the talk.
func (s *session) onScriptAnswer(ctx context.Context, body []byte) error {
	answer, err := protocol.DecodeScriptAnswer(body)
	if err != nil {
		return err
	}
	t := s.talking
	s.talking = nil
	if t == nil || answer != 0 {
		return nil
	}
	teacher, near := s.svc.presence.npcInReach(s.playing.vid, t.teacher)
	if !near {
		return nil
	}
	if why, refused := s.whyNot(teacher.Teaches); refused {
		return s.tell(why)
	}
	if !t.confirming {
		s.talking = &talk{teacher: t.teacher, confirming: true}
		return s.ask("Czy na pewno? Wybór drogi " + paths[teacher.Teaches] + " to poważna decyzja.")
	}
	return s.choose(ctx, teacher.Teaches)
}

// choose takes up the skill group, forgetting what was learnt: it is kept, then the client
// is told the group, the levels, all 0 now, after it, as it forgets them on the group, the
// points the character has, and the taskbar's skill slots emptied.
func (s *session) choose(ctx context.Context, group account.SkillGroup) error {
	if err := s.svc.skills.Choose(ctx, s.playing.ID, group); err != nil {
		s.conn.Log().Error("skill group not kept", "group", group, "err", err)
		return nil
	}
	points := s.svc.presence.chooseGroup(s.playing.vid, group)
	s.conn.Log().Info("skill group chosen", "character", s.playing.Name, "group", group)
	return s.conn.Send(append([][]byte{
		protocol.EncodeChangeSkillGroup(skillGroupNumber(s.playing.Class, group)),
		protocol.EncodeSkillLevels(skillLevels(nil)),
		protocol.EncodePointChange(s.playing.vid, protocol.PointSkillPoints, 0, int32(points)), //nolint:gosec // under 2^31
		script("Od dziś podążasz drogą " + paths[group] + ".[ENTER][DONE]"),
	}, s.forgetSkillSlots()...)...)
}

// raiseSkill raises the skill the skill window's + button names by the client's number for
// it, if the player has chosen its group, it is below its last level and a point is free.
// A skill of the group the game does not have yet is not raised, and the player is told.
func (s *session) raiseSkill(ctx context.Context, arg string) error {
	vnum, err := strconv.Atoi(arg)
	sk, known := skillByVNum(vnum)
	if err != nil || !known {
		return s.conn.Send(protocol.EncodeChat(protocol.ChatInfo, 0, 0, textToClient("Tej umiejętności nie można jeszcze rozwijać.")))
	}
	if !s.svc.presence.mayRaise(s.playing.vid, sk) {
		s.conn.Log().Debug("skill not raised", "skill", sk)
		return nil
	}
	level, err := s.svc.skills.Raise(ctx, s.playing.ID, sk)
	if err != nil {
		s.conn.Log().Error("skill raised not kept", "skill", sk, "err", err)
		return nil
	}
	levels, points := s.svc.presence.raised(s.playing.vid, sk, level)
	return s.conn.Send(
		protocol.EncodeSkillLevels(levels),
		protocol.EncodePointChange(s.playing.vid, protocol.PointSkillPoints, -1, int32(points)), //nolint:gosec // under 2^31
	)
}

// tell shows a line from the NPC the player talks to, with an OK that answers nothing.
func (s *session) tell(line string) error {
	s.talking = nil
	return s.conn.Send(script(line + "[ENTER][DONE]"))
}

// ask shows a yes-or-no question from the NPC; no comes last, as the client sends the last
// button for the escape key.
func (s *session) ask(question string) error {
	return s.conn.Send(script(question + "[ENTER][QUESTION 1;Tak|2;Nie]"))
}

func script(text string) []byte {
	return protocol.EncodeScript(protocol.ScriptDialog, []byte(textToClient(text)))
}

// trainee is what a teacher needs to know of the player: their level and skill group.
func (w *presence) trainee(vid uint32) (uint8, account.SkillGroup) {
	w.mu.Lock()
	defer w.mu.Unlock()
	c := w.all[vid].char
	return c.Level, c.SkillGroup
}

// chooseGroup puts the player on the skill group with nothing learnt, and returns the
// points they now have.
func (w *presence) chooseGroup(vid uint32, group account.SkillGroup) uint32 {
	w.mu.Lock()
	defer w.mu.Unlock()
	c := w.all[vid].char
	c.SkillGroup, c.Skills = group, map[account.Skill]uint8{}
	return skill.Points(c.Level, nil)
}

// mayRaise is whether the player may raise the skill a level: it is of their class and the
// group they chose, below its last level, and they have a point for it.
func (w *presence) mayRaise(vid uint32, sk account.Skill) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	c := w.all[vid].char
	p, _ := skill.Lookup(sk)
	return p.Class == c.Class && p.Group == c.SkillGroup && c.Skills[sk] < skill.MaxLevel && skill.Points(c.Level, c.Skills) > 0
}

// raised puts the skill at its new level, and returns every skill's level for the client
// and the points left.
func (w *presence) raised(vid uint32, sk account.Skill, level uint8) (protocol.SkillLevels, uint32) {
	w.mu.Lock()
	defer w.mu.Unlock()
	c := w.all[vid].char
	c.Skills[sk] = level
	return skillLevels(c.Skills), skill.Points(c.Level, c.Skills)
}

// skillPoints is what the skill window shows: the points the character has once it has
// chosen a group, which is what they are for.
func skillPoints(c *account.Character) uint32 {
	if c.SkillGroup == "" {
		return 0
	}
	return skill.Points(c.Level, c.Skills)
}
