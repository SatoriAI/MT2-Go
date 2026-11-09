// Package world is what the server knows about the game world itself. So far: which maps
// exist and where each lies in world coordinates.
package world

// cell is the side of one map cell in world coordinates; atlasinfo.txt measures a map's
// size in cells.
const cell = 25600

// Map is one map's area in world coordinates: x from X up to X+Width, likewise y. Mobs
// multiplies how many monsters each of its packs holds: 1 as they were placed, 2 twice as
// many, 0.5 half, never fewer than one; its NPCs, stones and pack leaders stay one each.
type Map struct {
	Name          string
	X, Y          int32
	Width, Height int32
	Mobs          float64
}

// maps are the client's atlasinfo.txt entries for the maps a character can be on. With no
// warps yet, those are each kingdom's first village.
var maps = []Map{
	{Name: "metin2_map_a1", X: 409600, Y: 896000, Width: 4 * cell, Height: 5 * cell, Mobs: 1},
	{Name: "metin2_map_b1", X: 0, Y: 102400, Width: 4 * cell, Height: 5 * cell, Mobs: 1},
	{Name: "metin2_map_c1", X: 921600, Y: 204800, Width: 4 * cell, Height: 5 * cell, Mobs: 1},
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
