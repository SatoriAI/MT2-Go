// Package world is what the server knows about the game world itself. So far: which maps
// exist and where each lies in world coordinates.
package world

// cell is the side of one map cell in world coordinates; atlasinfo.txt measures a map's
// size in cells.
const cell = 25600

// Map is one map's area in world coordinates: x from X up to X+Width, likewise y. Mobs
// multiplies how many monsters each of its packs holds: 1 as they were placed, 2 twice as
// many, 0.5 half, never fewer than one; its NPCs, stones and pack leaders stay one each.
// Arrival, in world coordinates, is where the ring brings a player to a map no kingdom
// starts on and where one dying there rises in town; zero on a kingdom's own map.
type Map struct {
	Name          string
	X, Y          int32
	Width, Height int32
	Mobs          float64
	Arrival       Spot
}

// Valley is Seungryong Valley's name: the one map all three kingdoms share.
const Valley = "map_a2"

// maps are the client's atlasinfo.txt entries for the maps a character can be on: each
// kingdom's first village, and the valley, whose arrival is open ground on its southern
// plateau (docs/mechanics/maps.md).
var maps = []Map{
	{Name: "metin2_map_a1", X: 409600, Y: 896000, Width: 4 * cell, Height: 5 * cell, Mobs: 1},
	{Name: "metin2_map_b1", X: 0, Y: 102400, Width: 4 * cell, Height: 5 * cell, Mobs: 1},
	{Name: "metin2_map_c1", X: 921600, Y: 204800, Width: 4 * cell, Height: 5 * cell, Mobs: 1},
	{Name: Valley, X: 256000, Y: 665600, Width: 6 * cell, Height: 6 * cell, Mobs: 1, Arrival: Spot{256000 + 82000, 665600 + 137500}},
}

// MapNamed returns the map of the name, or false when the server knows none.
func MapNamed(name string) (Map, bool) {
	for _, m := range maps {
		if m.Name == name {
			return m, true
		}
	}
	return Map{}, false
}

// MapAt returns the map the point lies on, or false when it lies on none the server knows.
func MapAt(x, y int32) (Map, bool) {
	for _, m := range maps {
		if x >= m.X && x < m.X+m.Width && y >= m.Y && y < m.Y+m.Height {
			return m, true
		}
	}
	return Map{}, false
}
