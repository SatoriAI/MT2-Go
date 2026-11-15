package world

import (
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/account"
)

// Each first village has one gate to the valley, with somewhere open to land a step inside
// it, and one closed; the valley has one gate home and two closed.
func TestEachMapHasItsGates(t *testing.T) {
	for name, want := range map[string]map[Way]int{
		"metin2_map_a1": {ToValley: 1, Closed: 1}, "metin2_map_b1": {ToValley: 1, Closed: 1},
		"metin2_map_c1": {ToValley: 1, Closed: 1}, Valley: {ToHome: 1, Closed: 2},
	} {
		got := map[Way]int{}
		m, _ := MapNamed(name)
		for _, p := range portalsByMap[name] {
			got[p.Way]++
			if p.At.X < 0 || p.At.X >= m.Width || p.At.Y < 0 || p.At.Y >= m.Height {
				t.Errorf("%s: a gate at %v off the map", name, p.At)
			}
			if p.Way == ToValley && distance(p.Land, p.At) <= portalRadius {
				t.Errorf("%s: landing at %v is inside its gate", name, p.Land)
			}
		}
		for way, n := range want {
			if got[way] != n {
				t.Errorf("%s: %d gates %s, want %d", name, got[way], way, n)
			}
		}
	}
}

// A point within 6 m of a gate's centre is in it, in world coordinates; one past that is not.
func TestAGateTakesThoseWithinItsRadius(t *testing.T) {
	m, _ := MapNamed("metin2_map_a1")
	if p, in := PortalAt(m.X+40500, m.Y+7300+600); !in || p.Way != ToValley || p.Land != (Spot{m.X + 40500, m.Y + 9300}) {
		t.Errorf("at the edge: %+v %v", p, in)
	}
	if _, in := PortalAt(m.X+40500, m.Y+7300+601); in {
		t.Error("past the edge, still in")
	}
}

// One coming home from the valley lands a step inside their kingdom's gate to it.
func TestHomeIsInsideTheKingdomsGateToTheValley(t *testing.T) {
	for e, want := range map[account.Empire]Spot{
		account.EmpireShinsoo: {409600 + 40500, 896000 + 9300},
		account.EmpireChunjo:  {87600, 102400 + 110700},
		account.EmpireJinno:   {921600 + 12290, 204800 + 14010},
	} {
		x, y, _ := e.StartPosition()
		if got, ok := HomeGate(x, y); !ok || got != want {
			t.Errorf("%s: home at %v, want %v", e, got, want)
		}
	}
}

// Each first village has its teleporter.
func TestEveryFirstVillageHasItsTeleporter(t *testing.T) {
	for _, name := range []string{"metin2_map_a1", "metin2_map_b1", "metin2_map_c1"} {
		n := 0
		for _, s := range spawnsByMap[name] {
			if s.vnum == 9012 {
				n++
			}
		}
		if n != 1 {
			t.Errorf("%s: %d teleporters", name, n)
		}
	}
}
