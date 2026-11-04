package world

// Spawn is a place where the world puts mobs: Count of the kind VNum, each at a random
// point within Spread of X, Y.
type Spawn struct {
	VNum   uint16
	X, Y   int32 // world coordinates
	Spread int32
	Count  int
	Facing float32 // degrees; NPCs keep it, monsters turn as they walk
}

// spawnAt is a spawn written in the map's own coordinates, as the client's map files and
// the minimap give them.
type spawnAt struct {
	vnum   uint16
	x, y   int32
	spread int32
	count  int
	facing float32
}

// The village NPCs stand at the client's own minimap marks for them
// (locale/pl/map/metin2_map_*_point.txt): weapons 9001, armour 9002, general store 9003,
// the storekeeper 9005, the old lady 9006, and the fishermen 9009. The monster areas are
// this server's, placed in the fields around each village and checked by eye in the client.
var spawnsByMap = map[string][]spawnAt{
	"metin2_map_a1": {
		{vnum: 9001, x: 59600, y: 55700}, {vnum: 9002, x: 59600, y: 60500}, {vnum: 9003, x: 67800, y: 56500},
		{vnum: 9005, x: 67400, y: 60700}, {vnum: 9006, x: 63300, y: 62200},
		{vnum: 9009, x: 35400, y: 32700}, {vnum: 9009, x: 32500, y: 40500}, {vnum: 9009, x: 25100, y: 87400},
		{vnum: 9009, x: 19500, y: 93700}, {vnum: 9009, x: 30400, y: 118600}, {vnum: 9009, x: 52500, y: 114700},
		{vnum: 9009, x: 59100, y: 116600}, {vnum: 9009, x: 68000, y: 37900}, {vnum: 9009, x: 82100, y: 29700},
		{vnum: 9009, x: 73100, y: 65900},
		{vnum: 101, x: 75000, y: 78000, spread: 2500, count: 6}, {vnum: 101, x: 52000, y: 78000, spread: 2500, count: 6},
		{vnum: 102, x: 78000, y: 92000, spread: 2500, count: 6}, {vnum: 103, x: 50000, y: 92000, spread: 2500, count: 6},
		{vnum: 108, x: 85000, y: 65000, spread: 2500, count: 6},
	},
	"metin2_map_b1": {
		{vnum: 9001, x: 67600, y: 66200}, {vnum: 9002, x: 67600, y: 61700}, {vnum: 9003, x: 59000, y: 68900},
		{vnum: 9005, x: 60900, y: 59600}, {vnum: 9006, x: 58800, y: 63300},
		{vnum: 9009, x: 67500, y: 53900}, {vnum: 9009, x: 74200, y: 87900}, {vnum: 9009, x: 20600, y: 55200},
		{vnum: 9009, x: 21800, y: 67900}, {vnum: 9009, x: 22100, y: 86500}, {vnum: 9009, x: 23100, y: 97600},
		{vnum: 9009, x: 23400, y: 107700},
		{vnum: 101, x: 70000, y: 45000, spread: 2500, count: 6}, {vnum: 101, x: 48000, y: 45000, spread: 2500, count: 6},
		{vnum: 102, x: 72000, y: 32000, spread: 2500, count: 6}, {vnum: 103, x: 45000, y: 32000, spread: 2500, count: 6},
		{vnum: 108, x: 80000, y: 55000, spread: 2500, count: 6},
	},
	"metin2_map_c1": {
		{vnum: 9001, x: 43000, y: 60800}, {vnum: 9002, x: 40300, y: 58500}, {vnum: 9003, x: 38300, y: 69300},
		{vnum: 9005, x: 31500, y: 56000}, {vnum: 9006, x: 41700, y: 67100},
		{vnum: 9009, x: 71100, y: 23400}, {vnum: 9009, x: 73200, y: 39000}, {vnum: 9009, x: 77200, y: 47200},
		{vnum: 9009, x: 76300, y: 80900}, {vnum: 9009, x: 77300, y: 90500}, {vnum: 9009, x: 42800, y: 48000},
		{vnum: 9009, x: 38600, y: 84900}, {vnum: 9009, x: 81900, y: 31500},
		{vnum: 101, x: 52000, y: 80000, spread: 2500, count: 6}, {vnum: 101, x: 30000, y: 82000, spread: 2500, count: 6},
		{vnum: 102, x: 55000, y: 95000, spread: 2500, count: 6}, {vnum: 103, x: 25000, y: 95000, spread: 2500, count: 6},
		{vnum: 108, x: 62000, y: 70000, spread: 2500, count: 6},
	},
}

// Spawns returns every place the world puts mobs, in world coordinates.
func Spawns() []Spawn {
	var all []Spawn
	for _, m := range maps {
		for _, s := range spawnsByMap[m.Name] {
			count := max(s.count, 1)
			all = append(all, Spawn{VNum: s.vnum, X: m.X + s.x, Y: m.Y + s.y, Spread: s.spread, Count: count, Facing: s.facing})
		}
	}
	return all
}
