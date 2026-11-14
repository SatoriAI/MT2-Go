package world

import (
	"math"
	"slices"
	"testing"
)

// The valley's centre and reach, in its own coordinates, as the placing script found them
// (docs/mechanics/maps.md): the open ground nearest the middle, and the arrival's distance
// from it.
var (
	valleyCentre = Spot{74500, 79500}
	valleyReach  = 58483.0
)

// valleyArrival is the arrival in the valley's own coordinates.
func valleyArrival(t *testing.T) Spot {
	t.Helper()
	m, ok := MapNamed(Valley)
	if !ok {
		t.Fatal("no valley")
	}
	return Spot{m.Arrival.X - m.X, m.Arrival.Y - m.Y}
}

// nearArrival is whether a pack, in world coordinates, stands within 12000 of the valley's
// arrival, where its packs are a little smaller.
func nearArrival(s Spawn) bool {
	m, _ := MapNamed(Valley)
	return math.Hypot(float64(s.X-m.Arrival.X), float64(s.Y-m.Arrival.Y)) < 12000
}

// depth is how far in a point of the valley lies from its arrival's ring about the centre.
func depth(x, y int32) float64 {
	return max(0, valleyReach-math.Hypot(float64(x-valleyCentre.X), float64(y-valleyCentre.Y)))
}

func distance(a, b Spot) float64 { return math.Hypot(float64(a.X-b.X), float64(a.Y-b.Y)) }

func TestTheValleyIsAMapAndItsArrivalLiesOnIt(t *testing.T) {
	m, ok := MapNamed(Valley)
	if !ok || m.X != 256000 || m.Y != 665600 || m.Width != 6*cell || m.Height != 6*cell {
		t.Fatalf("valley %+v", m)
	}
	if on, ok := MapAt(m.Arrival.X, m.Arrival.Y); !ok || on.Name != Valley {
		t.Errorf("the arrival %v lies on %q", m.Arrival, on.Name)
	}
	for _, other := range maps {
		if other.Name != Valley && other.Arrival != (Spot{}) {
			t.Errorf("%s, a kingdom's own map, has an arrival", other.Name)
		}
	}
}

// Nothing stands near the arrival: no pack's ground, its 800 and a stroll of 600, within
// 2500 of it, and no stone's spot within 6000.
func TestNothingStandsByTheValleysArrival(t *testing.T) {
	arrival := valleyArrival(t)
	for _, s := range spawnsByMap[Valley] {
		if s.spots == nil && distance(Spot{s.x, s.y}, arrival) < float64(s.spread)+600+2500 {
			t.Errorf("%d %s at (%d, %d)", s.vnum, s.family, s.x, s.y)
		}
		for _, p := range s.spots {
			if distance(p, arrival) < 6000 {
				t.Errorf("%d's spot %v", s.vnum, p)
			}
		}
	}
}

// Each of the valley's packs says how deep it stands, and holds the orcs there: the
// harder towards the centre.
func TestEveryValleyPackStandsAsDeepAsItSays(t *testing.T) {
	for _, s := range spawnsByMap[Valley] {
		if s.family == "" {
			continue
		}
		if s.family != orcs || math.Abs(depth(s.x, s.y)-float64(s.out)) > 1 || len(kindsAt(orcs, float64(s.out))) == 0 {
			t.Errorf("%s at (%d, %d) says %d deep, is %.0f", s.family, s.x, s.y, s.out, depth(s.x, s.y))
		}
	}
}

func TestTheOrcsHardenTowardsTheCentre(t *testing.T) {
	for d, want := range map[float64][]uint16{
		0:     {601},
		10000: {601, 602},
		24000: {602, 603, 604},
		38000: {604, 631, 632, 633, 634},
		55000: {634, 635, 636, 637},
	} {
		var got []uint16
		for _, k := range kindsAt(orcs, d) {
			got = append(got, k.VNum)
		}
		if !equal(got, want) {
			t.Errorf("%.0f deep: %v, want %v", d, got, want)
		}
	}
}

func equal(a, b []uint16) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// The valley holds about 200 packs: its families', a few leaders with their kin, and the
// orc lord, alone at the centre.
func TestTheValleyIsFull(t *testing.T) {
	families, leaders, lords := 0, 0, 0
	for _, s := range spawnsByMap[Valley] {
		switch {
		case s.family != "":
			families++
		case s.vnum == 691:
			lords++
			if s.x != valleyCentre.X || s.y != valleyCentre.Y || s.count != 1 {
				t.Errorf("the lord at (%d, %d), %d of him", s.x, s.y, s.count)
			}
		case s.spots == nil && s.count == 1:
			leaders++
		}
	}
	if families+leaders+lords < 180 || families+leaders+lords > 210 || lords != 1 || leaders != 8 {
		t.Errorf("%d family packs, %d leaders, %d lords", families, leaders, lords)
	}
}

// A leader stands with three of its kin: a 603 with 601s halfway in, a 635 with 633s
// further, a 636 with 631s at the core.
func TestEveryValleyLeaderStandsWithThreeOfItsKinInItsBand(t *testing.T) {
	kin := map[uint16]struct {
		vnum     uint16
		from, to float64
	}{603: {601, 20000, 35000}, 635: {633, 40000, 50000}, 636: {631, 50000, math.Inf(1)}}
	all := spawnsByMap[Valley]
	for i, s := range all {
		k, leads := kin[s.vnum]
		if !leads {
			continue
		}
		if d := depth(s.x, s.y); d < k.from || d >= k.to || i+1 == len(all) {
			t.Fatalf("%d at %.0f deep", s.vnum, d)
		}
		if n := all[i+1]; n.vnum != k.vnum || n.count != 3 || n.x != s.x || n.y != s.y || s.count != 1 {
			t.Errorf("%d with %+v", s.vnum, n)
		}
	}
}

// The valley's stones: three of 8005 in its outer ring, three of 8006 and two of 8007
// further in, one 8008 at the core, each with three spots in its band and 9000 from
// every other spot.
func TestTheValleysStonesStandInTheirBands(t *testing.T) {
	bands := map[uint16]struct {
		n        int
		from, to float64
	}{8005: {3, 0, 20000}, 8006: {3, 15000, 35000}, 8007: {2, 30000, 48000}, 8008: {1, 45000, math.Inf(1)}}
	count := map[uint16]int{}
	var spots []Spot
	for _, s := range spawnsByMap[Valley] {
		if s.spots == nil {
			continue
		}
		count[s.vnum]++
		b := bands[s.vnum]
		if len(s.spots) != 3 {
			t.Errorf("%d with %d spots", s.vnum, len(s.spots))
		}
		for _, p := range s.spots {
			if d := depth(p.X, p.Y); d < b.from || d >= b.to {
				t.Errorf("%d's spot %v at %.0f deep", s.vnum, p, d)
			}
			for _, q := range spots {
				if distance(p, q) < 9000 {
					t.Errorf("spots %v and %v", p, q)
				}
			}
			spots = append(spots, p)
		}
	}
	for vnum, b := range bands {
		if count[vnum] != b.n {
			t.Errorf("%d stones of %d", count[vnum], vnum)
		}
	}
}

// The valley's packs stand apart: 2000 between centres where both lie less than 30000
// deep, 2500 where either lies deeper, and 3500 from every stone's spot.
func TestTheValleysPacksStandApart(t *testing.T) {
	var centres []spawnAt
	var spots []Spot
	for _, s := range spawnsByMap[Valley] {
		switch {
		case s.spots != nil:
			spots = append(spots, s.spots...)
		case len(centres) == 0 || centres[len(centres)-1].x != s.x || centres[len(centres)-1].y != s.y:
			centres = append(centres, s)
		}
	}
	for i, a := range centres {
		for _, o := range centres[i+1:] {
			gap := 2000.0
			if depth(a.x, a.y) >= 30000 || depth(o.x, o.y) >= 30000 {
				gap = 2500
			}
			if d := distance(Spot{a.x, a.y}, Spot{o.x, o.y}); d < gap {
				t.Errorf("packs at (%d, %d) and (%d, %d) are %.0f apart", a.x, a.y, o.x, o.y, d)
			}
		}
		for _, p := range spots {
			if distance(p, Spot{a.x, a.y}) < 3500 {
				t.Errorf("the pack at (%d, %d) is near a stone's spot %v", a.x, a.y, p)
			}
		}
	}
}

// The valley's Mobs multiplies its packs as a first map's does, those by the arrival from
// their 4: 6 to 8 at twice.
func TestTheValleysMobsMultiplyItsPacksByTheArrivalToo(t *testing.T) {
	changed := slices.Clone(maps)
	for i := range changed {
		if changed[i].Name == Valley {
			changed[i].Mobs = 2
		}
	}
	near := 0
	for _, s := range spawnsOn(changed) {
		if m, _ := MapAt(s.X, s.Y); m.Name != Valley || len(s.Kinds) == 0 {
			continue
		}
		most := 10
		if nearArrival(s) {
			most, near = 8, near+1
		}
		if s.MinCount != 6 || s.Count != most {
			t.Errorf("a pack at (%d, %d) of %d to %d", s.X, s.Y, s.MinCount, s.Count)
		}
	}
	if near == 0 {
		t.Error("no pack by the arrival")
	}
}
