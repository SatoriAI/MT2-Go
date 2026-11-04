package game

import (
	"net"
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/world"
)

const pointChangeSize = 17

// pointChange reads the next POINT_CHANGE and returns its point, amount and value.
func pointChange(t *testing.T, c net.Conn, vid uint32) (point protocol.Point, amount, value int32) {
	t.Helper()
	r := protocol.NewReader(read(t, c, pointChangeSize)[4:])
	if got := r.U32(); got != vid {
		t.Fatalf("a point change for %d, want %d", got, vid)
	}
	return protocol.Point(r.U8()), r.I32(), r.I32()
}

// killAlone has player 1000, at the dog's side with the character c, kill it in one blow,
// and reads what the kill itself told them.
func killAlone(t *testing.T, c *account.Character) (*Service, net.Conn) {
	t.Helper()
	s, dog, client := aDog(t, homeX+100)
	s.presence.all[1000].char = c
	c.Strength, c.Class = 90, account.Warrior // one blow kills
	s.presence.attack(1000, dog.p.add.VID, 0, someRand, noon)
	read(t, client, damageSize+deadSize)
	return s, client
}

// A player who kills a monster alone gets all its experience: a wild dog's 15, at level 1
// level with it.
func TestKillingAloneGivesAllTheExperience(t *testing.T) {
	c := &account.Character{HP: 1000, SP: 1000, Level: 1}
	_, client := killAlone(t, c)

	if point, amount, value := pointChange(t, client, 1000); point != protocol.PointExp || amount != 15 || value != 15 {
		t.Fatalf("got point %d, +%d to %d; want exp +15 to 15", point, amount, value)
	}
	if c.Exp != 15 {
		t.Fatalf("the character has %d exp", c.Exp)
	}
}

// Far above a monster, a player gets almost nothing of it: a level-30 player's 1% of a
// dog's 15 rounds to nothing, and nothing is sent.
func TestAMonsterFarBelowGivesNothingWorthSending(t *testing.T) {
	c := &account.Character{HP: 1000, SP: 1000, Level: 30}
	_, client := killAlone(t, c)

	expectNothing(t, client)
	if c.Exp != 0 {
		t.Fatalf("got %d exp", c.Exp)
	}
}

// A monster above the player gives more: a boar, level 7, is 6 above a level-1 player,
// 130% of its 87.
func TestAMonsterAboveGivesMore(t *testing.T) {
	s := worldWith(t, world.Spawn{VNum: 108, X: homeX, Y: homeY, Count: 1})
	client := s.playerAt(t, homeX+100, addSize)
	c := &account.Character{HP: 1000, SP: 1000, Level: 1, Class: account.Warrior, Stats: account.Stats{Strength: 90}}
	s.presence.all[1000].char = c
	boar := s.monsters[0].p.add.VID
	s.presence.attack(1000, boar, 0, someRand, noon) // a boar's 248 hit points take two blows
	read(t, client, damageSize)
	s.presence.attack(1000, boar, 0, someRand, noon)
	read(t, client, damageSize+deadSize)

	if _, amount, _ := pointChange(t, client, 1000); amount != 87*130/100 {
		t.Fatalf("got %d, want %d", amount, 87*130/100)
	}
}

// Filling the need raises the level: the player is told their experience, new level,
// next need, stat points and refilled maximums; an onlooker, their new level.
func TestFillingTheNeedRaisesTheLevelForTheirClientAndOnlookers(t *testing.T) {
	s, dog, client := aDog(t, homeX+100)
	c := &account.Character{HP: 1000, SP: 1000, Level: 1, Exp: 290, Class: account.Warrior, Stats: account.Stats{Strength: 90}}
	s.presence.all[1000].char = c
	conn, onlooker := watcher(t)
	s.presence.enter(&present{out: conn, add: protocol.CharacterAdd{VID: 2000, X: homeX + 200, Y: homeY}})
	read(t, onlooker, addSize+addSize) // the dog and the player
	read(t, client, addSize)           // the onlooker

	s.presence.attack(1000, dog.p.add.VID, 0, someRand, noon)
	read(t, client, damageSize+deadSize)

	points, _ := c.Points()
	want := []struct {
		point protocol.Point
		value int32
	}{
		{protocol.PointExp, 5}, {protocol.PointLevel, 2}, {protocol.PointNextExp, 800}, {protocol.PointStatPoints, 3},
		{protocol.PointMaxHP, int32(points.MaxHP)}, {protocol.PointHP, int32(points.MaxHP)}, //nolint:gosec // small
		{protocol.PointMaxSP, int32(points.MaxSP)}, {protocol.PointSP, int32(points.MaxSP)}, //nolint:gosec // small
	}
	for _, w := range want {
		if point, _, value := pointChange(t, client, 1000); point != w.point || value != w.value {
			t.Fatalf("got point %d at %d, want %d at %d", point, value, w.point, w.value)
		}
	}
	read(t, onlooker, deadSize)
	if point, _, value := pointChange(t, onlooker, 1000); point != protocol.PointLevel || value != 2 {
		t.Fatalf("the onlooker got point %d at %d, want the level at 2", point, value)
	}
	if s.presence.all[1000].info.Level != 2 {
		t.Fatal("someone arriving later would see level 1")
	}
}

// Two players share a monster's experience by the share of its hit points each took; what
// rounding leaves goes to the killer.
func TestExperienceIsSharedByTheDamageEachDealt(t *testing.T) {
	s, dog, first := aDog(t, homeX+100)
	conn, second := watcher(t)
	s.presence.enter(&present{out: conn, add: protocol.CharacterAdd{VID: 2000, X: homeX + 150, Y: homeY}})
	read(t, second, addSize+addSize)
	read(t, first, addSize)
	weak := &account.Character{HP: 1000, SP: 1000, Level: 1, Class: account.Warrior, Stats: account.Stats{Strength: 30}}
	strong := &account.Character{HP: 1000, SP: 1000, Level: 1, Class: account.Warrior, Stats: account.Stats{Strength: 90}}
	s.presence.all[1000].char, s.presence.all[2000].char = weak, strong
	vid := dog.p.add.VID

	s.presence.attack(1000, vid, 0, someRand, noon)
	read(t, first, damageSize)
	dealt := 126 - dog.p.fight.hp
	s.presence.attack(2000, vid, 0, someRand, noon)
	read(t, second, damageSize+deadSize)
	read(t, first, deadSize)

	firstShare := int32(15 * dealt / 126) //nolint:gosec // small
	if _, amount, _ := pointChange(t, first, 1000); amount != firstShare {
		t.Fatalf("the first got %d for %d of 126, want %d", amount, dealt, firstShare)
	}
	if _, amount, _ := pointChange(t, second, 2000); amount != 15-firstShare {
		t.Fatalf("the killer got %d, want the other %d", amount, 15-firstShare)
	}
}

// A player who left before the kill gets nothing, and their share is not handed on.
func TestAPlayerWhoLeftGetsNoShare(t *testing.T) {
	s, dog, killer := aDog(t, homeX+100)
	conn, _ := watcher(t)
	s.presence.enter(&present{out: conn, add: protocol.CharacterAdd{VID: 2000, X: homeX + 150, Y: homeY}})
	read(t, killer, addSize)
	gone := &account.Character{HP: 1000, SP: 1000, Level: 1, Class: account.Warrior, Stats: account.Stats{Strength: 30}}
	s.presence.all[2000].char = gone
	s.presence.all[1000].char = &account.Character{HP: 1000, SP: 1000, Level: 1, Class: account.Warrior, Stats: account.Stats{Strength: 90}}
	vid := dog.p.add.VID
	s.presence.attack(2000, vid, 0, someRand, noon)
	dealt := 126 - dog.p.fight.hp
	s.presence.leave(2000)
	read(t, killer, deleteSize)

	s.presence.attack(1000, vid, 0, someRand, noon)
	read(t, killer, damageSize+deadSize)

	if _, amount, _ := pointChange(t, killer, 1000); amount != 15-int32(15*dealt/126) { //nolint:gosec // small
		t.Fatalf("the killer got %d, want their own share and the rounding", amount)
	}
	if gone.Exp != 0 {
		t.Fatalf("the player who left got %d", gone.Exp)
	}
}

// A stat point goes into the stat the button names; the player sees the stat, their
// points left, and for vitality their new maximum hit points.
func TestSpendingAStatPointRaisesTheStat(t *testing.T) {
	s, _, client := aDog(t, homeX+100)
	c := &account.Character{HP: 1000, SP: 1000, Level: 2, StatPoints: 2, Class: account.Warrior, Stats: account.Stats{Strength: 6, Vitality: 4}}
	s.presence.all[1000].char = c

	s.presence.spendStatPoint(1000, "st")
	if point, _, value := pointChange(t, client, 1000); point != protocol.PointST || value != 7 {
		t.Fatalf("got point %d at %d, want strength 7", point, value)
	}
	if point, _, value := pointChange(t, client, 1000); point != protocol.PointStatPoints || value != 1 {
		t.Fatalf("got point %d at %d, want 1 point left", point, value)
	}
	s.presence.spendStatPoint(1000, "ht")
	pointChange(t, client, 1000)
	pointChange(t, client, 1000)
	points, _ := c.Points()
	if point, _, value := pointChange(t, client, 1000); point != protocol.PointMaxHP || value != int32(points.MaxHP) { //nolint:gosec // small
		t.Fatalf("got point %d at %d, want maximum hit points %d", point, value, points.MaxHP)
	}
}

// With no point to spend, or a name the buttons never send, nothing changes.
func TestNoPointOrAnUnknownStatChangesNothing(t *testing.T) {
	s, _, client := aDog(t, homeX+100)
	c := &account.Character{HP: 1000, SP: 1000, Level: 1, Stats: account.Stats{Strength: 6}}
	s.presence.all[1000].char = c

	s.presence.spendStatPoint(1000, "st")
	c.StatPoints = 1
	s.presence.spendStatPoint(1000, "luck")

	expectNothing(t, client)
	if c.Strength != 6 || c.StatPoints != 1 {
		t.Fatalf("strength %d, %d points", c.Strength, c.StatPoints)
	}
}
