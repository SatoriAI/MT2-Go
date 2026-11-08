package world

import (
	"math"
	"slices"
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
		for _, a := range areas(m.Name) {
			if !ring[a.vnum] {
				continue
			}
			if d := math.Hypot(float64(a.x-(x-m.X)), float64(a.y-(y-m.Y))); d < 30000 || d > 55000 {
				t.Errorf("%s: %d at (%d, %d) is %.0f from the start", m.Name, a.vnum, a.x, a.y, d)
			}
			for _, o := range areas(m.Name) {
				if (o.x != a.x || o.y != a.y) && math.Hypot(float64(o.x-a.x), float64(o.y-a.y)) < 9000 {
					t.Errorf("%s: %d at (%d, %d) is near %d at (%d, %d)", m.Name, a.vnum, a.x, a.y, o.vnum, o.x, o.y)
				}
			}
		}
	}
}

// Each first village has three Metin Cierpienia, two Metin Walki and two Metin Bitwy, each
// with three spots of its own.
func TestEveryFirstVillageHasItsStones(t *testing.T) {
	for _, village := range []string{"metin2_map_a1", "metin2_map_b1", "metin2_map_c1"} {
		count := map[uint16]int{}
		for _, s := range spawnsByMap[village] {
			if len(s.spots) > 0 {
				count[s.vnum]++
				if len(s.spots) != 3 || s.count > 1 || s.spread != 0 {
					t.Errorf("%s: %+v", village, s)
				}
			}
		}
		if count[8001] != 3 || count[8002] != 2 || count[8003] != 2 || len(count) != 3 {
			t.Errorf("%s: stones %v, want three, two and two", village, count)
		}
	}
}

// Each stone's spots lie on its map, in its band from the kingdom's start, the stronger the
// further: 25000 to 35000, 35000 to 45000 and 45000 to 55000. Each is at least 3500 from
// any monster area, 8000 from any NPC, and 9000 from any other spot.
func TestStoneSpotsLieInTheirBandsAndApart(t *testing.T) {
	bands := map[uint16][2]float64{8001: {25000, 35000}, 8002: {35000, 45000}, 8003: {45000, 55000}}
	for _, e := range []account.Empire{account.EmpireShinsoo, account.EmpireChunjo, account.EmpireJinno} {
		x, y, _ := e.StartPosition()
		m, _ := MapAt(x, y)
		var spots []Spot
		for _, s := range spawnsByMap[m.Name] {
			spots = append(spots, s.spots...)
		}
		for _, s := range spawnsByMap[m.Name] {
			for _, p := range s.spots {
				if got, ok := MapAt(m.X+p.X, m.Y+p.Y); !ok || got.Name != m.Name {
					t.Errorf("%s: %d at %v is off the map", m.Name, s.vnum, p)
				}
				band := bands[s.vnum]
				if d := math.Hypot(float64(p.X-(x-m.X)), float64(p.Y-(y-m.Y))); d < band[0] || d > band[1] {
					t.Errorf("%s: %d at %v is %.0f from the start", m.Name, s.vnum, p, d)
				}
				for _, o := range areas(m.Name) {
					gap := 3500.0
					if o.vnum >= 9000 {
						gap = 8000
					}
					if math.Hypot(float64(o.x-p.X), float64(o.y-p.Y)) < gap {
						t.Errorf("%s: %d at %v is near %d at (%d, %d)", m.Name, s.vnum, p, o.vnum, o.x, o.y)
					}
				}
				for _, o := range spots {
					if o != p && math.Hypot(float64(o.X-p.X), float64(o.Y-p.Y)) < 9000 {
						t.Errorf("%s: %v is near %v", m.Name, p, o)
					}
				}
			}
		}
	}
}

func areas(m string) []spawnAt {
	return slices.DeleteFunc(slices.Clone(spawnsByMap[m]), func(s spawnAt) bool { return len(s.spots) > 0 })
}

// Each first village has the warrior's two teachers, the ninja's and the sura's, one of
// each.
func TestEveryFirstVillageHasItsTeachers(t *testing.T) {
	teachers := map[string][6]uint16{
		"metin2_map_a1": {20300, 20301, 20302, 20303, 20304, 20305},
		"metin2_map_b1": {20320, 20321, 20322, 20323, 20324, 20325},
		"metin2_map_c1": {20340, 20341, 20342, 20343, 20344, 20345},
	}
	for village, want := range teachers {
		count := map[uint16]int{}
		for _, s := range spawnsByMap[village] {
			count[s.vnum]++
		}
		for _, vnum := range want {
			if count[vnum] != 1 {
				t.Errorf("%s: %d of teacher %d", village, count[vnum], vnum)
			}
		}
	}
}
