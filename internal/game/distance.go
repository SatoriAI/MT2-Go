package game

// Every reach is compared squared, in whole units, so no root is taken on the hot paths.

// apart is how far apart two points stand, squared.
func apart(x, y, ox, oy int32) int64 {
	dx, dy := int64(x)-int64(ox), int64(y)-int64(oy)
	return dx*dx + dy*dy
}

// distance is how far apart two characters stand, squared.
func distance(p, o *present) int64 {
	return apart(p.add.X, p.add.Y, o.add.X, o.add.Y)
}

// within is whether two characters stand at most reach apart.
func within(p, o *present, reach int64) bool {
	return distance(p, o) <= reach*reach
}

// near is whether (x, y) is less than reach from (fx, fy).
func near(x, y, fx, fy int32, reach int64) bool {
	return apart(x, y, fx, fy) < reach*reach
}

// nearest is the one of among nearest to from that ok takes, or nil; of two as near,
// whichever the map gives first.
func nearest(from *present, among map[uint32]*present, ok func(*present) bool) *present {
	var found *present
	for _, o := range among {
		if ok(o) && (found == nil || distance(from, o) < distance(from, found)) {
			found = o
		}
	}
	return found
}
