package world

// Spawn is a place where the world puts mobs: Count of the kind VNum, each at a random
// point within Spread of X, Y, or, for a stone, at one of its Spots picked afresh each
// time it comes back.
type Spawn struct {
	VNum   uint16
	X, Y   int32 // world coordinates
	Spread int32
	Count  int
	Spots  []Spot
}

// Spot is a point in world coordinates.
type Spot struct{ X, Y int32 }

// spawnAt is a spawn written in the map's own coordinates, as the client's map files and
// the minimap give them.
type spawnAt struct {
	vnum   uint16
	x, y   int32
	spread int32
	count  int
	spots  []Spot // in the map's coordinates
}

// The village NPCs stand at the client's minimap marks (locale/pl/map/*_point.txt). The
// teachers and the blacksmith, which the marks leave out, stand in a row through the old
// lady at this server's spots, not checked against the terrain: the warrior's and then
// the ninja's west of her, the sura's and the blacksmith east. The monster areas are on
// open, gentle ground of the client's terrain: a ring 10000 to 30000 from each start, and
// one of stronger kinds 30000 to 55000 out; each Metin stone stands at one of three spots
// of the same ground, the stronger the further out (docs/mechanics/stones.md).
var spawnsByMap = map[string][]spawnAt{
	"metin2_map_a1": {
		{vnum: 9001, x: 59600, y: 55700}, {vnum: 9002, x: 59600, y: 60500}, {vnum: 9003, x: 67800, y: 56500},
		{vnum: 9005, x: 67400, y: 60700}, {vnum: 9006, x: 63300, y: 62200},
		{vnum: 20304, x: 63800, y: 62200}, {vnum: 20305, x: 64300, y: 62200}, {vnum: 20016, x: 64800, y: 62200},
		{vnum: 20300, x: 62800, y: 62200}, {vnum: 20301, x: 62300, y: 62200},
		{vnum: 20302, x: 61800, y: 62200}, {vnum: 20303, x: 61300, y: 62200},
		{vnum: 9009, x: 35400, y: 32700}, {vnum: 9009, x: 32500, y: 40500}, {vnum: 9009, x: 25100, y: 87400},
		{vnum: 9009, x: 19500, y: 93700}, {vnum: 9009, x: 30400, y: 118600}, {vnum: 9009, x: 52500, y: 114700},
		{vnum: 9009, x: 59100, y: 116600}, {vnum: 9009, x: 68000, y: 37900}, {vnum: 9009, x: 82100, y: 29700},
		{vnum: 9009, x: 73100, y: 65900},
		{vnum: 101, x: 64000, y: 80500, spread: 2000, count: 6}, {vnum: 101, x: 83500, y: 71500, spread: 2000, count: 6},
		{vnum: 102, x: 80500, y: 58000, spread: 2000, count: 6}, {vnum: 103, x: 53500, y: 88000, spread: 2000, count: 6},
		{vnum: 108, x: 83500, y: 82000, spread: 2000, count: 6},
		{vnum: 104, x: 46000, y: 95500, spread: 2000, count: 6},
		{vnum: 105, x: 35500, y: 92500, spread: 2000, count: 3}, {vnum: 109, x: 35500, y: 92500, spread: 2000, count: 3},
		{vnum: 110, x: 26500, y: 50500, spread: 2000, count: 3}, {vnum: 111, x: 26500, y: 50500, spread: 2000, count: 3},
		{vnum: 114, x: 20500, y: 43000, spread: 2000, count: 6},
		{vnum: 8001, spots: []Spot{{75000, 99000}, {36000, 81000}, {42000, 61000}}},
		{vnum: 8001, spots: []Spot{{48000, 40000}, {51000, 99000}, {84000, 42000}}},
		{vnum: 8001, spots: []Spot{{88000, 57000}, {92000, 83000}, {63000, 104000}}},
		{vnum: 8002, spots: []Spot{{92000, 102000}, {34000, 97000}, {46000, 109000}}},
		{vnum: 8002, spots: []Spot{{30000, 50000}, {22000, 80000}, {90000, 33000}}},
		{vnum: 8003, spots: []Spot{{75000, 17000}, {13000, 56000}, {12000, 71000}}},
		{vnum: 8003, spots: []Spot{{46000, 19000}, {25000, 33000}, {17000, 44000}}},
	},
	"metin2_map_b1": {
		{vnum: 9001, x: 67600, y: 66200}, {vnum: 9002, x: 67600, y: 61700}, {vnum: 9003, x: 59000, y: 68900},
		{vnum: 9005, x: 60900, y: 59600}, {vnum: 9006, x: 58800, y: 63300},
		{vnum: 20324, x: 59300, y: 63300}, {vnum: 20325, x: 59800, y: 63300}, {vnum: 20016, x: 60300, y: 63300},
		{vnum: 20320, x: 58300, y: 63300}, {vnum: 20321, x: 57800, y: 63300},
		{vnum: 20322, x: 57300, y: 63300}, {vnum: 20323, x: 56800, y: 63300},
		{vnum: 9009, x: 67500, y: 53900}, {vnum: 9009, x: 74200, y: 87900}, {vnum: 9009, x: 20600, y: 55200},
		{vnum: 9009, x: 21800, y: 67900}, {vnum: 9009, x: 22100, y: 86500}, {vnum: 9009, x: 23100, y: 97600},
		{vnum: 9009, x: 23400, y: 107700},
		{vnum: 101, x: 64000, y: 44500, spread: 2000, count: 6}, {vnum: 101, x: 65500, y: 35500, spread: 2000, count: 6},
		{vnum: 102, x: 38500, y: 55000, spread: 2000, count: 6}, {vnum: 103, x: 58000, y: 31000, spread: 2000, count: 6},
		{vnum: 108, x: 88000, y: 64000, spread: 2000, count: 6},
		{vnum: 104, x: 85000, y: 73000, spread: 2000, count: 6},
		{vnum: 105, x: 35500, y: 28000, spread: 2000, count: 3}, {vnum: 109, x: 35500, y: 28000, spread: 2000, count: 3},
		{vnum: 110, x: 37000, y: 91000, spread: 2000, count: 3}, {vnum: 111, x: 37000, y: 91000, spread: 2000, count: 3},
		{vnum: 114, x: 77500, y: 7000, spread: 2000, count: 6},
		{vnum: 8001, spots: []Spot{{30000, 70000}, {40000, 28000}, {27000, 49000}}},
		{vnum: 8001, spots: []Spot{{83000, 28000}, {62000, 19000}, {92000, 63000}}},
		{vnum: 8001, spots: []Spot{{83000, 76000}, {86000, 49000}, {41000, 80000}}},
		{vnum: 8002, spots: []Spot{{64000, 98000}, {76000, 16000}, {55000, 89000}}},
		{vnum: 8002, spots: []Spot{{43000, 95000}, {87000, 89000}, {36000, 16000}}},
		{vnum: 8003, spots: []Spot{{92000, 9000}, {31000, 100000}, {62000, 108000}}},
		{vnum: 8003, spots: []Spot{{52000, 105000}, {74000, 6000}, {86000, 16000}}},
	},
	"metin2_map_c1": {
		{vnum: 9001, x: 43000, y: 60800}, {vnum: 9002, x: 40300, y: 58500}, {vnum: 9003, x: 38300, y: 69300},
		{vnum: 9005, x: 31500, y: 56000}, {vnum: 9006, x: 41700, y: 67100},
		{vnum: 20344, x: 42200, y: 67100}, {vnum: 20345, x: 42700, y: 67100}, {vnum: 20016, x: 43200, y: 67100},
		{vnum: 20340, x: 41200, y: 67100}, {vnum: 20341, x: 40700, y: 67100},
		{vnum: 20342, x: 40200, y: 67100}, {vnum: 20343, x: 39700, y: 67100},
		{vnum: 9009, x: 71100, y: 23400}, {vnum: 9009, x: 73200, y: 39000}, {vnum: 9009, x: 77200, y: 47200},
		{vnum: 9009, x: 76300, y: 80900}, {vnum: 9009, x: 77300, y: 90500}, {vnum: 9009, x: 42800, y: 48000},
		{vnum: 9009, x: 38600, y: 84900}, {vnum: 9009, x: 81900, y: 31500},
		{vnum: 101, x: 56500, y: 67000, spread: 2000, count: 6}, {vnum: 101, x: 61000, y: 77500, spread: 2000, count: 6},
		{vnum: 102, x: 22000, y: 70000, spread: 2000, count: 6}, {vnum: 103, x: 40000, y: 94000, spread: 2000, count: 6},
		{vnum: 108, x: 20500, y: 88000, spread: 2000, count: 6},
		{vnum: 104, x: 73000, y: 67000, spread: 2000, count: 6},
		{vnum: 105, x: 64000, y: 43000, spread: 2000, count: 3}, {vnum: 109, x: 64000, y: 43000, spread: 2000, count: 3},
		{vnum: 110, x: 35500, y: 31000, spread: 2000, count: 3}, {vnum: 111, x: 35500, y: 31000, spread: 2000, count: 3},
		{vnum: 114, x: 67000, y: 116500, spread: 2000, count: 6},
		{vnum: 8001, spots: []Spot{{37000, 106000}, {13000, 58000}, {60000, 44000}}},
		{vnum: 8001, spots: []Spot{{43000, 39000}, {63000, 91000}, {57000, 103000}}},
		{vnum: 8001, spots: []Spot{{73000, 63000}, {16000, 87000}, {69000, 52000}}},
		{vnum: 8002, spots: []Spot{{20000, 35000}, {71000, 102000}, {7000, 96000}}},
		{vnum: 8002, spots: []Spot{{14000, 106000}, {32000, 32000}, {64000, 112000}}},
		{vnum: 8003, spots: []Spot{{48000, 19000}, {7000, 32000}, {19000, 24000}}},
		{vnum: 8003, spots: []Spot{{68000, 31000}, {34000, 19000}, {41000, 26000}}},
	},
}

// Spawns returns every place the world puts mobs, in world coordinates.
func Spawns() []Spawn {
	var all []Spawn
	for _, m := range maps {
		for _, s := range spawnsByMap[m.Name] {
			count := max(s.count, 1)
			var spots []Spot
			for _, p := range s.spots {
				spots = append(spots, Spot{m.X + p.X, m.Y + p.Y})
			}
			all = append(all, Spawn{VNum: s.vnum, X: m.X + s.x, Y: m.Y + s.y, Spread: s.spread, Count: count, Spots: spots})
		}
	}
	return all
}
