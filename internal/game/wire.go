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
	if k == mob.NPC {
		return protocol.EntityNPC
	}
	return protocol.EntityMonster
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
// the cells after them, the body at 90 and the weapon at 94.
const inventoryWindow uint8 = 1

var wornCells = map[item.Wear]uint16{item.Body: 90, item.Weapon: 94}

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
	skill.DragonSwirl:    62,
	skill.EnchantedBlade: 63,
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

// taskbarItem is the client's type for a taskbar slot that holds an inventory cell.
const taskbarItem uint8 = 1

// slotFilled tells the client the taskbar slot holds the item at the bag cell.
func slotFilled(slot uint8, cell uint16) []byte {
	return protocol.EncodeQuickslotAdd(protocol.QuickslotAdd{Slot: slot, Type: taskbarItem, Position: uint8(cell)}) //nolint:gosec // bag cells are under 90
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
