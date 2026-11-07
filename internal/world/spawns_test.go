package world

import (
	"math"
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/account"
)

// A spawn's whole area lies on its map, so every mob it puts in the world is on one.
func TestEverySpawnAreaLiesOnItsMap(t *testing.T) {
	for _, m := range maps {
		for _, s := range spawnsByMap[m.Name] {
			for _, corner := range [][2]int32{{-1, -1}, {1, 1}, {-1, 1}, {1, -1}} {
				x, y := m.X+s.x+corner[0]*s.spread, m.Y+s.y+corner[1]*s.spread
				if got, ok := MapAt(x, y); !ok || got.Name != m.Name {
					t.Errorf("%s: %d at (%d, %d) reaches (%d, %d), off the map", m.Name, s.vnum, s.x, s.y, x, y)
				}
			}
		}
	}
}

func TestSpawnsAreInWorldCoordinatesWithAtLeastOneMob(t *testing.T) {
	var weaponsInJinno *Spawn
	for _, s := range Spawns() {
		if s.Count < 1 {
			t.Errorf("%d at (%d, %d) spawns nothing", s.VNum, s.X, s.Y)
		}
		if s.VNum == 9001 && s.X == 921600+43000 {
			weaponsInJinno = &s
		}
	}
	if weaponsInJinno == nil || weaponsInJinno.Y != 204800+60800 {
		t.Fatalf("the Jinno weapon dealer: got %+v, want at (964600, 265600)", weaponsInJinno)
	}
}

// Each first village has its blacksmith, beside the sura's teachers.
func TestEveryFirstVillageHasItsBlacksmith(t *testing.T) {
	for _, village := range []string{"metin2_map_a1", "metin2_map_b1", "metin2_map_c1"} {
		smiths := 0
		for _, s := range spawnsByMap[village] {
			if s.vnum == 20016 {
				smiths++
			}
		}
		if smiths != 1 {
			t.Errorf("%s: %d blacksmiths", village, smiths)
		}
	}
}

// Each first village has its second ring of stronger monsters: blue wolves, alpha blue
// wolves with red boars, bears with grizzlies, and tigers.
func TestEveryFirstVillageHasItsSecondRing(t *testing.T) {
	for _, village := range []string{"metin2_map_a1", "metin2_map_b1", "metin2_map_c1"} {
		count := map[uint16]int{}
		for _, s := range spawnsByMap[village] {
			count[s.vnum] += s.count
		}
		for vnum, want := range map[uint16]int{104: 6, 105: 3, 109: 3, 110: 3, 111: 3, 114: 6} {
			if count[vnum] != want {
				t.Errorf("%s: %d of %d, want %d", village, count[vnum], vnum, want)
			}
		}
	}
}

// The second ring lies 30000 to 55000 from each kingdom's start, and each of its areas at
// least 9000 from every other spawn but its own.
func TestTheSecondRingLiesOutFromTheStartAndApart(t *testing.T) {
	ring := map[uint16]bool{104: true, 105: true, 109: true, 110: true, 111: true, 114: true}
	for _, e := range []account.Empire{account.EmpireShinsoo, account.EmpireChunjo, account.EmpireJinno} {
		x, y, _ := e.StartPosition()
		m, _ := MapAt(x, y)
		for _, a := range spawnsByMap[m.Name] {
			if !ring[a.vnum] {
				continue
			}
			if d := math.Hypot(float64(a.x-(x-m.X)), float64(a.y-(y-m.Y))); d < 30000 || d > 55000 {
				t.Errorf("%s: %d at (%d, %d) is %.0f from the start", m.Name, a.vnum, a.x, a.y, d)
			}
			for _, o := range spawnsByMap[m.Name] {
				if (o.x != a.x || o.y != a.y) && math.Hypot(float64(o.x-a.x), float64(o.y-a.y)) < 9000 {
					t.Errorf("%s: %d at (%d, %d) is near %d at (%d, %d)", m.Name, a.vnum, a.x, a.y, o.vnum, o.x, o.y)
				}
			}
		}
	}
}
