package game

import (
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/world"
)

// Restart in town, fallen in the valley, brings her back at its arrival, kept there.
func TestRestartingInTownInTheValleyRisesAtItsArrival(t *testing.T) {
	s, _, _, _ := ringing(t, 20)
	valley, _ := world.MapNamed(world.Valley)
	s.svc.record.held(func(l locked) {
		p := l.all[1000]
		p.add.X, p.add.Y, p.char.HP = valley.X+70000, valley.Y+70000, 0
	})

	command(t, s, "restart_town")

	at := s.svc.record.state.all[1000].add
	if at.X != valley.Arrival.X || at.Y != valley.Arrival.Y || s.playing.X != at.X || s.playing.Y != at.Y {
		t.Fatalf("rose at (%d, %d), kept at (%d, %d), want %v", at.X, at.Y, s.playing.X, s.playing.Y, valley.Arrival)
	}
}

// One who falls in the valley rises in town at its arrival.
func TestTheTownOfTheValleyIsItsArrival(t *testing.T) {
	valley, _ := world.MapNamed(world.Valley)
	homeX, homeY, _ := account.EmpireShinsoo.StartPosition()
	if x, y := riseAt(valley.X+70000, valley.Y+70000, homeX, homeY); x != valley.Arrival.X || y != valley.Arrival.Y {
		t.Errorf("rises at (%d, %d), want %v", x, y, valley.Arrival)
	}
}

// One who falls on her kingdom's map rises in town at its start.
func TestTheTownOfAKingdomsMapIsItsStart(t *testing.T) {
	homeX, homeY, _ := account.EmpireShinsoo.StartPosition()
	if x, y := riseAt(homeX+5000, homeY, homeX, homeY); x != homeX || y != homeY {
		t.Errorf("rises at (%d, %d)", x, y)
	}
}
