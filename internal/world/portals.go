package world

// Way is where a portal leads.
type Way string

const (
	ToValley Way = "valley" // Seungryong Valley's arrival, from its level
	ToHome   Way = "home"   // the walker's kingdom's way back from the valley
	Closed   Way = "closed" // a road not open yet, which tells the walker so
)

// Portal is one of the gates the client draws at its minimap's WARP marks: whoever walks
// within portalRadius of At goes the Way it leads. Land is where one coming the other way
// through it arrives, a step inside, on open ground: only a gate to the valley has one.
type Portal struct {
	At   Spot
	Way  Way
	Land Spot
}

// portalRadius is how near a gate's centre a walker goes through it: within the
// platform's inner half-width, 6 m, which the arch's 4.4 m fits inside.
const portalRadius = 600

// portalsByMap are each map's gates, at the client's WARP marks in the map's own
// coordinates, and where one coming through lands, 20 m inside, as the client session
// checked open. Each first village's gate named for its kingdom's second map, which lies
// further from the village than the other, leads to the valley until that map exists;
// the other, named for the old man, and the valley's gates to the second maps stay
// closed. The valley's southern gate leads home (docs/mechanics/maps.md).
var portalsByMap = map[string][]Portal{
	"metin2_map_a1": {{At: Spot{40500, 7300}, Way: ToValley, Land: Spot{40500, 9300}}, {At: Spot{7600, 60100}, Way: Closed}},
	"metin2_map_b1": {{At: Spot{87600, 112700}, Way: ToValley, Land: Spot{87600, 110700}}, {At: Spot{90800, 8700}, Way: Closed}},
	"metin2_map_c1": {{At: Spot{13700, 12600}, Way: ToValley, Land: Spot{12290, 14010}}, {At: Spot{12500, 111800}, Way: Closed}},
	Valley:          {{At: Spot{64000, 143600}, Way: ToHome}, {At: Spot{147200, 7300}, Way: Closed}, {At: Spot{13100, 74600}, Way: Closed}},
}

// PortalAt returns the gate the point lies within, in world coordinates, or false.
func PortalAt(x, y int32) (Portal, bool) {
	m, ok := MapAt(x, y)
	if !ok {
		return Portal{}, false
	}
	for _, p := range portalsByMap[m.Name] {
		dx, dy := int64(x-m.X-p.At.X), int64(y-m.Y-p.At.Y)
		if dx*dx+dy*dy <= portalRadius*portalRadius {
			return Portal{At: Spot{m.X + p.At.X, m.Y + p.At.Y}, Way: p.Way, Land: Spot{m.X + p.Land.X, m.Y + p.Land.Y}}, true
		}
	}
	return Portal{}, false
}

// HomeGate is where one coming home from the valley lands on the map holding (x, y), the
// kingdom's start: a step inside its gate to the valley.
func HomeGate(x, y int32) (Spot, bool) {
	m, ok := MapAt(x, y)
	if !ok {
		return Spot{}, false
	}
	for _, p := range portalsByMap[m.Name] {
		if p.Way == ToValley {
			return Spot{m.X + p.Land.X, m.Y + p.Land.Y}, true
		}
	}
	return Spot{}, false
}
