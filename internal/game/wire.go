// An adapter for the original TMP4 client, which numbers what the game names. It is the
// only place those numbers appear, and it goes when our own client replaces that one.
// docs/decisions/client-adapter.md has the reasoning.

package game

import (
	"strings"

	"golang.org/x/text/encoding/charmap"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/mob"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/skill"
)

var empireNumbers = map[account.Empire]uint8{
	account.EmpireShinsoo: 1,
	account.EmpireChunjo:  2,
	account.EmpireJinno:   3,
}

// empireFromNumber returns the kingdom the client's number stands for, or false.
func empireFromNumber(n uint8) (account.Empire, bool) {
	for e, number := range empireNumbers {
		if number == n {
			return e, true
		}
	}
	return account.EmpireNone, false
}

type look struct {
	class account.Class
	sex   account.Sex
}

// races is the client's single number for class and sex, from its playersettingmodule.py.
// It is not "female plus four": the female ninja and shaman are 1 and 3.
var races = map[look]uint8{
	{account.Warrior, account.Male}:   0,
	{account.Ninja, account.Female}:   1,
	{account.Sura, account.Male}:      2,
	{account.Shaman, account.Female}:  3,
	{account.Warrior, account.Female}: 4,
	{account.Ninja, account.Male}:     5,
	{account.Sura, account.Female}:    6,
	{account.Shaman, account.Male}:    7,
}

func raceNumber(c account.Class, s account.Sex) uint8 { return races[look{c, s}] }

// raceFromNumber returns the class and sex the client's race number stands for, or false.
func raceFromNumber(n uint16) (account.Class, account.Sex, bool) {
	for l, number := range races {
		if uint16(number) == n {
			return l.class, l.sex, true
		}
	}
	return "", "", false
}

// skillGroupNumber is 1 or 2 for the class's first or second path, 0 for none chosen.
func skillGroupNumber(c account.Class, g account.SkillGroup) uint8 {
	for i, group := range c.SkillGroups() {
		if group == g {
			return uint8(i + 1)
		}
	}
	return 0
}

// slots lays the characters out as the selection screen shows them.
func slots(characters []account.Character) [protocol.CharacterSlots]protocol.CharacterSummary {
	var out [protocol.CharacterSlots]protocol.CharacterSummary
	for _, c := range characters {
		out[c.Slot] = summary(c)
	}
	return out
}

// summary is one character as a slot of the selection screen shows it. The client draws
// the body part as a shape from the class's .msm file, where 0 and 1 are the two looks of
// the create screen; until characters wear armour, the body part is the look.
func summary(c account.Character) protocol.CharacterSummary {
	return protocol.CharacterSummary{
		ID:          c.ID,
		Name:        c.Name,
		Race:        raceNumber(c.Class, c.Sex),
		Level:       c.Level,
		PlayMinutes: c.PlayMinutes,
		ST:          c.Strength,
		HT:          c.Vitality,
		DX:          c.Dexterity,
		IQ:          c.Intelligence,
		BodyPart:    bodyPart(c),
		HairPart:    c.Hair,
		X:           c.X,
		Y:           c.Y,
		SkillGroup:  skillGroupNumber(c.Class, c.SkillGroup),
	}
}

// blockBits are the client's bits for what a character blocks (Packet.h, BLOCK_*).
var blockBits = []struct {
	block account.Block
	bit   uint32
}{
	{account.BlockExchange, 1 << 0},
	{account.BlockParty, 1 << 1},
	{account.BlockGuild, 1 << 2},
	{account.BlockWhisper, 1 << 3},
	{account.BlockFriend, 1 << 4},
	{account.BlockPartyRequest, 1 << 5},
}

// blocksFromMask reads the client's mask; bits it does not name are dropped.
func blocksFromMask(mask uint32) account.Blocks {
	blocks := account.Blocks{}
	for _, b := range blockBits {
		if mask&b.bit != 0 {
			blocks = append(blocks, b.block)
		}
	}
	return blocks
}

func blockMask(blocks account.Blocks) uint32 {
	var mask uint32
	for _, b := range blockBits {
		if blocks.Has(b.block) {
			mask |= b.bit
		}
	}
	return mask
}

// entityType is the client's number for the kind of mob.
func entityType(k mob.Kind) uint8 {
	switch k {
	case mob.NPC:
		return protocol.EntityNPC
	case mob.Stone:
		return protocol.EntityStone
	}
	return protocol.EntityMonster
}

// stunBit is the client's AFFECT_STUN, which draws a stunned monster asleep on its feet
// (InstanceBase.h, InstanceBaseEffect.cpp).
const stunBit = 1 << 5

// poisonBit is the client's AFFECT_POISON, which tints a poisoned monster, and slowBit its
// AFFECT_SLOW.
const (
	poisonBit = 1 << 3
	slowBit   = 1 << 4
	fireBit   = 1 << 28 // AFFECT_FIRE, flames on a burning monster
)

// mobAffects are the effects the client draws on a monster: its stun, poison, slow and fire.
func mobAffects(p *present) uint64 {
	var bits uint64
	if p.fight != nil && p.fight.stunned {
		bits |= stunBit
	}
	if p.fight != nil && p.fight.poisoned {
		bits |= poisonBit
	}
	if p.fight != nil && p.fight.slowed {
		bits |= slowBit
	}
	if p.fight != nil && p.fight.burning {
		bits |= fireBit
	}
	return bits
}

// smokeOf is the client's affect bit for a stone's smoke at the share of its hit points
// left, as the target window rounds it: its thickest at a tenth or less, then at 45 and
// 85 in a hundred or less, as the client's STONE_SMOKE flags are marked (InstanceBase.h),
// and none above.
func smokeOf(f *fight) uint64 {
	switch hp := f.hpPercent(); {
	case hp <= 10:
		return 1 << 7
	case hp <= 45:
		return 1 << 4
	case hp <= 85:
		return 1 << 1
	}
	return 0
}

// textToClient turns UTF-8 into the client's code page, cp1250, as textFromClient reads
// it. A character cp1250 lacks becomes "?", so encoding never fails.
func textToClient(text string) string {
	known := strings.Map(func(r rune) rune {
		if _, ok := charmap.Windows1250.EncodeRune(r); ok {
			return r
		}
		return '?'
	}, text)
	cp, _ := charmap.Windows1250.NewEncoder().String(known)
	return cp
}

// ClientCanShow reports whether every character of text exists in the client's code page,
// cp1250, so textToClient would send it unchanged rather than with a "?".
func ClientCanShow(text string) bool {
	for _, r := range text {
		if _, ok := charmap.Windows1250.EncodeRune(r); !ok {
			return false
		}
	}
	return true
}

// textFromClient turns text the client typed into UTF-8. The client writes in its Windows
// code page, cp1250 for Polish (docs/mechanics/chat.md). The few bytes cp1250 leaves
// undefined become U+FFFD; decoding never fails.
func textFromClient(text string) string {
	utf8, _ := charmap.Windows1250.NewDecoder().String(text)
	return utf8
}

// inventoryWindow is the client's number for the inventory among its item windows
// (GameType.h, INVENTORY): its cells 0 to 89 are the game's own, and what is worn is in
// the cells after them, by the client's WEAR_ places: the body at 90, the head at 91, the
// feet at 92, the weapon at 94, the arrows at 99 and the shield at 100 (ItemData.h).
const inventoryWindow uint8 = 1

var wornCells = map[item.Wear]uint16{item.Body: 90, item.Head: 91, item.Feet: 92, item.Weapon: 94, item.Arrow: 99, item.Shield: 100}

// placeFromWire returns where a position the client names is: an inventory cell, or
// where an item is worn. False for another window, or a cell the game does not use.
func placeFromWire(at protocol.ItemPos) (uint16, item.Wear, bool) {
	if at.Window != inventoryWindow {
		return 0, "", false
	}
	if at.Cell < item.Cells {
		return at.Cell, "", true
	}
	for worn, cell := range wornCells {
		if cell == at.Cell {
			return 0, worn, true
		}
	}
	return 0, "", false
}

// skillVNums are the client's numbers for the skills the game has, by which its skill
// window, skill levels and + buttons know them.
var skillVNums = map[account.Skill]uint8{
	skill.FingerStrike:    61,
	skill.DragonSwirl:     62,
	skill.EnchantedBlade:  63,
	skill.EnchantedArmour: 65,
	skill.ThreeWayCut:     1,
	skill.SwordSpin:       2,
	skill.Berserk:         3,
	skill.SwordAura:       4,
	skill.SpiritStrike:    16,
	skill.Bash:            17,
	skill.Stump:           18,
	skill.StrongBody:      19,
	skill.SwordStrike:     20,
	skill.Ambush:          31,
	skill.FastAttack:      32,
	skill.RollingDagger:   33,
	skill.DarkStrike:      76,
	skill.FlameStrike:     77,
	skill.DarkProtection:  79,
	skill.GhostStrike:     80,
	skill.DarkOrb:         81,
	skill.FlyingTalisman:  91,
	skill.ShootingDragon:  92,
	skill.DragonRoar:      93,
	skill.LightningThrow:  106,
	skill.SummonLightning: 107,
	skill.LightningClaw:   108,
	skill.Blessing:        94,
	skill.Reflection:      95,
	skill.DragonsHelp:     96,
	skill.Heal:            109,
	skill.Swiftness:       110,
	skill.AttackUp:        111,
}

// castingSpeed is what the client is told a character's casting speed is: 100, at which it
// counts a skill's cooldown as its table says, as this server does; at 0 it doubles it.
const castingSpeed = 100

// affectBits are the bits a buff sets in a character's effects, which the client draws,
// its AFFECT_ numbers (InstanceBase.h): 19 is the glow Enchanted Blade puts on the blade,
// 14 Berserk's, 15 Sword Aura's, 16 Strong Body's, 25 Dark Protection's, and the shaman's
// 22, 23, 24, 29 and 30.
var affectBits = map[account.Skill]uint64{
	skill.EnchantedBlade:  1 << 19,
	skill.EnchantedArmour: 1 << 21,
	skill.Berserk:         1 << 14,
	skill.SwordAura:       1 << 15,
	skill.StrongBody:      1 << 16,
	skill.DarkProtection:  1 << 25,
	skill.Blessing:        1 << 22,
	skill.Reflection:      1 << 23,
	skill.Swiftness:       1 << 24,
	skill.DragonsHelp:     1 << 29,
	skill.AttackUp:        1 << 30,
}

// affectsOf is the effects the client draws on a character: a bit for each buff running.
// A monster has none.
func affectsOf(c *account.Character) uint64 {
	if c == nil {
		return 0
	}
	var bits uint64
	for sk := range c.Buffs {
		bits |= affectBits[sk]
	}
	return bits
}

// skillByVNum is the skill the client's number names, if the game has it.
func skillByVNum(vnum int) (account.Skill, bool) {
	for s, v := range skillVNums {
		if int(v) == vnum {
			return s, true
		}
	}
	return "", false
}

// skillLevels lays out the levels of the skills learnt as the client takes them.
func skillLevels(learnt map[account.Skill]uint8) protocol.SkillLevels {
	var levels protocol.SkillLevels
	for s, level := range learnt {
		levels[skillVNums[s]] = level
	}
	return levels
}

// The client's types for a taskbar slot: an inventory cell, or a skill by its place in
// the skill window.
const (
	taskbarItem  uint8 = 1
	taskbarSkill uint8 = 2
)

// skillWindowSlots are where the client's skill window shows each skill, from 1 in each
// group, in the order its playersettingmodule lists them; each group has its own.
var skillWindowSlots = map[account.Skill]uint8{
	skill.FingerStrike:    1,
	skill.DragonSwirl:     2,
	skill.EnchantedBlade:  3,
	skill.EnchantedArmour: 5,
	skill.ThreeWayCut:     1,
	skill.SwordSpin:       2,
	skill.Berserk:         3,
	skill.SwordAura:       4,
	skill.SpiritStrike:    1,
	skill.Bash:            2,
	skill.Stump:           3,
	skill.StrongBody:      4,
	skill.SwordStrike:     5,
	skill.Ambush:          1,
	skill.FastAttack:      2,
	skill.RollingDagger:   3,
	skill.DarkStrike:      1,
	skill.FlameStrike:     2,
	skill.DarkProtection:  4,
	skill.GhostStrike:     5,
	skill.DarkOrb:         6,
	skill.FlyingTalisman:  1,
	skill.ShootingDragon:  2,
	skill.DragonRoar:      3,
	skill.LightningThrow:  1,
	skill.SummonLightning: 2,
	skill.LightningClaw:   3,
	skill.Blessing:        4,
	skill.Reflection:      5,
	skill.DragonsHelp:     6,
	skill.Heal:            4,
	skill.Swiftness:       5,
	skill.AttackUp:        6,
}

// skillByWindowSlot is the skill of the group at a place in the skill window, if the game
// has it.
func skillByWindowSlot(g account.SkillGroup, slot uint8) (account.Skill, bool) {
	for s, at := range skillWindowSlots {
		if p, _ := skill.Lookup(s); at == slot && p.Group == g {
			return s, true
		}
	}
	return "", false
}

// slotShown tells the client what the taskbar slot holds: a skill, or the item at its bag
// cell.
func slotShown(slot uint8, shown shownSlot) []byte {
	if shown.held.Skill != "" {
		return protocol.EncodeQuickslotAdd(protocol.QuickslotAdd{Slot: slot, Type: taskbarSkill, Position: skillWindowSlots[shown.held.Skill]})
	}
	return protocol.EncodeQuickslotAdd(protocol.QuickslotAdd{Slot: slot, Type: taskbarItem, Position: uint8(shown.cell)}) //nolint:gosec // bag cells are under 90
}

// yangVNum is the kind the client draws yang on the ground as, labelled with its name.
const yangVNum = 1

// groundVNum is the kind the client is told a ground item is: its own, or yang's.
func groundVNum(it *groundItem) uint32 {
	if it.gold > 0 {
		return yangVNum
	}
	return it.vnum
}

// itemPos is where the client shows an item: its cell, or its place among what is worn.
func itemPos(it item.Item) protocol.ItemPos {
	if it.Worn != "" {
		return protocol.ItemPos{Window: inventoryWindow, Cell: wornCells[it.Worn]}
	}
	return protocol.ItemPos{Window: inventoryWindow, Cell: it.Cell}
}

func itemSet(it item.Item) []byte { return protocol.EncodeItemSet(itemPos(it), it.VNum, it.Count) }

// itemCleared empties the place the item stood in.
func itemCleared(it item.Item) []byte { return protocol.EncodeItemSet(itemPos(it), 0, 0) }

// bodyPart is how the client is told to dress a character: in its armour, by the
// armour's vnum, which the client turns into a model through the item's own data, or
// else in the class's look the player picked, 0 or 1.
func bodyPart(c account.Character) uint16 {
	if c.Body != 0 {
		return uint16(c.Body) //nolint:gosec // armour vnums are under 2^16
	}
	return uint16(c.Shape)
}

// weaponPart is the weapon a character holds, by its vnum, or 0.
func weaponPart(c account.Character) uint16 {
	return uint16(c.Weapon) //nolint:gosec // weapon vnums are under 2^16
}
