// Package upgrade is what upgrading gear a grade at the blacksmith takes, and what comes
// of a failure.
package upgrade

import "github.com/SatoriAI/MT2-Go/internal/item"

// The materials the later grades take: a wolf's claw for a weapon, a boar's tooth for
// armour.
const (
	WolfClaw      uint32 = 30028
	WildBoarTooth uint32 = 30004
)

// Step is what upgrading one kind a grade takes: yang, and from the step to +5 on a
// count of the material for its kind, with a chance in 100 of success. A failure loses
// what it took; from the step to +5 on, the item also drops a grade. It is never
// destroyed.
type Step struct {
	Into      uint32 // the kind it becomes
	Yang      uint32
	Chance    uint8
	Material  uint32 // 0 for none
	Count     uint8
	Downgrade bool
}

// Failed is the kind it is after a failure: itself, or a grade lower.
func (s Step) Failed() uint32 {
	if s.Downgrade {
		return s.Into - 2
	}
	return s.Into - 1
}

// steps are the veteran's table, by the grade upgraded to: every grade costs more and
// succeeds less, so +4 is a session of farming and +9 the goal of the first maps.
var steps = [item.MaxGrade + 1]struct {
	yang   uint32
	chance uint8
	count  uint8
}{
	1: {200, 100, 0}, 2: {400, 90, 0}, 3: {800, 80, 0}, 4: {1500, 70, 0},
	5: {3000, 60, 1}, 6: {5000, 50, 2}, 7: {8000, 40, 3}, 8: {12000, 30, 4}, 9: {20000, 20, 5},
}

// Of is what upgrading the kind a grade takes, or false for one that is not upgraded or
// is at its last grade.
func Of(p item.Proto) (Step, bool) {
	if !p.Graded || p.Grade >= item.MaxGrade {
		return Step{}, false
	}
	to := steps[p.Grade+1]
	s := Step{Into: p.VNum + 1, Yang: to.yang, Chance: to.chance, Count: to.count, Downgrade: to.count > 0}
	if to.count > 0 {
		s.Material = WildBoarTooth
		if p.Wear == item.Weapon {
			s.Material = WolfClaw
		}
	}
	return s, true
}
