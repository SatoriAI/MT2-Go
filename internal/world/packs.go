package world

// Kind is one kind a pack may hold, as likely as its weight against the others'.
type Kind struct {
	VNum   uint16
	Weight int
}

// A family is the kinds that may stand in one pack: they are kin, so a pack never holds a
// dog beside a bear (docs/mechanics/monsters.md).
type family string

const (
	dogs   family = "dogs"
	wolves family = "wolves"
	boars  family = "boars"
	bears  family = "bears"
	tigers family = "tigers"
)

// A family's pack holds from smallest, the original's three, to largest, rolled each time
// it comes back.
const (
	smallest = 3
	largest  = 5
)

var families = map[family][]uint16{
	dogs:   {101},
	wolves: {102, 103, 104, 105},
	boars:  {108, 109},
	bears:  {110, 111},
	tigers: {114},
}

// band is how far out from its village a kind stands: none before from, more and more
// up to peak, most from there to fall, fewer and fewer to to. The bands overlap, so the
// kinds change gradually, the harder the further out (docs/mechanics/monsters.md).
type band struct{ from, peak, fall, to float64 }

var bands = map[uint16]band{
	101: {1500, 1500, 6000, 12000},
	102: {2000, 6000, 12000, 20000},
	103: {6000, 12000, 18000, 25000},
	108: {7000, 12000, 20000, 25000},
	104: {12000, 18000, 25000, 30000},
	105: {17000, 23000, 30000, 35000},
	109: {22000, 28000, 35000, 40000},
	110: {18000, 22000, 36000, 48000},
	111: {20000, 28000, 46000, 55000},
	114: {40000, 48000, 55000, 55001},
}

// weight is how likely the kind is d out from the village, out of 100 at its most.
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

// kindsAt are the family's kinds a pack d out from its village holds, with their weights.
func kindsAt(f family, d float64) []Kind {
	var kinds []Kind
	for _, vnum := range families[f] {
		if w := bands[vnum].weight(d); w > 0 {
			kinds = append(kinds, Kind{vnum, w})
		}
	}
	return kinds
}

// backAfter is how long a pack d out from its village takes to come back, in seconds: the
// nearer, the sooner.
func backAfter(d float64) int {
	switch {
	case d < 18000:
		return 30
	case d < 30000:
		return 45
	}
	return 60
}
