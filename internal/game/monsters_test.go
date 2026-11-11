package game

import (
	"math"
	"net"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/handshake"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/world"
)

// home is where the test monsters stand, in Jinno's first village.
const homeX, homeY = 963400, 278200

// playerAt puts a player x east of the monsters' home and returns their client, with what
// they were shown on entering read.
func (svc *Service) playerAt(t *testing.T, x int32, shown int) net.Conn {
	t.Helper()
	conn, client := watcher(t)
	svc.record.enter(&present{out: conn, add: protocol.CharacterAdd{VID: 1000, X: x, Y: homeY}})
	read(t, client, shown)
	return client
}

const addSize, moveSize = 35, 24

func TestAMonsterStandsUntilItsStrollIsDueThenWalksNearHome(t *testing.T) {
	s := worldWith(t, world.Spawn{VNum: 101, X: homeX, Y: homeY, Count: 1})
	client := s.playerAt(t, homeX, addSize)

	s.tick(noon.Add(wanderMin - time.Millisecond))
	expectNothing(t, client)
	s.tick(noon.Add(wanderMax))

	m := protocol.NewReader(read(t, client, moveSize)[1:])
	kind, _, rotation, _, x, y := m.U8(), m.U8(), m.U8(), m.U32(), m.I32(), m.I32()
	if kind != protocol.MoveWait || x < homeX-wanderRange || x > homeX+wanderRange ||
		y < homeY-wanderRange || y > homeY+wanderRange {
		t.Fatalf("moved with type %d to (%d, %d), want a stroll within %d of home", kind, x, y, wanderRange)
	}
	if want := rotationTowards(x-homeX, y-homeY); rotation != want {
		t.Errorf("faced %d, want %d, towards where it walks", rotation, want)
	}
}

// Once it has strolled, the next stroll waits again.
func TestAMonsterRestsBetweenStrolls(t *testing.T) {
	s := worldWith(t, world.Spawn{VNum: 101, X: homeX, Y: homeY, Count: 1})
	client := s.playerAt(t, homeX, addSize)
	s.tick(noon.Add(wanderMax))
	read(t, client, moveSize)

	s.tick(noon.Add(wanderMax + wanderMin - time.Millisecond))

	expectNothing(t, client)
}

// A monster out of everyone's view still strolls, telling nobody.
func TestAMonsterOutOfViewStrollsUnseen(t *testing.T) {
	s := worldWith(t, world.Spawn{VNum: 101, X: homeX, Y: homeY, Count: 1})
	client := s.playerAt(t, homeX+20000, 0)

	s.tick(noon.Add(wanderMax))

	expectNothing(t, client)
	if w := s.monsters[0]; w.x == homeX && w.y == homeY {
		t.Error("it stayed home")
	}
}

// NPCs keep their place.
func TestAnNPCNeverStrolls(t *testing.T) {
	s := worldWith(t, world.Spawn{VNum: 9001, X: homeX, Y: homeY, Count: 1})
	client := s.playerAt(t, homeX, addSize+54)

	s.tick(noon.Add(time.Hour))

	expectNothing(t, client)
}

// An NPC stands facing where its spawn says, as ADD tells the client.
func TestAnNPCFacesWhereItsSpawnSays(t *testing.T) {
	s := worldWith(t, world.Spawn{VNum: 9001, X: homeX, Y: homeY, Count: 1, Facing: 270})

	for _, p := range s.record.state.all {
		if p.add.Race == 9001 {
			if p.add.Rotation != 270 {
				t.Fatalf("faces %v", p.add.Rotation)
			}
			return
		}
	}
	t.Fatal("no NPC in the world")
}

// A monster that walks into a player's view is shown before its move, as a player is.
func TestAMonsterWalkingIntoViewIsShownBeforeItsMove(t *testing.T) {
	s := worldWith(t, world.Spawn{VNum: 101, X: homeX + 9000, Y: homeY, Count: 1})
	client := s.playerAt(t, homeX, 0)
	vid := s.monsters[0].p.add.VID

	s.record.move(vid, protocol.Move{Type: protocol.MoveWait, X: homeX + 7000, Y: homeY})

	if got := read(t, client, addSize+moveSize); got[0] != byte(protocol.ServerCharacterAdd) ||
		got[addSize] != byte(protocol.ServerMove) {
		t.Fatalf("got %x..., want the add then the move", got[:2])
	}
}

// Each spawn's mobs stand within its area, as many as it asks for.
func TestASpawnPutsItsCountWithinItsSpread(t *testing.T) {
	s := worldWith(t, world.Spawn{VNum: 101, X: homeX, Y: homeY, Spread: 2500, Count: 6})

	if len(s.monsters) != 6 {
		t.Fatalf("%d monsters, want 6", len(s.monsters))
	}
	for _, w := range s.monsters {
		if w.x < homeX-2500 || w.x > homeX+2500 || w.y < homeY-2500 || w.y > homeY+2500 {
			t.Errorf("one stands at (%d, %d), outside its area", w.x, w.y)
		}
	}
}

func TestASpawnOfAnUnknownKindIsRefused(t *testing.T) {
	s := NewService(Deps{Clock: handshake.NewClock(), Spawns: []world.Spawn{{VNum: 1, Count: 1}}})

	if err := s.spawn(noon); err == nil {
		t.Fatal("spawned a kind with no proto")
	}
}

// The client's facing: degrees clockwise from the world's +y, in steps of 5.
func TestFacingIsTheClientsAngleInStepsOfFive(t *testing.T) {
	cases := []struct {
		dx, dy int32
		want   uint8
	}{
		{0, 100, 0}, {100, 0, 18}, {0, -100, 36}, {-100, 0, 54}, {100, 100, 9}, {0, 0, 0},
	}
	for _, c := range cases {
		if got := rotationTowards(c.dx, c.dy); got != c.want {
			t.Errorf("(%d, %d): got %d, want %d", c.dx, c.dy, got, c.want)
		}
	}
}

// Mobs see no mobs: a crowd in a field costs nothing between its members.
func TestMobsDoNotSeeEachOther(t *testing.T) {
	s := worldWith(t, world.Spawn{VNum: 101, X: homeX, Y: homeY, Count: 3}, world.Spawn{VNum: 9001, X: homeX, Y: homeY, Count: 1})

	for vid, p := range s.record.state.all {
		if len(p.sees) != 0 {
			t.Errorf("mob %d sees %d others", vid, len(p.sees))
		}
	}
}

// Someone meeting a monster after its stroll sees it facing the way it went.
func TestAMonsterMetLaterFacesTheWayItLastWalked(t *testing.T) {
	s := worldWith(t, world.Spawn{VNum: 101, X: homeX, Y: homeY, Count: 1})
	s.tick(noon.Add(wanderMax))
	w := s.monsters[0]

	client := s.playerAt(t, w.x, 0)

	add := protocol.NewReader(read(t, client, addSize)[5:]) // after the header and vid
	want := float32(rotationTowards(w.x-homeX, w.y-homeY)) * 5
	if got := math.Float32frombits(add.U32()); got != want {
		t.Fatalf("shown facing %v degrees, want %v", got, want)
	}
}

// However far a monster strolls, it stays on its spawn's map: the spawn's spread and a
// stroll together lie on the map the spawn is on, and so do a stone's waves, called out
// around each of its spots, and their strolls.
func TestNoStrollLeavesTheMapOfItsSpawn(t *testing.T) {
	for _, sp := range world.Spawns() {
		centres, r := []world.Spot{{X: sp.X, Y: sp.Y}}, sp.Spread+wanderRange
		if len(sp.Spots) > 0 {
			centres, r = sp.Spots, waveFrom+waveDepth+wanderRange
		}
		for _, at := range centres {
			home, _ := world.MapAt(at.X, at.Y)
			for _, c := range [][2]int32{{-r, -r}, {r, r}, {-r, r}, {r, -r}} {
				if m, ok := world.MapAt(at.X+c[0], at.Y+c[1]); !ok || m.Name != home.Name {
					t.Errorf("%d at (%d, %d) can stroll to (%d, %d), off %s", sp.VNum, at.X, at.Y, at.X+c[0], at.Y+c[1], home.Name)
				}
			}
		}
	}
}
