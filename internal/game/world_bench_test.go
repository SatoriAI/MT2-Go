package game

import (
	"fmt"
	"math/rand/v2"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/handshake"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/world"
)

// These measure the world record and loop at today's size and at ten and a hundred
// times it; docs/decisions/world-loop.md reads the results as a budget. Run them with
// `make bench`.

// sink stands in for a player's connection, counting what it is sent: thousands of
// sockets would measure the network, not the world.
type sink struct{ sends atomic.Int64 }

func (s *sink) Send(...[]byte) error {
	s.sends.Add(1)
	return nil
}

var worldSizes = []struct{ players, monsters int }{{100, 160}, {1000, 1600}, {10000, 16000}}

// The three first maps, in world coordinates, which the players and monsters share.
var firstMaps = []struct{ x, y int32 }{{409600, 896000}, {0, 102400}, {921600, 204800}}

const mapWidth, mapHeight = 102400, 128000

// benchWorld is a world of players and wild dogs at random points across the three
// first maps, the same each run.
func benchWorld(b *testing.B, players, monsters int) (*Service, []*present, *sink) {
	b.Helper()
	r := rand.New(rand.NewPCG(7, 7)) //nolint:gosec // a fixed seed, so each run measures the same world
	point := func() (int32, int32) {
		m := firstMaps[r.IntN(len(firstMaps))]
		return m.x + 1000 + r.Int32N(mapWidth-2000), m.y + 1000 + r.Int32N(mapHeight-2000)
	}
	var spawns []world.Spawn
	for range monsters {
		x, y := point()
		spawns = append(spawns, world.Spawn{VNum: 101, X: x, Y: y, Count: 1})
	}
	s := NewService(nil, nil, nil, nil, nil, nil, handshake.NewClock(), netip.AddrPort{}, spawns)
	s.rand = rand.New(rand.NewPCG(1, 2)) //nolint:gosec // a fixed seed, as above
	if err := s.spawn(noon); err != nil {
		b.Fatal(err)
	}
	out := &sink{}
	var everyone []*present
	for i := range players {
		x, y := point()
		p := &present{out: out, add: protocol.CharacterAdd{VID: uint32(100000 + i), X: x, Y: y, MoveSpeed: 150},
			tether: tether{x: x, y: y}}
		s.presence.enter(p)
		everyone = append(everyone, p)
	}
	return s, everyone, out
}

func size(players, monsters int) string {
	return fmt.Sprintf("players=%d/monsters=%d", players, monsters)
}

// A player steps 300 one way and back, so the world stays the same size and spread, each
// step a second after the last, as a walk takes it.
func BenchmarkPlayerMove(b *testing.B) {
	for _, n := range worldSizes {
		b.Run(size(n.players, n.monsters), func(b *testing.B) {
			s, players, out := benchWorld(b, n.players, n.monsters)
			out.sends.Store(0) // only what the moves send
			b.ReportAllocs()
			i := 0
			for b.Loop() {
				p := players[i%len(players)]
				step := int32(300 - 600*(i/len(players)%2))
				s.presence.walk(p.add.VID, protocol.Move{Type: 1, X: p.add.X + step, Y: p.add.Y}, noon.Add(time.Duration(i)*time.Second))
				i++
			}
			b.ReportMetric(float64(out.sends.Load())/float64(b.N), "sends/op")
		})
	}
}

// A player's move, with drops lying about: one for every ten monsters, a generous guess at
// what a minute of kills leaves on the ground. The move looks at each, to bring the ones
// that come into view and take away those that leave it.
func BenchmarkPlayerMoveAmongDrops(b *testing.B) {
	for _, n := range worldSizes {
		b.Run(size(n.players, n.monsters), func(b *testing.B) {
			s, players, out := benchWorld(b, n.players, n.monsters)
			r := rand.New(rand.NewPCG(9, 9)) //nolint:gosec // a fixed seed, so each run measures the same ground
			for i := range n.monsters / 10 {
				m := firstMaps[r.IntN(len(firstMaps))]
				x, y := m.x+1000+r.Int32N(mapWidth-2000), m.y+1000+r.Int32N(mapHeight-2000)
				on, _ := world.MapAt(x, y)
				vid := uint32(900000 + i)
				s.presence.ground[vid] = &groundItem{vid: vid, world: on.Name, x: x, y: y, vnum: 27001, count: 1}
			}
			out.sends.Store(0)
			b.ReportAllocs()
			i := 0
			for b.Loop() {
				p := players[i%len(players)]
				step := int32(300 - 600*(i/len(players)%2))
				s.presence.walk(p.add.VID, protocol.Move{Type: 1, X: p.add.X + step, Y: p.add.Y}, noon.Add(time.Duration(i)*time.Second))
				i++
			}
			b.ReportMetric(float64(out.sends.Load())/float64(b.N), "sends/op")
		})
	}
}

// One monster strolls: the loop finds it due and walks it, the cost of each of the
// world's strolls.
func BenchmarkMonsterStroll(b *testing.B) {
	for _, n := range worldSizes {
		b.Run(size(n.players, n.monsters), func(b *testing.B) {
			s, _, _ := benchWorld(b, n.players, n.monsters)
			b.ReportAllocs()
			i := 0
			for b.Loop() {
				w := s.monsters[i%len(s.monsters)]
				w.nextStroll = time.Time{}
				s.tick(noon) // walks w alone: every other monster is due later
				w.nextStroll = noon.Add(time.Hour)
				i++
			}
		})
	}
}

// A tick that finds nothing due, as most of the ten each second do.
func BenchmarkTickIdle(b *testing.B) {
	for _, n := range worldSizes {
		b.Run(size(n.players, n.monsters), func(b *testing.B) {
			s, _, _ := benchWorld(b, n.players, n.monsters)
			for _, w := range s.monsters {
				w.nextStroll = noon.Add(time.Hour)
			}
			b.ReportAllocs()
			for b.Loop() {
				s.tick(noon)
			}
		})
	}
}

// A player enters a populated world and leaves again, as logging in and out does.
func BenchmarkPlayerEnter(b *testing.B) {
	for _, n := range worldSizes {
		b.Run(size(n.players, n.monsters), func(b *testing.B) {
			s, players, out := benchWorld(b, n.players, n.monsters)
			at := players[0].add
			b.ReportAllocs()
			for b.Loop() {
				p := &present{out: out, info: protocol.CharacterInfo{Name: "Newcomer"},
					add: protocol.CharacterAdd{VID: 1 << 30, X: at.X, Y: at.Y}}
				s.presence.enter(p)
				s.presence.leave(p.add.VID)
			}
		})
	}
}

// BenchmarkPlayerRegeneration is one round of healing the players, every three seconds,
// which holds the world lock over all of them: each a level-1 sura short of her hit points
// by more than a round restores, so every round sends to each.
func BenchmarkPlayerRegeneration(b *testing.B) {
	for _, n := range worldSizes {
		b.Run(size(n.players, n.monsters), func(b *testing.B) {
			s, everyone, _ := benchWorld(b, n.players, n.monsters)
			for _, p := range everyone {
				p.char = &account.Character{Level: 1, Class: account.Sura, Stats: account.Stats{Vitality: 3, Intelligence: 5}}
				p.info.Name = fmt.Sprint(p.add.VID)
				s.presence.byName[p.info.Name] = p
			}
			b.ReportAllocs()
			for b.Loop() {
				for _, p := range everyone {
					p.char.HP, p.char.SP = 1, 0
				}
				s.presence.regenPlayers(noon)
			}
		})
	}
}
