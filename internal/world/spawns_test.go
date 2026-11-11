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
}

// leaders are the cursed kinds that lead a pack of their kin.
var leaders = map[uint16]uint16{132: 131, 134: 133, 136: 135, 138: 137}

// monsterAreas are each first village's monster packs, the families' and the cursed.
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

// home is the village a map's packs are measured out from: the box its NPCs stand in and
// the safe ground within 3000 of it, not the safe spots about the map.
func home(name string) [][4]int32 {
	box := town(name)[len(safeGround[name])]
	rects := [][4]int32{box}
	for _, r := range safeGround[name] {
		if clearOf(r[0], r[1], [][4]int32{box}) < 3000 || clearOf(r[2], r[3], [][4]int32{box}) < 3000 ||
			clearOf(r[0], r[3], [][4]int32{box}) < 3000 || clearOf(r[2], r[1], [][4]int32{box}) < 3000 {
			rects = append(rects, r)
		}
	}
	return rects
}

// A family's pack says how far out from its village it stands, as placed, within the
// attribute map's 100; at that distance at least one of its family's kinds stands.
func TestEveryFamilyPackStandsWhereItsKindsDo(t *testing.T) {
	for name, all := range monsterAreas(t) {
		for _, a := range all {
			if a.family == "" {
				continue
			}
			if d := clearOf(a.x, a.y, home(name)); math.Abs(d-float64(a.out)) > 150 || len(kindsAt(a.family, float64(a.out))) == 0 {
				t.Errorf("%s: %s at (%d, %d) says %d out, is %.0f", name, a.family, a.x, a.y, a.out, d)
			}
		}
	}
}

// Each stretch out from a village holds the kinds whose bands cover it, the harder the
// further: dogs and wolves by the gate, bears from 18000 and grizzlies from 20000, tigers
// only from 40000.
func TestTheKindsGetHarderFurtherOut(t *testing.T) {
	for d, want := range map[float64][]uint16{
		3000:  {101, 102},
		10000: {101, 102, 103, 108},
		20000: {103, 104, 105, 108, 110, 111},
		26000: {104, 105, 109, 110, 111},
		45000: {110, 111, 114},
	} {
		var got []uint16
		for f := range families {
			for _, k := range kindsAt(f, d) {
				got = append(got, k.VNum)
			}
		}
		slices.Sort(got)
		if !slices.Equal(got, want) {
			t.Errorf("%.0f out: %v, want %v", d, got, want)
		}
	}
}

// A kind is most likely on its band's plateau, less so towards its edges, and absent
// beyond them.
func TestAKindsWeightRisesAndFalls(t *testing.T) {
	b := bands[110]
	if b.weight(17999) != 0 || b.weight(20000) != 50 || b.weight(30000) != 100 || b.weight(42000) != 50 || b.weight(48000) != 0 {
		t.Errorf("bears weigh %d, %d, %d, %d, %d", b.weight(17999), b.weight(20000), b.weight(30000), b.weight(42000), b.weight(48000))
	}
}

// Each cursed kind stands in its band from the kingdom's start and comes back after 60 s.
func TestEveryCursedPackStandsInItsBand(t *testing.T) {
	for name, all := range monsterAreas(t) {
		for _, a := range all {
			if a.family != "" {
				continue
			}
			band, known := cursedBands[a.vnum]
			if d := fromStart(name, a); !known || d < band.from || d >= band.to || a.respawn != 60 {
				t.Errorf("%s: %d at (%d, %d), %.0f out, back after %d s", name, a.vnum, a.x, a.y, d, a.respawn)
			}
		}
	}
}

// A family's pack holds 3 to 5, as the world serves it; a cursed pack is a leader with
// 3 of its kin or 3 to 5 of one kind.
func TestEveryPackHoldsThreeToFive(t *testing.T) {
	for _, s := range Spawns() {
		if len(s.Kinds) > 0 && (s.MinCount != 3 || s.Count != 5) {
			t.Errorf("a pack at (%d, %d) of %d to %d", s.X, s.Y, s.MinCount, s.Count)
		}
	}
	for name, all := range monsterAreas(t) {
		for centre, pack := range packs(all) {
			switch {
			case pack[0].family != "":
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

// Packs stand apart: 2000 between centres where both lie within 30000 of the village, 2500
// where either lies further, and 4000 from a cursed pack, which attacks on sight; and their
// centres 2500 from every NPC and 3500 from every stone's spot.
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
				gap := 2000.0
				switch {
				case a.family == "" || o.family == "":
					gap = 4000
				case a.out >= 30000 || o.out >= 30000:
					gap = 2500
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
					t.Errorf("%s: the pack at (%d, %d) is near a stone's spot %v", name, a.x, a.y, p)
				}
			}
		}
	}
}

// Each first village's fields are full, as the world serves them, its Mobs applied: some
// 800 monsters on average, the middle of each pack's 3 to 5, at least 60 of them within
// 10000 of the village, and fewer and fewer further out past 20000.
func TestEveryFirstVillagesFieldsAreFull(t *testing.T) {
	for name, all := range monsterAreas(t) {
		var bands [11]float64
		total := 0.0
		for _, a := range all {
			if a.family == "" {
				continue
			}
			bands[min(int(a.out/5000), 10)] += 4
			total += 4
		}
		if total < 760 || bands[0]+bands[1] < 60 {
			t.Errorf("%s: %.0f monsters, %.0f within 10000 of the village", name, total, bands[0]+bands[1])
		}
		for i := 4; i < 10; i++ {
			if bands[i+1] > bands[i] {
				t.Errorf("%s: more out at %d-%d (%.0f) than at %d-%d (%.0f)", name, (i+1)*5, (i+2)*5, bands[i+1], i*5, (i+1)*5, bands[i])
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

// A family's pack carries its time to come back, as the game reads it: 30 s within 18000
// of the village, 45 within 30000, 60 further out.
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

// A map's Mobs multiplies its packs, rounded and never below one, and no other map's:
// Jinno's families' packs of 3 to 5 become 6 to 10 at 2 and 1 to 1 at 0.1, its cursed
// packs likewise, while its leaders, NPCs and stones stay one each.
func TestAMapsMobsMultiplyItsPacks(t *testing.T) {
	for _, c := range []struct {
		mobs        float64
		least, most int
		cursed      map[int]int // a cursed pack's count as placed, and as the map holds it
	}{{2, 6, 10, map[int]int{3: 6, 4: 8, 5: 10}}, {1.5, 5, 8, map[int]int{3: 5, 4: 6, 5: 8}}, {0.1, 1, 1, map[int]int{3: 1, 4: 1, 5: 1}}} {
		changed := slices.Clone(maps)
		changed[2].Mobs = c.mobs // metin2_map_c1, Jinno
		plain := spawnsOn(maps)
		for i, s := range spawnsOn(changed) {
			if m, _ := MapAt(s.X, s.Y); len(s.Kinds) == 0 && len(s.Spots) == 0 && m.Name == "metin2_map_c1" {
				want := plain[i].Count
				if want > 1 {
					want = c.cursed[want]
				}
				if s.Count != want {
					t.Errorf("at %v: %d at (%d, %d) holds %d, want %d", c.mobs, s.VNum, s.X, s.Y, s.Count, want)
				}
			}
			m, _ := MapAt(s.X, s.Y)
			switch {
			case len(s.Kinds) == 0:
			case m.Name == "metin2_map_c1" && (s.MinCount != c.least || s.Count != c.most):
				t.Errorf("at %v: a pack of %d to %d", c.mobs, s.MinCount, s.Count)
			case m.Name != "metin2_map_c1" && (s.MinCount != 3 || s.Count != 5):
				t.Errorf("at %v: %s's pack of %d to %d", c.mobs, m.Name, s.MinCount, s.Count)
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
