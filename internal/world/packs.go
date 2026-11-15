package world

// Kind is one kind a pack may hold, as likely as its weight against the others'.
type Kind struct {
	VNum   uint16
	Weight int
}

// A family is the kinds that may stand in one pack, rolled anew each time it comes back.
// Only the valley's orcs stand in packs (docs/mechanics/maps.md); the first villages'
// monsters stand as the original's do, alone or in small clumps (docs/mechanics/monsters.md).
type family string

const orcs family = "orcs"

// A family's pack holds from smallest, the original's three, to largest, rolled each time
// it comes back.
const (
	smallest = 3
	largest  = 5
)

var families = map[family][]uint16{
	orcs: {601, 602, 603, 604, 631, 632, 633, 634, 635, 636, 637},
}

// band is how far out a kind stands: none before from, more and more up to peak, most
// from there to fall, fewer and fewer to to. The bands overlap, so the kinds change
// gradually. In the valley, out is how far in from its arrival's ring about the centre,
// so the orcs harden towards the middle (docs/mechanics/maps.md).
type band struct{ from, peak, fall, to float64 }

var bands = map[uint16]band{
	601: {0, 0, 12000, 22000},
	602: {4000, 12000, 20000, 30000},
	603: {12000, 20000, 28000, 36000},
	604: {18000, 26000, 34000, 42000},
	631: {26000, 34000, 40000, 46000},
	632: {30000, 36000, 44000, 50000},
	633: {34000, 40000, 48000, 54000},
	634: {36000, 42000, 50000, 56000},
	635: {44000, 50000, 60000, 60001},
	636: {48000, 54000, 60000, 60001},
	637: {52000, 56000, 60000, 60001},
}

// weight is how likely the kind is d out, out of 100 at its most.
func (b band) weight(d float64) int {
	switch {
	case d < b.from || d >= b.to:
		return 0
	case d < b.peak:
		return max(1, int(100*(d-b.from)/(b.peak-b.from)))
	case d < b.fall:
		return 100
	}
	return max(1, int(100*(b.to-d)/(b.to-b.fall)))
}

// kindsAt are the family's kinds a pack d out holds, with their weights.
func kindsAt(f family, d float64) []Kind {
	var kinds []Kind
	for _, vnum := range families[f] {
		if w := bands[vnum].weight(d); w > 0 {
			kinds = append(kinds, Kind{vnum, w})
		}
	}
	return kinds
}

// backAfter is how long a pack d out takes to come back, in seconds: the nearer the
// arrival, the sooner.
func backAfter(d float64) int {
	switch {
	case d < 18000:
		return 30
	case d < 30000:
		return 45
	}
	return 60
}
