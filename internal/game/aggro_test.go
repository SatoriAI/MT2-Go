package game

import (
	"net"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// suraHits puts a level-1 sura, vid 1000, at x east of a wild dog's home, has her hit it
// once at noon, and returns her client with that blow's damage read, and her character.
func suraHits(t *testing.T, x int32) (*Service, *monster, net.Conn, *account.Character) {
	t.Helper()
	s, dog, client := aDog(t, homeX+x)
	c := &account.Character{Level: 1, Class: account.Sura, Stats: account.Stats{Strength: 5, Vitality: 3, Dexterity: 3, Intelligence: 5}}
	points, _ := c.Points()
	c.HP, c.SP = points.MaxHP, points.MaxSP
	s.presence.all[1000].char = c
	if got := s.presence.attack(1000, dog.p.add.VID, 0, someRand, noon); got != blowLanded {
		t.Fatalf("the blow came to %v", got)
	}
	read(t, client, damageSize)
	return s, dog, client, c
}

// expectBite reads a bite of the dog on the sura: its attack, facing her, the damage, and
// her hit points left, which it checks against her character. It returns the damage.
func expectBite(t *testing.T, client net.Conn, dog *monster, c *account.Character) int32 {
	t.Helper()
	m := protocol.NewReader(read(t, client, moveSize)[1:])
	kind, _, rotation, vid := m.U8(), m.U8(), m.U8(), m.U32()
	if kind != protocol.MoveAttack || vid != dog.p.add.VID || rotation != rotationTowards(1, 0) {
		t.Fatalf("got move %d of %d facing %d, want the dog's attack facing east", kind, vid, rotation)
	}
	d := protocol.NewReader(read(t, client, damageSize)[1:])
	if victim, _, damage := d.U32(), d.U8(), d.I32(); victim != 1000 || damage < 1 || damage > 31 {
		t.Fatalf("got %d damage on %d, want up to 31 on the sura", damage, victim)
	} else if point, amount, value := pointChange(t, client, 1000); point != protocol.PointHP || amount != -damage ||
		value != int32(c.HP) { //nolint:gosec // small
		t.Fatalf("got point %d %+d to %d, want her hit points down %d to %d", point, amount, value, damage, c.HP)
	} else {
		return damage
	}
	return 0
}

// A monster that is hit bites back at once when its attacker is in reach, as hard as its
// dice allow, and she loses as many hit points.
func TestAHitMonsterBitesBackInReach(t *testing.T) {
	s, dog, client, c := suraHits(t, 100)
	full := c.HP

	s.tick(noon)

	if damage := expectBite(t, client, dog, c); damage < 27 || c.HP != full-uint32(damage) {
		t.Fatalf("bit for %d leaving %d of %d, want 27 to 31", damage, c.HP, full)
	}
}

// A dog bites every two seconds, its attack speed of 100, and not between.
func TestAMonsterBitesAsOftenAsItsAttackSpeed(t *testing.T) {
	s, dog, client, c := suraHits(t, 100)
	s.tick(noon)
	expectBite(t, client, dog, c)

	s.tick(noon.Add(500 * time.Millisecond))
	s.tick(noon.Add(1500 * time.Millisecond))
	expectNothing(t, client)
	s.tick(noon.Add(2 * time.Second))

	expectBite(t, client, dog, c)
}

// A monster hit from beyond its bite walks towards its attacker, stopping at nine tenths
// of its range, and bites once there.
func TestAMonsterChasesAnAttackerOutOfReach(t *testing.T) {
	s, dog, client, c := suraHits(t, 280)

	s.tick(noon)

	m := protocol.NewReader(read(t, client, moveSize)[1:])
	kind, _, _, _, x, y := m.U8(), m.U8(), m.U8(), m.U32(), m.I32(), m.I32()
	if kind != protocol.MoveWait || x != homeX+123 || y != homeY {
		t.Fatalf("moved with type %d to (%d, %d), want to 157 short of her, at (%d, %d)", kind, x, y, homeX+123, homeY)
	}
	s.tick(noon.Add(500 * time.Millisecond))
	expectBite(t, client, dog, c)
}

// A chasing monster walks no faster than its move speed: 200 each half second at 100.
func TestAChaseStepIsNoLongerThanTheMonsterWalks(t *testing.T) {
	s, dog, client, _ := suraHits(t, 100)
	s.presence.move(1000, protocol.Move{X: homeX + 3000, Y: homeY})

	for i, at := range []time.Duration{0, 500 * time.Millisecond} {
		s.tick(noon.Add(at))
		m := protocol.NewReader(read(t, client, moveSize)[1:])
		m.U8()
		m.U8()
		m.U8()
		if vid, x := m.U32(), m.I32(); vid != dog.p.add.VID || x != homeX+200*int32(i+1) {
			t.Fatalf("step %d: %d moved to %d, want %d", i, vid, x, homeX+200*(i+1))
		}
	}
}

// A monster that has not bitten for ten seconds gives up and walks home, and strolls
// again from there.
func TestAMonsterThatCannotBiteForTenSecondsGivesUp(t *testing.T) {
	s, dog, _, _ := suraHits(t, 100)
	s.presence.move(1000, protocol.Move{X: homeX + 7900, Y: homeY})
	for at := time.Duration(0); at < giveUpAfter; at += chaseStep {
		s.tick(noon.Add(at))
	}
	if dog.chase == nil {
		t.Fatal("gave up before ten seconds")
	}

	s.tick(noon.Add(giveUpAfter))

	if dog.chase != nil || len(s.fighting) != 0 || dog.x != dog.homeX || dog.y != dog.homeY {
		t.Fatalf("still fighting %v, at (%d, %d), want home at (%d, %d)", dog.chase != nil, dog.x, dog.y, dog.homeX, dog.homeY)
	}
}

// A monster that strayed 5000 from where it last bit gives up, however recently.
func TestAMonsterLedTooFarGivesUp(t *testing.T) {
	s, dog, _, _ := suraHits(t, 280)
	s.tick(noon)
	dog.chase.fromX = dog.x - leash

	s.tick(noon.Add(chaseStep))

	if dog.chase != nil || dog.x != dog.homeX {
		t.Fatalf("still fighting %v at %d, want home", dog.chase != nil, dog.x)
	}
}

// A monster whose target left the world gives up.
func TestAMonsterWhoseTargetLeftGivesUp(t *testing.T) {
	s, dog, _, _ := suraHits(t, 100)
	s.presence.leave(1000)

	s.tick(noon)

	if dog.chase != nil || len(s.fighting) != 0 {
		t.Fatal("still fighting someone who left")
	}
}

// A monster killed mid-fight stops fighting.
func TestAMonsterKilledMidFightStopsFighting(t *testing.T) {
	s, dog, _, c := suraHits(t, 100)
	s.tick(noon)
	c.Strength, c.Class = 90, account.Warrior
	s.presence.attack(1000, dog.p.add.VID, 0, someRand, noon.Add(time.Second))

	s.tick(noon.Add(time.Second))

	if dog.chase != nil || len(s.fighting) != 0 {
		t.Fatal("a dead monster is still fighting")
	}
}

// A bite that takes the last hit points kills her: she is told her hit points, the
// experience the death costs, and her death, and so is everyone who sees her of the death;
// the dog then gives up.
func TestABiteThatTakesTheLastHitPointsKills(t *testing.T) {
	s, dog, client, c := suraHits(t, 100)
	onlooker, onlookerClient := watcher(t)
	s.presence.enter(&present{out: onlooker, add: protocol.CharacterAdd{VID: 2000, X: homeX + 200, Y: homeY},
		char: &account.Character{HP: 1000}})
	read(t, client, addSize) // the onlooker
	c.HP, c.Exp = 10, 100

	s.tick(noon)

	read(t, client, moveSize+damageSize)
	if point, amount, value := pointChange(t, client, 1000); point != protocol.PointHP || amount != -10 || value != 0 {
		t.Fatalf("got point %d %+d to %d, want her last 10 hit points gone", point, amount, value)
	}
	if point, amount, value := pointChange(t, client, 1000); point != protocol.PointExp || amount != -6 || value != 94 {
		t.Fatalf("got point %d %+d to %d, want 2%% of 300 lost", point, amount, value)
	}
	if got := read(t, client, deadSize); string(got) != string(protocol.EncodeDead(1000)) {
		t.Fatalf("got %x, want her death", got)
	}
	read(t, onlookerClient, 2*addSize+moveSize) // the dog and her, then its attack
	if got := read(t, onlookerClient, deadSize); string(got) != string(protocol.EncodeDead(1000)) {
		t.Fatalf("the onlooker got %x, want her death", got)
	}
	s.tick(noon.Add(chaseStep))
	if dog.chase != nil {
		t.Fatal("the dog is still fighting a dead player")
	}
}

// A dead player can neither hit nor pick anything.
func TestADeadPlayerCannotHitOrPick(t *testing.T) {
	s, dog, client, c := suraHits(t, 100)
	c.HP = 0

	s.presence.target(1000, dog.p.add.VID)
	if got := s.presence.attack(1000, dog.p.add.VID, 0, someRand, noon.Add(time.Second)); got != blowRefused {
		t.Fatalf("a dead player's blow came to %v", got)
	}
	expectNothing(t, client)
}

// Restarting where she fell brings her back with half her hit points and mana: the restart
// window closes, she gets herself afresh and her points, then everyone in view.
func TestRestartingHereGivesHalfTheHitPoints(t *testing.T) {
	s, dog, client, c := suraHits(t, 100)
	c.HP = 0
	p := s.presence.all[1000]
	points, _ := c.Points()

	if !s.presence.revive(1000, false, 0, 0, noon) {
		t.Fatal("not revived")
	}

	expectRevival(t, client, p, points.MaxHP/2, points.MaxSP/2)
	if got := read(t, client, addSize); string(got) != string(dog.p.appearance()[0]) {
		t.Fatalf("got %x, want the dog again", got)
	}
	if p.add.X != homeX+100 {
		t.Fatalf("came back at %d, want where she fell", p.add.X)
	}
}

func expectRevival(t *testing.T, client net.Conn, p *present, hp, sp uint32) {
	t.Helper()
	closing := protocol.EncodeChat(protocol.ChatCommand, 1000, p.info.Empire, "CloseRestartWindow")
	if got := read(t, client, len(closing)); string(got) != string(closing) {
		t.Fatalf("got %x, want the restart window closed", got)
	}
	for _, want := range append(p.appearance(),
		protocol.EncodePointChange(1000, protocol.PointHP, 0, int32(hp)),   //nolint:gosec // small
		protocol.EncodePointChange(1000, protocol.PointSP, 0, int32(sp))) { //nolint:gosec // small
		if got := read(t, client, len(want)); string(got) != string(want) {
			t.Fatalf("got %x, want %x", got, want)
		}
	}
}

// Restarting in town brings her back at the start with all her hit points and mana; those
// who saw her where she fell see her go.
func TestRestartingInTownGivesAllTheHitPointsAtTheStart(t *testing.T) {
	s, _, client, c := suraHits(t, 100)
	onlooker, onlookerClient := watcher(t)
	s.presence.enter(&present{out: onlooker, add: protocol.CharacterAdd{VID: 2000, X: homeX + 200, Y: homeY},
		char: &account.Character{HP: 1000}})
	read(t, client, addSize)           // the onlooker
	read(t, onlookerClient, 2*addSize) // the dog and her
	c.HP = 0
	p := s.presence.all[1000]
	points, _ := c.Points()

	s.presence.revive(1000, true, homeX+20000, homeY, noon)

	expectRevival(t, client, p, points.MaxHP, points.MaxSP)
	if p.add.X != homeX+20000 {
		t.Fatalf("came back at %d, want the start", p.add.X)
	}
	if got := read(t, onlookerClient, deleteSize); string(got) != string(protocol.EncodeCharacterDelete(1000)) {
		t.Fatalf("the onlooker got %x, want her gone", got)
	}
	expectNothing(t, client)
}

// A restart while alive, which the client does not send, changes nothing.
func TestRestartingWhileAliveChangesNothing(t *testing.T) {
	s, _, client, c := suraHits(t, 100)
	hp := c.HP

	if s.presence.revive(1000, true, 0, 0, noon) {
		t.Fatal("revived a living player")
	}
	if c.HP != hp {
		t.Fatalf("hit points %d, want %d", c.HP, hp)
	}
	expectNothing(t, client)
}

// A fighting monster does not stroll off, even when a stroll is due.
func TestAFightingMonsterDoesNotStroll(t *testing.T) {
	s, dog, client, c := suraHits(t, 100)
	s.tick(noon)
	expectBite(t, client, dog, c)
	dog.nextStroll = noon

	s.tick(noon.Add(chaseStep))

	expectNothing(t, client)
}

// A player who died before the monster did gets no share of it, so no level-up brings
// them back behind the client's back.
func TestADeadPlayerGetsNoShare(t *testing.T) {
	s, dog, client, c := suraHits(t, 100)
	c.HP = 0
	killer, _ := watcher(t)
	strong := &account.Character{HP: 1000, SP: 1000, Level: 1, Class: account.Warrior, Stats: account.Stats{Strength: 90}}
	s.presence.enter(&present{out: killer, add: protocol.CharacterAdd{VID: 2000, X: homeX + 150, Y: homeY}, char: strong})
	read(t, client, addSize) // the killer

	s.presence.attack(2000, dog.p.add.VID, 0, someRand, noon)

	read(t, client, deadSize) // the dog
	expectNothing(t, client)
	if c.Exp != 0 || c.HP != 0 {
		t.Fatalf("the dead sura got %d exp and %d hit points", c.Exp, c.HP)
	}
}

// A monster killed after the loop last looked for deaths does not bite from its body.
func TestAMonsterKilledBeforeItsStepDoesNotBite(t *testing.T) {
	s, dog, _, c := suraHits(t, 100)
	s.engage([]provocation{{dog.p, s.presence.all[1000]}}, noon)
	c.Strength, c.Class = 90, account.Warrior
	s.presence.attack(1000, dog.p.add.VID, 0, someRand, noon)
	hp := c.HP

	s.fightOn(noon)

	if c.HP != hp || dog.chase != nil {
		t.Fatalf("hit points %d of %d, still fighting %v", c.HP, hp, dog.chase != nil)
	}
}

// Its target's blows keep a monster fighting however long since it last bit; others'
// blows do not.
func TestBlowsFromItsTargetKeepAMonsterFighting(t *testing.T) {
	s, dog, _, _ := suraHits(t, 280)
	s.tick(noon)
	later := noon.Add(9 * time.Second)

	s.engage([]provocation{{dog.p, &present{}}}, later)
	if !dog.chase.lastExchange.Equal(noon) {
		t.Fatal("another's blow counted")
	}
	s.engage([]provocation{{dog.p, s.presence.all[1000]}}, later)

	if !dog.chase.lastExchange.Equal(later) {
		t.Fatalf("last fought at %v, want her blow at %v", dog.chase.lastExchange, later)
	}
}

// A monster sent home does not stroll before the client has walked it there: 8,000 at
// 400 a second is 20 seconds, then its usual rest of at least five.
func TestAMonsterSentHomeStrollsOnlyOnceItCanBeThere(t *testing.T) {
	s, dog, _, _ := suraHits(t, 100)
	dog.x = dog.homeX + 8000

	s.goHome(dog, noon)

	if earliest := noon.Add(20*time.Second + wanderMin); dog.nextStroll.Before(earliest) {
		t.Fatalf("strolls at %v, before %v", dog.nextStroll, earliest)
	}
}
