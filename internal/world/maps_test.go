package world

import (
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/account"
)

func TestAPointIsOnAMapFromItsCornerUpToItsFarEdge(t *testing.T) {
	cases := []struct {
		x, y int32
		want string
	}{
		{409600, 896000, "metin2_map_a1"},  // the first corner
		{511999, 1023999, "metin2_map_a1"}, // just inside the far corner
		{512000, 896000, ""},               // just past the east edge
		{409599, 896000, ""},               // just before the west edge
		{0, 102400, "metin2_map_b1"},
		{963400, 278200, "metin2_map_c1"},
		{5, 5, ""},
	}
	for _, c := range cases {
		m, ok := MapAt(c.x, c.y)
		if got := m.Name; got != c.want || ok != (c.want != "") {
			t.Errorf("(%d, %d): got %q %v, want %q", c.x, c.y, got, ok, c.want)
		}
	}
}

func TestEveryKingdomStartsOnAMapTheServerKnows(t *testing.T) {
	for _, empire := range []account.Empire{account.EmpireShinsoo, account.EmpireChunjo, account.EmpireJinno} {
		x, y, _ := empire.StartPosition()
		if _, ok := MapAt(x, y); !ok {
			t.Errorf("%s starts at (%d, %d), on no known map", empire, x, y)
		}
	}
}
