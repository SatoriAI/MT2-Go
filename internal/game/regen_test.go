package game

import (
	"net"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// hurtSura is a level-1 sura, 950 hit points and 850 mana at most, in a world of her own,
// with hp and sp; the loop has ticked once at noon, so her first healing is due at
// noon and three seconds.
func hurtSura(t *testing.T, hp, sp uint32) (*Service, net.Conn, *account.Character) {
	t.Helper()
	s := worldWith(t)
	client := s.playerAt(t, homeX, 0)
	c := &account.Character{HP: hp, SP: sp, Level: 1, Class: account.Sura,
		Stats: account.Stats{Strength: 5, Vitality: 3, Dexterity: 3, Intelligence: 5}}
	s.presence.all[1000].char = c
	s.tick(noon)
	return s, client, c
}

func expectPoint(t *testing.T, client net.Conn, point protocol.Point, amount, value int32) {
	t.Helper()
	if p, a, v := pointChange(t, client, 1000); p != point || a != amount || v != value {
		t.Fatalf("got point %d %+d to %d, want %d %+d to %d", p, a, v, point, amount, value)
	}
}

// Every three seconds a living player regains 15 hit points and 5% of their maximum, 47 of
// 950, and 5% of their mana, 42 of 850, and is told.
func TestAPlayerRegainsHitPointsAndManaEveryThreeSeconds(t *testing.T) {
	s, client, c := hurtSura(t, 500, 100)

	s.tick(noon.Add(playerRegenEvery - time.Millisecond))
	expectNothing(t, client)
	s.tick(noon.Add(playerRegenEvery))

	expectPoint(t, client, protocol.PointHP, 62, 562)
	expectPoint(t, client, protocol.PointSP, 42, 142)
	if c.HP != 562 || c.SP != 142 {
		t.Fatalf("has %d and %d", c.HP, c.SP)
	}
}

// Healing stops at the maximum, and a player who is whole is told nothing.
func TestHealingStopsAtTheMaximum(t *testing.T) {
	s, client, c := hurtSura(t, 940, 850)

	s.tick(noon.Add(playerRegenEvery))
	expectPoint(t, client, protocol.PointHP, 10, 950)
	s.tick(noon.Add(2 * playerRegenEvery))

	expectNothing(t, client)
	if c.HP != 950 {
		t.Fatalf("has %d", c.HP)
	}
}

// The dead do not heal: only a restart brings them back.
func TestTheDeadDoNotHeal(t *testing.T) {
	s, client, c := hurtSura(t, 0, 0)

	s.tick(noon.Add(playerRegenEvery))

	expectNothing(t, client)
	if c.HP != 0 {
		t.Fatalf("a dead player has %d hit points", c.HP)
	}
}

// A hurt monster heals its 7% every six seconds, a wild dog's 8 of 126, and whoever picked
// it sees its bar fill; once whole it leaves the list of the hurt.
func TestAHurtMonsterHealsUntilWhole(t *testing.T) {
	s, dog, client := aDog(t, homeX+100)
	s.presence.target(1000, dog.p.add.VID)
	read(t, client, targetSize)
	dog.p.fight.hp = 115
	s.hurt(dog, noon)

	s.healMonsters(noon.Add(6*time.Second - time.Millisecond))
	expectNothing(t, client)
	for i, want := range []uint8{98, 100} { // 123 of 126, then whole, not over
		s.healMonsters(noon.Add(time.Duration(i+1) * 6 * time.Second))
		if got := read(t, client, targetSize); string(got) != string(protocol.EncodeTarget(dog.p.add.VID, want)) {
			t.Fatalf("heal %d: got %x, want its bar at %d%%", i, got, want)
		}
	}

	if dog.p.fight.hp != 126 || len(s.healing) != 0 || dog.healing {
		t.Fatalf("at %d hit points, %d hurt on the list", dog.p.fight.hp, len(s.healing))
	}
}

// A monster that died, or is whole, is not healed and leaves the list, telling nobody.
func TestADeadOrWholeMonsterIsNotHealed(t *testing.T) {
	s, dog, client := aDog(t, homeX+100)
	s.presence.target(1000, dog.p.add.VID)
	read(t, client, targetSize)
	s.hurt(dog, noon)

	s.healMonsters(noon.Add(6 * time.Second))

	expectNothing(t, client)
	if len(s.healing) != 0 || dog.healing {
		t.Fatal("a whole monster stayed on the list")
	}
	dog.p.fight.hp, dog.p.fight.diedAt = 10, noon
	s.hurt(dog, noon)
	s.healMonsters(noon.Add(6 * time.Second))
	if dog.p.fight.hp != 10 || len(s.healing) != 0 {
		t.Fatalf("a dead monster healed to %d", dog.p.fight.hp)
	}
}

// A blow puts the monster on the list of the hurt, once however many blows land.
func TestABlowPutsAMonsterOnTheListOfTheHurtOnce(t *testing.T) {
	s, dog, _, _ := suraHits(t, 100)
	s.presence.attack(1000, dog.p.add.VID, 0, someRand, noon.Add(time.Second))

	s.tick(noon.Add(time.Second))

	if len(s.healing) != 1 || !dog.healing {
		t.Fatalf("%d on the list of the hurt, want the dog once", len(s.healing))
	}
}

// A monster that died with healing to come, and came back, is healed only from its own
// first blow: the old entry drops, and the newcomer keeps its own.
func TestAMonsterThatCameBackHealsOnlyFromItsOwnBlow(t *testing.T) {
	s, dog, _ := aDog(t, homeX+100)
	dog.p.fight.hp = 100
	s.hurt(dog, noon)
	dog.p.fight.diedAt, dog.diedAt, dog.gone = noon, noon, true
	s.place(dog, noon.Add(respawnAfter))
	dog.p.fight.hp = 100
	s.hurt(dog, noon.Add(respawnAfter))

	s.healMonsters(noon.Add(respawnAfter)) // the old entry's time has long come

	if dog.p.fight.hp != 100 || len(s.healing) != 1 || !dog.healing {
		t.Fatalf("at %d hit points with %d on the list, healing %v; want 100, its own entry, healing", dog.p.fight.hp,
			len(s.healing), dog.healing)
	}
}

// A player in a fight does not heal: only five seconds after their last blow or bite.
func TestAPlayerHealsOnlyOutOfAFight(t *testing.T) {
	s, client, c := hurtSura(t, 500, 100)
	c.LastFought = noon.Add(time.Second)

	s.tick(noon.Add(playerRegenEvery))
	expectNothing(t, client)
	s.tick(noon.Add(2 * playerRegenEvery))

	expectPoint(t, client, protocol.PointHP, 62, 562)
}

// Her blow and the dog's bite both count as fighting.
func TestABlowAndABiteMarkTheFight(t *testing.T) {
	s, dog, client, c := suraHits(t, 100)
	if !c.LastFought.Equal(noon) {
		t.Fatalf("her blow at noon left her last fight at %v", c.LastFought)
	}
	c.LastFought = time.Time{}

	s.tick(noon.Add(time.Second))
	expectBite(t, client, dog, c)

	if !c.LastFought.Equal(noon.Add(time.Second)) {
		t.Fatalf("the bite left her last fight at %v", c.LastFought)
	}
}

// A restart ends the fight that ended in death, so healing goes on as usual; a blow that
// lands nowhere starts none.
func TestARestartEndsTheFightAndARefusedBlowStartsNone(t *testing.T) {
	s, dog, _, c := suraHits(t, 100)
	c.HP = 0

	s.presence.revive(1000, false, 0, 0)

	if !c.LastFought.IsZero() {
		t.Fatalf("still fighting since %v after a restart", c.LastFought)
	}
	s.presence.move(1000, protocol.Move{X: homeX + 2000, Y: homeY})
	if got := s.presence.attack(1000, dog.p.add.VID, 0, someRand, noon.Add(time.Second)); got != blowRefused {
		t.Fatalf("a blow from 2000 away came to %v", got)
	}
	if !c.LastFought.IsZero() {
		t.Fatalf("a refused blow started a fight at %v", c.LastFought)
	}
}
