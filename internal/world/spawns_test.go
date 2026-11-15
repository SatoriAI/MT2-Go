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

// Each first village has its blacksmith.
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

// cursedBands are how far from its kingdom's start each cursed kind stands, which attacks
// on sight: in the far bands only.
var cursedBands = map[uint16]struct{ from, to float64 }{
	131: {30000, 42000}, 132: {30000, 42000},
	133: {30000, 55000}, 137: {30000, 55000}, // kin of a far leader too
	134: {42000, 55000}, 135: {42000, 55000}, 136: {42000, 55000}, 138: {42000, 55000}, 139: {42000, 55000},
	115: {48000, 60000},
}

// leaders are the cursed kinds that lead a pack of their kin.
var leaders = map[uint16]uint16{132: 131, 134: 133, 136: 135, 138: 137}

// The field kinds: the original's, and our edge's bears and tigers, which its first maps
// lack, each with how long it takes to come back, in seconds.
var (
	originalKinds = map[uint16]int{101: 5, 102: 15, 103: 15, 104: 30, 105: 30, 106: 40, 107: 40, 108: 60, 109: 60, 110: 60}
	edgeKinds     = map[uint16]int{111: 60, 112: 60, 113: 60, 114: 60}
)

func fieldKind(vnum uint16) bool {
	_, original := originalKinds[vnum]
	_, edge := edgeKinds[vnum]
	return original || edge
}

// monsterAreas are each first village's monster spawns, the fields' and the cursed.
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

// packs are a village's monster spawns grouped by the centre they share: a clump of the
// field, or a cursed pack.
func packs(all []spawnAt) map[[2]int32][]spawnAt {
	byCentre := map[[2]int32][]spawnAt{}
	for _, a := range all {
		byCentre[[2]int32{a.x, a.y}] = append(byCentre[[2]int32{a.x, a.y}], a)
	}
	return byCentre
}

// fieldDistances are how far from the start each of a village's field monsters of the
// kinds stands, one entry a monster, nearest first.
func fieldDistances(name string, all []spawnAt, kinds func(uint16) bool) []float64 {
	var d []float64
	for _, a := range all {
		if kinds(a.vnum) {
			for range a.count {
				d = append(d, fromStart(name, a))
			}
		}
	}
	slices.Sort(d)
	return d
}

// Every monster of a first village is of a field kind or a cursed one.
func TestEveryFirstVillagesMonsterIsOfAKnownKind(t *testing.T) {
	for name, all := range monsterAreas(t) {
		for _, a := range all {
			if _, cursed := cursedBands[a.vnum]; !cursed && !fieldKind(a.vnum) {
				t.Errorf("%s: %d at (%d, %d)", name, a.vnum, a.x, a.y)
			}
		}
	}
}

// Each first village's fields hold as many of each kind as the original's, some 930 to
// 945, every one of its kinds at least 25, and our edge's 80 besides, or a few fewer where
// open ground ran short.
func TestEveryFirstVillagesFieldsHoldTheOriginalsNumbers(t *testing.T) {
	for name, all := range monsterAreas(t) {
		count := map[uint16]int{}
		for _, a := range all {
			count[a.vnum] += a.count
		}
		original, edge := 0, 0
		for vnum := range originalKinds {
			original += count[vnum]
			if count[vnum] < 25 {
				t.Errorf("%s: %d of %d", name, count[vnum], vnum)
			}
		}
		for vnum := range edgeKinds {
			edge += count[vnum]
		}
		if original < 900 || original > 950 || edge < 75 || edge > 80 {
			t.Errorf("%s: %d of the original's kinds, %d of the edge's", name, original, edge)
		}
	}
}

// As the original's, most of the fields' clumps are one monster, the rest small, up to 11,
// with no more than two kinds in one.
func TestMostFieldClumpsAreOneMonster(t *testing.T) {
	for name, all := range monsterAreas(t) {
		alone, clumps := 0, 0
		for centre, pack := range packs(all) {
			if !fieldKind(pack[0].vnum) {
				continue
			}
			clumps++
			size := 0
			for _, a := range pack {
				size += a.count
				if !fieldKind(a.vnum) {
					t.Errorf("%s: a cursed %d in the clump at %v", name, a.vnum, centre)
				}
			}
			if size == 1 {
				alone++
			}
			if size > 11 || len(pack) > 2 {
				t.Errorf("%s: a clump of %d at %v", name, size, centre)
			}
		}
		if alone*2 < clumps {
			t.Errorf("%s: %d of %d clumps alone", name, alone, clumps)
		}
	}
}

// The fields reach as far as the original's, past 650 m from the start, and the kinds get
// harder further out: the dogs and first wolves nearest, then the blue wolves, the grey
// ones, and our edge's beyond three quarters of the original's monsters.
func TestTheFieldsReachFarAndHardenOutwards(t *testing.T) {
	median := func(d []float64) float64 { return d[len(d)/2] }
	of := func(vnums ...uint16) func(uint16) bool {
		return func(v uint16) bool { return slices.Contains(vnums, v) }
	}
	for name, all := range monsterAreas(t) {
		original := fieldDistances(name, all, func(v uint16) bool { _, ok := originalKinds[v]; return ok })
		if original[len(original)-1] < 65000 {
			t.Errorf("%s: the fields end %.0f from the start", name, original[len(original)-1])
		}
		first, blue, grey := median(fieldDistances(name, all, of(101, 102, 103))), median(fieldDistances(name, all, of(104, 105))),
			median(fieldDistances(name, all, of(106, 107)))
		if first >= blue || blue >= grey {
			t.Errorf("%s: the first wolves stand %.0f out, the blue %.0f, the grey %.0f", name, first, blue, grey)
		}
		edge := fieldDistances(name, all, func(v uint16) bool { _, ok := edgeKinds[v]; return ok })
		if quarter := original[len(original)*3/4]; edge[0] < quarter {
			t.Errorf("%s: an edge kind %.0f out, within three quarters' %.0f", name, edge[0], quarter)
		}
	}
}

// Each field kind comes back as soon as the original's: the dogs after 5 s, the first wolves
// after 15, the blue after 30 but Shinsoo's alpha blue after 15, the grey after 40, the
// boars and bears after 60.
func TestEveryFieldKindComesBackAsTheOriginals(t *testing.T) {
	for name, all := range monsterAreas(t) {
		for _, a := range all {
			want, ok := originalKinds[a.vnum]
			if name == "metin2_map_a1" && a.vnum == 105 {
				want = 15
			}
			if !ok {
				want, ok = edgeKinds[a.vnum]
			}
			if ok && a.respawn != want {
				t.Errorf("%s: %d at (%d, %d) back after %d s, want %d", name, a.vnum, a.x, a.y, a.respawn, want)
			}
		}
	}
}

// Each cursed kind stands in its band from the kingdom's start and comes back after 60 s.
func TestEveryCursedPackStandsInItsBand(t *testing.T) {
	for name, all := range monsterAreas(t) {
		for _, a := range all {
			band, cursed := cursedBands[a.vnum]
			if !cursed {
				continue
			}
			if d := fromStart(name, a); d < band.from || d >= band.to || a.respawn != 60 {
				t.Errorf("%s: %d at (%d, %d), %.0f out, back after %d s", name, a.vnum, a.x, a.y, d, a.respawn)
			}
		}
	}
}

// A family's pack holds 3 to 5, as the world serves it, or 3 to 4 by the valley's arrival
// (valley_test.go); a cursed pack is a leader with 3 of its kin or 3 to 5 of one kind.
func TestEveryPackHoldsThreeToFive(t *testing.T) {
	for _, s := range Spawns() {
		most := 5
		if nearArrival(s) {
			most = 4
		}
		if len(s.Kinds) > 0 && (s.MinCount != 3 || s.Count != most) {
			t.Errorf("a pack at (%d, %d) of %d to %d", s.X, s.Y, s.MinCount, s.Count)
		}
	}
	for name, all := range monsterAreas(t) {
		for centre, pack := range packs(all) {
			switch {
			case fieldKind(pack[0].vnum):
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

// Clumps stand apart: 1000 between centres, as the original's monsters stand some 10 m
// apart, and 4000 from a cursed pack, which attacks on sight; and their centres 2500 from
// every NPC and 3500 from every stone.
func TestPacksStandApart(t *testing.T) {
	for name, all := range monsterAreas(t) {
		var centres []spawnAt
		for _, p := range packs(all) {
			centres = append(centres, p[0])
		}
		var spots []Spot
		for _, s := range spawnsByMap[name] {
			spots = append(spots, s.spots...)
		}
		for i, a := range centres {
			for _, o := range centres[i+1:] {
				gap := 1000.0
				if !fieldKind(a.vnum) || !fieldKind(o.vnum) {
					gap = 4000
				}
				if d := math.Hypot(float64(o.x-a.x), float64(o.y-a.y)); d < gap {
					t.Errorf("%s: packs at (%d, %d) and (%d, %d) are %.0f apart", name, a.x, a.y, o.x, o.y, d)
				}
			}
			for _, n := range areas(name) {
				if n.vnum >= 9000 && math.Hypot(float64(n.x-a.x), float64(n.y-a.y)) < 2500 {
					t.Errorf("%s: the pack at (%d, %d) is near NPC %d", name, a.x, a.y, n.vnum)
				}
			}
			for _, p := range spots {
				if math.Hypot(float64(p.X-a.x), float64(p.Y-a.y)) < 3500 {
					t.Errorf("%s: the pack at (%d, %d) is near a stone %v", name, a.x, a.y, p)
				}
			}
		}
	}
}

// Each first village has as many Metin stones as the original's, 36 in Shinsoo's and 61 in
// the others', each standing at one spot of its own: Metin Cierpienia, Walki and Bitwy.
func TestEveryFirstVillageHasTheOriginalsStones(t *testing.T) {
	for village, want := range map[string]int{"metin2_map_a1": 36, "metin2_map_b1": 61, "metin2_map_c1": 61} {
		count := map[uint16]int{}
		for _, s := range spawnsByMap[village] {
			if len(s.spots) > 0 {
				count[s.vnum]++
				if len(s.spots) != 1 || s.count > 1 || s.spread != 0 {
					t.Errorf("%s: %+v", village, s)
				}
			}
		}
		if count[8001]+count[8002]+count[8003] != want || len(count) != 3 {
			t.Errorf("%s: stones %v, want %d", village, count, want)
		}
	}
}

// The stones are the stronger the further from the kingdom's start: every Metin Walki
// further than every Metin Cierpienia, every Metin Bitwy further than every Metin Walki.
// Each is at least 3500 from any monster area, 8000 from any NPC, and 5000 from any other
// stone, and lies on its map.
func TestStonesHardenOutwardsAndStandApart(t *testing.T) {
	for _, e := range []account.Empire{account.EmpireShinsoo, account.EmpireChunjo, account.EmpireJinno} {
		x, y, _ := e.StartPosition()
		m, _ := MapAt(x, y)
		var spots []Spot
		nearest, farthest := map[uint16]float64{}, map[uint16]float64{}
		for _, s := range spawnsByMap[m.Name] {
			spots = append(spots, s.spots...)
		}
		for _, s := range spawnsByMap[m.Name] {
			for _, p := range s.spots {
				if got, ok := MapAt(m.X+p.X, m.Y+p.Y); !ok || got.Name != m.Name {
					t.Errorf("%s: %d at %v is off the map", m.Name, s.vnum, p)
				}
				d := math.Hypot(float64(p.X-(x-m.X)), float64(p.Y-(y-m.Y)))
				if n, ok := nearest[s.vnum]; !ok || d < n {
					nearest[s.vnum] = d
				}
				farthest[s.vnum] = max(farthest[s.vnum], d)
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
					if o != p && math.Hypot(float64(o.X-p.X), float64(o.Y-p.Y)) < 5000 {
						t.Errorf("%s: %v is near %v", m.Name, p, o)
					}
				}
			}
		}
		if farthest[8001] > nearest[8002] || farthest[8002] > nearest[8003] {
			t.Errorf("%s: stones from %v to %v", m.Name, nearest, farthest)
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

// A spawn carries where its NPC looks, as the game reads it: Jinno's teachers with their
// backs to the temple, which faces west.
func TestASpawnCarriesWhereItsNPCLooks(t *testing.T) {
	teachers := 0
	for _, s := range Spawns() {
		if s.VNum >= 20340 && s.VNum <= 20347 {
			teachers++
			if s.Facing != 270 {
				t.Errorf("teacher %d faces %v", s.VNum, s.Facing)
			}
		}
	}
	if teachers != 8 {
		t.Fatalf("%d of Jinno's teachers", teachers)
	}
}

// A valley pack carries its time to come back, as the game reads it: 30 s within 18000 in
// from the arrival's ring, 45 within 30000, 60 further in.
func TestASpawnCarriesItsTimeToComeBack(t *testing.T) {
	for _, m := range maps {
		for _, s := range spawnsByMap[m.Name] {
			if s.family == "" {
				continue
			}
			if got := packOf(m, s).Respawn; got != time.Duration(backAfter(float64(s.out)))*time.Second {
				t.Errorf("%s at (%d, %d), %d out, back after %v", s.family, s.x, s.y, s.out, got)
			}
		}
	}
	if backAfter(17999) != 30 || backAfter(18000) != 45 || backAfter(30000) != 60 {
		t.Error("the bands' times")
	}
}

// A map's Mobs multiplies its clumps and packs, rounded and never below one, and no other
// map's: Jinno's clumps and cursed packs of 2 to 11 hold twice as many at 2, half again,
// rounded, at 1.5 and one at 0.1, while a monster alone, its leaders, NPCs and stones stay
// one each.
func TestAMapsMobsMultiplyItsPacks(t *testing.T) {
	for mobs, want := range map[float64][12]int{
		2:   {0, 1, 4, 6, 8, 10, 12, 14, 16, 18, 20, 22},
		1.5: {0, 1, 3, 5, 6, 8, 9, 11, 12, 14, 15, 17},
		0.1: {0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
	} {
		changed := slices.Clone(maps)
		changed[2].Mobs = mobs // metin2_map_c1, Jinno
		plain := spawnsOn(maps)
		for i, s := range spawnsOn(changed) {
			if len(s.Kinds) > 0 {
				continue // the valley's packs (valley_test.go)
			}
			m, _ := MapAt(s.X, s.Y)
			if got := s.Count; m.Name == "metin2_map_c1" && got != want[plain[i].Count] || m.Name != "metin2_map_c1" && got != plain[i].Count {
				t.Errorf("at %v: %s's %d at (%d, %d) holds %d, placed %d", mobs, m.Name, s.VNum, s.X, s.Y, got, plain[i].Count)
			}
			if len(s.Spots) > 0 && s.Count != 1 {
				t.Errorf("a stone %d of %d", s.VNum, s.Count)
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
	"metin2_map_a1": {{6100, 58800, 9300, 62000}, {39000, 6100, 42200, 9300}, {58600, 67000, 61000, 69400}, {61900, 55900, 67500, 61500}},
	"metin2_map_b1": {{54600, 54400, 57000, 56800}, {61600, 61700, 66000, 66100}, {86500, 110000, 88900, 112400}, {88900, 7900, 91700, 10700}},
	"metin2_map_c1": {{11300, 108900, 14900, 112500}, {13500, 12000, 17100, 15600}, {35900, 61800, 40700, 66600}, {46400, 71900, 49600, 75100}},
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

// No monster strolls into a town or onto a kingdom's start: every pack's ground, its spread
// and a stroll of 600 beyond, stays 2000 from the safe ground, the box of the village's NPCs
// and the start, which lies outside both; and every stone's spot stays 4000 from the town,
// as its waves gather round it.
func TestTheTownsAreFreeOfMonsters(t *testing.T) {
	for name, all := range monsterAreas(t) {
		for _, a := range all {
			if d := clearOf(a.x, a.y, town(name)); d < float64(a.spread)+600+2000 {
				t.Errorf("%s: %d at (%d, %d) is %.0f from the town", name, a.vnum, a.x, a.y, d)
			}
		}
		for _, a := range all {
			if d := fromStart(name, a); d < float64(a.spread)+600+2000 {
				t.Errorf("%s: %d at (%d, %d) is %.0f from the start", name, a.vnum, a.x, a.y, d)
			}
		}
		for _, s := range spawnsByMap[name] {
			for _, p := range s.spots {
				if d := clearOf(p.X, p.Y, town(name)); d < 4000 {
					t.Errorf("%s: %d at %v is %.0f from the town", name, s.vnum, p, d)
				}
			}
		}
	}
}
