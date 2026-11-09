package game

import (
	"math/rand/v2"
	"net"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/skill"
)

// casting is Rugor of warring as a level-10 Black Magic sura of INT 20, DEX 10 and CON 10,
// a sword in hand, every Black Magic skill the game has at its first level.
func casting(t *testing.T, dogs int) (*session, net.Conn) {
	t.Helper()
	s, client := warring(t, dogs)
	c := s.svc.presence.all[1000].char
	c.Class, c.SkillGroup, c.Weapon = account.Sura, "black_magic", 10
	c.Intelligence, c.Dexterity, c.Vitality, c.Strength = 20, 10, 10, 5
	c.Skills = map[account.Skill]uint8{skill.DarkStrike: 1, skill.FlameStrike: 1, skill.DarkProtection: 1,
		skill.GhostStrike: 1, skill.DarkOrb: 1}
	return s, client
}

// A spell's damage is its tooltip's, of the sura's INT, level, hit rate and the sword's
// magic attack, 15 to 19, and its roll of 50 to 100, whatever the dog's armour: Dark
// Strike at its first level does 130 + (555 to 633)·0.73·0.05, 150 to 153.
func TestDarkStrikeDoesItsTooltipsDamageThroughArmour(t *testing.T) {
	s, client := casting(t, 1)
	dog := s.svc.monsters[0].p
	dog.fight.defender.Armour = 200 // no blow gets through this

	if err := s.onUseSkill(usedOn(skill.DarkStrike, dog.add.VID)); err != nil {
		t.Fatal(err)
	}

	if got := read(t, client, pointChangeSize); string(got) != string(manaSpent(37, 463)) { // 30 + 140 × 0.05
		t.Fatalf("told %x", got)
	}
	if got := dog.fight.damageBy[s.svc.presence.all[1000]]; got < 150 || got > 153 {
		t.Fatalf("struck for %d, want 150 to 153", got)
	}
}

// Dark Strike lands on the dog picked up to 1500 off and up to four within 200 of it.
func TestDarkStrikeReachesFarAndSplashes(t *testing.T) {
	s, client := casting(t, 6)
	for _, m := range s.svc.monsters[:6] {
		s.svc.presence.mu.Lock()
		s.svc.presence.moveLocked(m.p, protocol.Move{X: homeX + 1400, Y: homeY})
		s.svc.presence.mu.Unlock()
	}
	drain(t, client)

	if err := s.onUseSkill(usedOn(skill.DarkStrike, s.svc.monsters[0].p.add.VID)); err != nil {
		t.Fatal(err)
	}

	read(t, client, pointChangeSize)
	if hurt, _ := struck(t, s, client); hurt != 5 {
		t.Fatalf("hurt %d, want 5", hurt)
	}
}

// Flame Strike strikes every dog within 500 around her, fifteen at most.
func TestFlameStrikeBurnsAroundHer(t *testing.T) {
	s, client := casting(t, 3)

	if err := s.onUseSkill(used(skill.FlameStrike)); err != nil {
		t.Fatal(err)
	}

	if got := read(t, client, pointChangeSize); string(got) != string(manaSpent(67, 433)) { // 60 + 140 × 0.05
		t.Fatalf("told %x", got)
	}
	if hurt, _ := struck(t, s, client); hurt != 4 { // the three at home and the one 250 off
		t.Fatalf("hurt %d, want 4", hurt)
	}
}

// Ghost Strike, at its first level, slows about a third of what it leaves alive, each to
// seven tenths of its pace and shown slowed, for ten seconds.
func TestGhostStrikeSlowsSome(t *testing.T) {
	s, client := casting(t, 9)
	s.rand = rand.New(rand.NewPCG(1, 1)) //nolint:gosec // a fixed seed
	if err := s.onUseSkill(usedOn(skill.GhostStrike, s.svc.monsters[0].p.add.VID)); err != nil {
		t.Fatal(err)
	}
	drain(t, client)
	slowed := 0
	for _, m := range s.svc.monsters {
		if m.p.fight.slowed {
			slowed++
			if int(m.p.add.MoveSpeed) != int(m.proto.MoveSpeed)*70/100 || mobAffects(m.p) != 1<<4 {
				t.Errorf("slowed to %d, affects %x", m.p.add.MoveSpeed, mobAffects(m.p))
			}
			if want := noon.Add(10*time.Second + 500*time.Millisecond); !m.p.fight.slowedUntil.Equal(want) {
				t.Errorf("slowed until %v, want %v", m.p.fight.slowedUntil, want)
			}
		}
	}
	if slowed == 0 || slowed == 9 {
		t.Fatalf("%d of 9 slowed at 35%%", slowed)
	}
}

// A slowed dog chasing her walks at seven tenths of its pace, and once its slow ends is
// back at its own, for everyone.
func TestASlowedMonsterWalksSlowerUntilItEnds(t *testing.T) {
	s, dog, client, _ := suraHits(t, 280)
	s.presence.mu.Lock()
	s.presence.slow(dog.p, noon.Add(time.Second))
	s.presence.moveLocked(s.presence.all[1000], protocol.Move{X: homeX + 2000, Y: homeY}) // so it walks a full step
	s.presence.mu.Unlock()
	drain(t, client)
	from := dog.p.add.X

	s.tick(noon)

	if walked := dog.p.add.X - from; walked != int32(walkPerSecond*0.7*chaseStep.Seconds()) {
		t.Fatalf("walked %d", walked)
	}
	s.tick(noon.Add(time.Second))
	if dog.p.fight.slowed || dog.p.add.MoveSpeed != dog.proto.MoveSpeed {
		t.Fatalf("still slowed at %d", dog.p.add.MoveSpeed)
	}
}

// Dark Protection raises her defence and takes a share of every bite from her mana: at
// INT 100 and its first level, 4 of every 100, 0.84·100·0.05.
func TestDarkProtectionTakesItsShareOfABiteFromMana(t *testing.T) {
	s, _, client, c := suraHits(t, 100)
	c.Intelligence, c.SP, c.HP = 100, 500, 10000
	c.Skills = map[account.Skill]uint8{skill.DarkProtection: 1}
	c.Buffs = map[account.Skill]time.Time{skill.DarkProtection: noon.Add(time.Minute)}

	s.tick(noon)

	read(t, client, moveSize)
	_, _, sp := pointChange(t, client, 1000)
	read(t, client, damageSize)
	_, amount, hp := pointChange(t, client, 1000)
	if took := 500 - sp; took < 0 || took != -amount*4/96 && took != -amount*4/96+1 {
		t.Fatalf("mana took %d of a bite that left %d off her hit points, now %d", took, -amount, hp)
	}
}

// With too little mana for its share, Dark Protection ends, and the bite is all hers.
func TestDarkProtectionEndsWithoutTheManaForItsShare(t *testing.T) {
	s, _, client, c := suraHits(t, 100)
	c.Intelligence, c.SP, c.HP = 100, 0, 10000
	c.Skills = map[account.Skill]uint8{skill.DarkProtection: 1}
	c.Buffs = map[account.Skill]time.Time{skill.DarkProtection: noon.Add(time.Minute)}

	s.tick(noon)

	drain(t, client)
	if _, running := c.Buffs[skill.DarkProtection]; running {
		t.Fatal("it runs on without mana")
	}
}

// Dark Protection drains 5 + 10k mana every five seconds it runs, told, and ends when she
// has less than that.
func TestDarkProtectionDrainsManaUntilThereIsTooLittle(t *testing.T) {
	s, client := casting(t, 1)
	c := s.svc.presence.all[1000].char
	if err := s.onUseSkill(used(skill.DarkProtection)); err != nil {
		t.Fatal(err)
	}
	drain(t, client)
	s.svc.nextPlayerRegen = noon.Add(time.Hour) // so her mana changes only by the drain
	s.svc.tick(noon)
	sp := c.SP

	s.svc.tick(noon.Add(drainEvery))
	if c.SP != sp-5 { // 5 + 10 × 0.05
		t.Fatalf("mana %d, from %d", c.SP, sp)
	}
	c.SP = 4
	s.svc.tick(noon.Add(2 * drainEvery))
	if _, running := c.Buffs[skill.DarkProtection]; running || len(s.svc.drainings) != 0 {
		t.Fatal("it runs on with too little mana")
	}
}

// Turning Dark Protection off costs nothing and starts no cooldown: its cooldown runs from
// when it was turned on, as the client counts it.
func TestTurningDarkProtectionOffStartsNoCooldown(t *testing.T) {
	s, client := casting(t, 1)
	if err := s.onUseSkill(used(skill.DarkProtection)); err != nil {
		t.Fatal(err)
	}
	on := s.svc.cooldowns.ready[cooldown{s.playing.ID, skill.DarkProtection}]
	s.svc.now = func() time.Time { return noon.Add(30 * time.Second) }
	if err := s.onUseSkill(used(skill.DarkProtection)); err != nil {
		t.Fatal(err)
	}
	drain(t, client)

	if _, running := s.svc.presence.all[1000].char.Buffs[skill.DarkProtection]; running {
		t.Fatal("still running")
	}
	if got := s.svc.cooldowns.ready[cooldown{s.playing.ID, skill.DarkProtection}]; !got.Equal(on) {
		t.Fatalf("ready at %v, want %v", got, on)
	}
}

// Dark Strike's damage spreads over its tooltip's range: over many casts it reaches 152 and
// 153, which only a roll above 50 gives with the sword's magic attack.
func TestDarkStrikesDamageSpreadsOverItsRange(t *testing.T) {
	s, _ := casting(t, 1)
	dog := s.svc.monsters[0].p
	pl := s.svc.presence.all[1000]
	pl.char.SP = 100000
	most := 0
	for range 100 {
		before := dog.fight.damageBy[pl]
		s.svc.presence.useSkill(1000, skill.DarkStrike, dog.add.VID, noon, 0, s.rand, func() bool { return true })
		most = max(most, dog.fight.damageBy[pl]-before)
	}
	if most < 152 {
		t.Fatalf("struck for at most %d", most)
	}
}
