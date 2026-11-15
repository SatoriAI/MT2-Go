package game

import (
	"context"
	"net"
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/network"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/world"
)

// The world coordinates of Shinsoo's gates: to the valley, and closed.
const (
	shinsooGateX, shinsooGateY     = 409600 + 40500, 896000 + 7300
	shinsooClosedX, shinsooClosedY = 409600 + 7600, 896000 + 60100
)

// standAt puts the player at (x, y), as a move there would, and takes her through any gate.
func standAt(t *testing.T, s *session, x, y int32) {
	t.Helper()
	s.svc.record.held(func(l locked) { l.all[1000].add.X, l.all[1000].add.Y = x, y })
	s.playing.X, s.playing.Y = x, y
	if err := s.passGate(); err != nil {
		t.Fatal(err)
	}
}

// From level 20, walking into her village's gate to the valley takes her to its arrival.
func TestTheGateToTheValleyTakesHerThereFromItsLevel(t *testing.T) {
	s, store, client, _ := ringing(t, 20)
	drain(t, client)

	standAt(t, s, shinsooGateX, shinsooGateY)

	valley, _ := world.MapNamed(world.Valley)
	if got := packets(t, drain(t, client)); !holds(got, protocol.EncodeWarp(valley.Arrival.X, valley.Arrival.Y, gameServer)) {
		t.Fatalf("told %x", got)
	}
	if _, at, _ := store.ofHendra(); at != [2]int32{valley.Arrival.X, valley.Arrival.Y} {
		t.Fatalf("saved at %v", at)
	}
}

// Below level 20 the gate says why once, while she stands in it, and again when she comes
// back; she stays.
func TestTheGateRefusesHerBelowTheValleysLevelOncePerApproach(t *testing.T) {
	s, _, client, _ := ringing(t, 19)
	drain(t, client)

	standAt(t, s, shinsooGateX, shinsooGateY)
	expectChatInfo(t, client, valleyLow)
	standAt(t, s, shinsooGateX+100, shinsooGateY)
	expectNothing(t, client)
	standAt(t, s, shinsooGateX, shinsooGateY+5000)
	standAt(t, s, shinsooGateX, shinsooGateY)

	expectChatInfo(t, client, valleyLow)
	if s.phase == protocol.PhaseSelect {
		t.Fatal("she was taken away")
	}
}

// A closed gate says so and takes nobody.
func TestAClosedGateSaysSo(t *testing.T) {
	s, _, client, _ := ringing(t, 30)
	drain(t, client)

	standAt(t, s, shinsooClosedX, shinsooClosedY)

	expectChatInfo(t, client, gateClosed)
	if s.phase == protocol.PhaseSelect {
		t.Fatal("she was taken away")
	}
}

// The valley's southern gate takes her home, a step inside her kingdom's gate to it.
func TestTheValleysGateTakesHerHome(t *testing.T) {
	s, _, client, _ := ringing(t, 30)
	drain(t, client)
	valley, _ := world.MapNamed(world.Valley)

	standAt(t, s, valley.X+64000, valley.Y+143600)

	if got := packets(t, drain(t, client)); !holds(got, protocol.EncodeWarp(409600+40500, 896000+9300, gameServer)) {
		t.Fatalf("told %x", got)
	}
}

// Trading, she is told the gate cannot be passed now.
func TestAGateIsNotPassedWhileTrading(t *testing.T) {
	s, _, client, _ := ringing(t, 30)
	drain(t, client)
	s.svc.record.held(func(l locked) { l.trades[1000] = &trade{} })

	standAt(t, s, shinsooGateX, shinsooGateY)

	expectChatInfo(t, client, gateBusy)
}

// atTheTeleporter is Hendra of Shinsoo at level, with gold yang, beside the teleporter,
// whose vid it returns, claimed so a warp can let her go.
func atTheTeleporter(t *testing.T, level uint8, gold uint32) (*session, *keptItems, net.Conn, uint32) {
	t.Helper()
	s := worldWith(t, world.Spawn{VNum: 9012, X: homeX, Y: homeY, Count: 1})
	kept := &keptItems{gold: gold, count: map[int64]uint8{}}
	s.items, s.accounts, s.public = kept, newSaves(), gameServer
	client, hendra := aPlayer(t, s, 1000, 7, "Hendra", 100)
	hendra.char.Level = level
	read(t, client, addSize+infoSize) // the teleporter
	sess := &session{svc: s, conn: hendra.out.(*network.Conn), phase: protocol.PhaseGame,
		playing: playingAs(hendra), gold: gold, empire: account.EmpireShinsoo}
	sess.playing.ID = 7
	claim, _, err := s.claimCharacter(context.Background(), 7, func() {})
	if err != nil {
		t.Fatal(err)
	}
	sess.claim = claim
	return sess, kept, client, s.lastVID.Load()
}

// The teleporter offers the valley, saying what lives there and its price; yes, from its
// level and with the yang, takes the yang and her there.
func TestTheTeleporterTakesHerToTheValleyForItsPrice(t *testing.T) {
	s, kept, client, teleporter := atTheTeleporter(t, 20, 2000)

	if err := s.onClick(vidBody(teleporter)); err != nil {
		t.Fatal(err)
	}
	expectSaid(t, client, teleporterOffer)
	if err := s.onScriptAnswer(context.Background(), answered(0)); err != nil {
		t.Fatal(err)
	}

	valley, _ := world.MapNamed(world.Valley)
	if got := packets(t, drain(t, client)); !holds(got, protocol.EncodeWarp(valley.Arrival.X, valley.Arrival.Y, gameServer)) || kept.gold != 500 || s.gold != 500 {
		t.Fatalf("told %x, yang %d", got, kept.gold)
	}
}

// Below the valley's level, without the yang, or trading, the teleporter says why and
// takes nothing; no takes nothing either.
func TestTheTeleporterSaysWhyNot(t *testing.T) {
	for name, c := range map[string]struct {
		level  uint8
		gold   uint32
		answer uint8
		said   string
	}{
		"below level": {19, 2000, 0, valleyLow},
		"no yang":     {20, 1499, 0, teleporterBroke},
		"no":          {20, 2000, 1, ""},
		"trading":     {20, 2000, 0, teleporterBusy},
	} {
		s, kept, client, teleporter := atTheTeleporter(t, c.level, c.gold)
		if name == "trading" {
			s.svc.record.held(func(l locked) { l.trades[1000] = &trade{} })
		}
		if err := s.onClick(vidBody(teleporter)); err != nil {
			t.Fatal(err)
		}
		drain(t, client)

		if err := s.onScriptAnswer(context.Background(), answered(c.answer)); err != nil {
			t.Fatal(err)
		}

		got := packets(t, drain(t, client))
		if kept.gold != c.gold || s.phase == protocol.PhaseSelect || c.said == "" && len(got) != 0 || c.said != "" && !holds(got, script(c.said+"[ENTER][DONE]")) {
			t.Errorf("%s: told %x, yang %d", name, got, kept.gold)
		}
	}
}

// A yes after she walked away from the teleporter takes nothing.
func TestTheTeleporterTakesNobodyWhoWalkedAway(t *testing.T) {
	s, kept, client, teleporter := atTheTeleporter(t, 20, 2000)
	if err := s.onClick(vidBody(teleporter)); err != nil {
		t.Fatal(err)
	}
	drain(t, client)
	s.svc.record.held(func(l locked) { l.all[1000].add.X += 5000 })

	if err := s.onScriptAnswer(context.Background(), answered(0)); err != nil {
		t.Fatal(err)
	}

	expectNothing(t, client)
	if kept.gold != 2000 {
		t.Fatalf("yang %d", kept.gold)
	}
}
