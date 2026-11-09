package world

import (
	"math"
	"slices"
	"testing"
	"time"

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

// bands are how far from its kingdom's start each kind of monster stands: dogs nearest,
// then wolves and boars, alpha and blue wolves, and the second ring, overlapping a
// little where a kind leads a pack.
var bands = map[uint16]struct{ from, to float64 }{
	101: {2500, 15000},
	102: {9000, 18000}, 108: {9000, 18000},
	103: {9000, 30000}, 104: {18000, 30000},
	105: {18000, 42000}, 109: {30000, 42000},
	110: {30000, 55000}, 111: {42000, 55000}, 114: {42000, 55000},
	// the cursed, which attack on sight, in the far bands only
	131: {30000, 42000}, 132: {30000, 42000},
	133: {30000, 55000}, 137: {30000, 55000}, // kin of a far leader too
	134: {42000, 55000}, 135: {42000, 55000}, 136: {42000, 55000}, 138: {42000, 55000}, 139: {42000, 55000},
}

// backAfter is how long a pack that far from the start takes to come back, in seconds.
func backAfter(d float64) int {
	switch {
	case d < 18000:
		return 30
	case d < 30000:
		return 45
	}
	return 60
}

// leaders are the kinds that lead a pack of their kin: an alpha wolf three wolves, an
// alpha blue wolf three blue wolves, and each cursed alpha, and the cursed red boar, three
// of their cursed kin.
var leaders = map[uint16]uint16{103: 102, 105: 104, 132: 131, 134: 133, 136: 135, 138: 137}

// monsterAreas are each first village's monster packs.
func monsterAreas(t *testing.T) map[string][]spawnAt {
	t.Helper()
	got := map[string][]spawnAt{}
	for _, e := range []account.Empire{account.EmpireShinsoo, account.EmpireChunjo, account.EmpireJinno} {
		x, y, _ := e.StartPosition()
		m, _ := MapAt(x, y)
		for _, a := range areas(m.Name) {
			if a.vnum < 9000 {
				got[m.Name] = append(got[m.Name], a)
			}
		}
	}
	return got
}

func fromStart(name string, a spawnAt) float64 {
	for _, e := range []account.Empire{account.EmpireShinsoo, account.EmpireChunjo, account.EmpireJinno} {
		x, y, _ := e.StartPosition()
		if m, _ := MapAt(x, y); m.Name == name {
			return math.Hypot(float64(a.x-(x-m.X)), float64(a.y-(y-m.Y)))
		}
	}
	return 0
}

// packs are a village's monster spawns grouped by the centre they share.
func packs(all []spawnAt) map[[2]int32][]spawnAt {
	byCentre := map[[2]int32][]spawnAt{}
	for _, a := range all {
		byCentre[[2]int32{a.x, a.y}] = append(byCentre[[2]int32{a.x, a.y}], a)
	}
	return byCentre
}

// Each kind of monster stands in its band from the kingdom's start, and comes back as its
// distance says: within 18000 after 30 s, within 30000 after 45, further out after 60.
func TestEveryMonsterStandsInItsBand(t *testing.T) {
	for name, all := range monsterAreas(t) {
		for _, a := range all {
			band, known := bands[a.vnum]
			if d := fromStart(name, a); !known || d < band.from || d >= band.to || a.respawn != backAfter(d) {
				t.Errorf("%s: %d at (%d, %d), %.0f out, back after %d s", name, a.vnum, a.x, a.y, d, a.respawn)
			}
		}
	}
}

// A pack is 3 to 5 of one kind, or a leader with 3 of its kin, standing at one centre.
func TestEveryPackIsOneKindOrALeaderWithItsKin(t *testing.T) {
	for name, all := range monsterAreas(t) {
		for centre, pack := range packs(all) {
			switch {
			case len(pack) == 1 && pack[0].count >= 3 && pack[0].count <= 5:
			case len(pack) == 2 && pack[0].count+pack[1].count == 4 &&
				(pack[0].count == 1 && leaders[pack[0].vnum] == pack[1].vnum ||
					pack[1].count == 1 && leaders[pack[1].vnum] == pack[0].vnum):
			default:
				t.Errorf("%s: at %v %+v", name, centre, pack)
			}
		}
	}
}

// Packs stand apart: 2000 between centres where both lie within 18000 of the start, 2500
// within 30000, and 4000 where either lies further; and their centres 2500 from every NPC
// and 3500 from every stone's spot.
func TestPacksStandApart(t *testing.T) {
	for name, all := range monsterAreas(t) {
		var centres [][2]int32
		for c := range packs(all) {
			centres = append(centres, c)
		}
		var spots []Spot
		for _, s := range spawnsByMap[name] {
			spots = append(spots, s.spots...)
		}
		for i, a := range centres {
			da := fromStart(name, spawnAt{x: a[0], y: a[1]})
			for _, o := range centres[i+1:] {
				gap := 2000.0
				switch further := max(da, fromStart(name, spawnAt{x: o[0], y: o[1]})); {
				case further >= 30000:
					gap = 4000
				case further >= 18000:
					gap = 2500
				}
				if d := math.Hypot(float64(o[0]-a[0]), float64(o[1]-a[1])); d < gap {
					t.Errorf("%s: packs at %v and %v are %.0f apart", name, a, o, d)
				}
			}
			for _, n := range areas(name) {
				if n.vnum >= 9000 && math.Hypot(float64(n.x-a[0]), float64(n.y-a[1])) < 2500 {
					t.Errorf("%s: the pack at %v is near NPC %d", name, a, n.vnum)
				}
			}
			for _, p := range spots {
				if math.Hypot(float64(p.X-a[0]), float64(p.Y-a[1])) < 3500 {
					t.Errorf("%s: the pack at %v is near a stone's spot %v", name, a, p)
				}
			}
		}
	}
}

// Each first village's fields are full, as the world serves them, its Mobs applied: at
// least 380 monsters, at least 70 within 15000 of the start, the nearest within 8000: the
// town stands between the start and the nearest fields.
func TestEveryFirstVillagesFieldsAreFull(t *testing.T) {
	for _, e := range []account.Empire{account.EmpireShinsoo, account.EmpireChunjo, account.EmpireJinno} {
		x, y, _ := e.StartPosition()
		home, _ := MapAt(x, y)
		total, near, nearest := 0, 0, math.Inf(1)
		for _, s := range Spawns() {
			if m, _ := MapAt(s.X, s.Y); m.Name != home.Name || s.VNum >= 9000 || len(s.Spots) > 0 {
				continue
			}
			d := math.Hypot(float64(s.X-x), float64(s.Y-y))
			total += s.Count
			if d < 15000 {
				near += s.Count
			}
			nearest = min(nearest, d)
		}
		if total < 380 || near < 70 || nearest > 8000 {
			t.Errorf("%s: %d monsters, %d within 15000, the nearest pack %.0f out", home.Name, total, near, nearest)
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

// Each first village has the two teachers of every class, one of each.
func TestEveryFirstVillageHasItsTeachers(t *testing.T) {
	teachers := map[string][8]uint16{
		"metin2_map_a1": {20300, 20301, 20302, 20303, 20304, 20305, 20306, 20307},
		"metin2_map_b1": {20320, 20321, 20322, 20323, 20324, 20325, 20326, 20327},
		"metin2_map_c1": {20340, 20341, 20342, 20343, 20344, 20345, 20346, 20347},
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

// A spawn carries its band's time to come back in, as the game reads it: 30 s for every
// dog pack.
func TestASpawnCarriesItsTimeToComeBack(t *testing.T) {
	dogs := 0
	for _, s := range Spawns() {
		if s.VNum == 101 {
			dogs++
			if s.Respawn != 30*time.Second {
				t.Errorf("dogs at (%d, %d) back after %v", s.X, s.Y, s.Respawn)
			}
		}
	}
	if dogs == 0 {
		t.Fatal("no dogs")
	}
}

// A map's Mobs multiplies its packs, rounded and never below one, and no other map's:
// Jinno's packs of 3, 4 and 5 at 2, 1.5 and 0.1, while its leaders, NPCs and stones stay
// one each.
func TestAMapsMobsMultiplyItsPacks(t *testing.T) {
	type place struct {
		vnum uint16
		x, y int32
	}
	count := func(on []Map) map[place]int {
		got := map[place]int{}
		for _, s := range spawnsOn(on) {
			if len(s.Spots) > 0 { // a stone, which stands at one of its spots
				if s.Count != 1 {
					t.Errorf("a stone %d of %d", s.VNum, s.Count)
				}
				continue
			}
			got[place{s.VNum, s.X, s.Y}] += s.Count
		}
		return got
	}
	plain := count(maps)
	for _, c := range []struct {
		mobs float64
		want map[int]int // a pack's count as placed, and as the map holds it
	}{
		{2, map[int]int{3: 6, 4: 8, 5: 10}},
		{1.5, map[int]int{3: 5, 4: 6, 5: 8}},
		{0.1, map[int]int{3: 1, 4: 1, 5: 1}},
	} {
		changed := slices.Clone(maps)
		changed[2].Mobs = c.mobs // metin2_map_c1, Jinno
		got := count(changed)
		for p, n := range plain {
			expect := n
			if m, _ := MapAt(p.x, p.y); m.Name == "metin2_map_c1" && n > 1 {
				expect = c.want[n]
			}
			if got[p] != expect {
				t.Errorf("at %v: %d at (%d, %d) holds %d, want %d", c.mobs, p.vnum, p.x, p.y, got[p], expect)
			}
		}
	}
}

// Every map says how many monsters its packs hold: one it forgot would hold a single
// monster a pack.
func TestEveryMapSaysHowManyMobsItHolds(t *testing.T) {
	for _, m := range maps {
		if m.Mobs <= 0 {
			t.Errorf("%s: Mobs %v", m.Name, m.Mobs)
		}
	}
}

// safeGround is where each first village's fighting stops, as the client's attribute map
// marks it (attr.atr, cell bit 2, no PvP), in rectangles of the map's coordinates: each
// village's squares, and a few spots of their own about the map.
var safeGround = map[string][][4]int32{
	"metin2_map_a1": {{6200, 58800, 9400, 62000}, {39000, 6200, 42200, 9400}, {58600, 67000, 61000, 69400}, {62000, 56000, 67600, 61600}},
	"metin2_map_b1": {{54600, 54400, 57000, 56800}, {61600, 61800, 66000, 66200}, {86600, 110000, 89000, 112400}, {89000, 8000, 91800, 10800}},
	"metin2_map_c1": {{11400, 109000, 15000, 112600}, {13600, 12000, 17200, 15600}, {36000, 61800, 40800, 66600}, {46400, 72000, 49600, 75200}},
}

// town is the map's safe ground and the box its village's NPCs stand in, the guards about
// the map left out.
func town(name string) [][4]int32 {
	box := [4]int32{math.MaxInt32, math.MaxInt32, 0, 0}
	for _, a := range areas(name) {
		if a.vnum >= 9000 && a.vnum != 9009 {
			box = [4]int32{min(box[0], a.x), min(box[1], a.y), max(box[2], a.x), max(box[3], a.y)}
		}
	}
	return append(slices.Clone(safeGround[name]), box)
}

// clearOf is how far (x, y) lies from the nearest of the rectangles.
func clearOf(x, y int32, rects [][4]int32) float64 {
	nearest := math.Inf(1)
	for _, r := range rects {
		dx, dy := max(r[0]-x, 0, x-r[2]), max(r[1]-y, 0, y-r[3])
		nearest = min(nearest, math.Hypot(float64(dx), float64(dy)))
	}
	return nearest
}

// No monster strolls into a town: every pack's ground, its spread and a stroll of 600
// beyond, stays 2000 from the safe ground and the box of the village's NPCs; and every
// stone's spot stays 4000 from the safe ground, as its waves gather round it.
func TestTheTownsAreFreeOfMonsters(t *testing.T) {
	for name, all := range monsterAreas(t) {
		for _, a := range all {
			if d := clearOf(a.x, a.y, town(name)); d < float64(a.spread)+600+2000 {
				t.Errorf("%s: %d at (%d, %d) is %.0f from the town", name, a.vnum, a.x, a.y, d)
			}
		}
		for _, s := range spawnsByMap[name] {
			for _, p := range s.spots {
				if d := clearOf(p.X, p.Y, safeGround[name]); d < 4000 {
					t.Errorf("%s: %d at %v is %.0f from safe ground", name, s.vnum, p, d)
				}
			}
		}
	}
}
