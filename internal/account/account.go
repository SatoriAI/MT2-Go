// Package account holds what the server knows about a player's account: the account
// itself, its characters, and the keys that carry a login from the auth port to the game
// port. These are the game's own types; how they look on the wire is the sessions' job.
package account

import (
	"errors"
	"slices"
	"time"
)

// Limits the client's login fields impose, in bytes.
const (
	MaxNameLength     = 30
	MaxPasswordLength = 16
)

var (
	// ErrNotFound means no account, or no character, matches.
	ErrNotFound = errors.New("account: not found")
	// ErrNameTaken means another character already has the name, in any case.
	ErrNameTaken = errors.New("account: character name taken")
	// ErrSlotTaken means the account already has a character in that slot.
	ErrSlotTaken = errors.New("account: character slot taken")
	// ErrKingdomChanged means the account's kingdom is not the one the creation assumed.
	ErrKingdomChanged = errors.New("account: kingdom changed")
)

// Empire is the kingdom an account belongs to; "" means not chosen yet.
type Empire string

const (
	EmpireNone    Empire = ""
	EmpireShinsoo Empire = "shinsoo" // red
	EmpireChunjo  Empire = "chunjo"  // yellow
	EmpireJinno   Empire = "jinno"   // blue
)

type Account struct {
	ID           int64
	Name         string
	PasswordHash []byte
	Empire       Empire
}

type Class string

const (
	Warrior Class = "warrior"
	Ninja   Class = "ninja"
	Sura    Class = "sura"
	Shaman  Class = "shaman"
)

// SkillGroups are the two paths a character of the class chooses between, in the
// order the game numbers them.
func (c Class) SkillGroups() [2]SkillGroup {
	switch c {
	case Warrior:
		return [2]SkillGroup{"body", "mental"}
	case Ninja:
		return [2]SkillGroup{"dagger", "archery"}
	case Sura:
		return [2]SkillGroup{"weaponry", "black_magic"}
	default:
		return [2]SkillGroup{"dragon", "healing"}
	}
}

type Sex string

const (
	Male   Sex = "male"
	Female Sex = "female"
)

// SkillGroup is the path of skills a character follows; "" until it chooses one.
type SkillGroup string

// Skill is one of the skills a skill group teaches, by name.
type Skill string

// BuffCaster is what a buff cast by a character runs at: their level of the skill and
// their intelligence.
type BuffCaster struct {
	Level, Intelligence uint8
}

// Shortcut is what a taskbar slot holds: an item the character owns, by its id, or a
// skill it has learnt.
type Shortcut struct {
	Item  int64
	Skill Skill
}

// Block is something a character refuses from other players, chosen in the options.
type Block string

const (
	BlockExchange     Block = "exchange"
	BlockParty        Block = "party"
	BlockGuild        Block = "guild"
	BlockWhisper      Block = "whisper"
	BlockFriend       Block = "friend"
	BlockPartyRequest Block = "party_request"
)

// Blocks is what a character refuses, each at most once.
type Blocks []Block

func (b Blocks) Has(block Block) bool { return slices.Contains(b, block) }

// Character is one of an account's characters, as far as the game knows it so far.
type Character struct {
	ID         uint32
	Slot       int
	Name       string
	Class      Class
	Sex        Sex
	Level      uint8
	Exp        uint32 // towards the next level
	StatPoints uint16 // earned by levelling, not yet spent
	HP, SP     uint32 // kept between logins; both 0 for a character never saved in play
	// Weapon and Body are the vnums of the weapon and armour worn, or 0. Body is read with
	// the character for the select screen; in play both follow what is put on and off.
	Weapon, Body uint32
	// Head, Feet and Shield are the helmet, shoes and shield worn, or 0, and Arrow the
	// arrows, in play only.
	Head, Feet, Shield, Arrow uint32
	// UnseenUntil is when aggressive monsters may first see the character after it enters
	// the world or rises, in play only; kept here rather than on the world record's entry,
	// which every move scans.
	UnseenUntil time.Time
	// LastFought is in play only: when it last landed a blow or took a bite.
	// It lives here, not on the world record's entry, which must stay small (world-loop.md).
	LastFought  time.Time
	PlayMinutes uint32
	Stats
	Shape      uint8 // which of the class's two looks the player picked: 0 or 1
	Hair       uint16
	X, Y       int32
	SkillGroup SkillGroup
	// Skills is each skill learnt, at its level from 1; read on Start, as they are kept
	// apart from the character's row.
	Skills map[Skill]uint8
	// Buffs are in play only: each buff skill running, until when. BuffBy is who cast
	// those another cast, as the caster's skill level and INT they run at.
	Buffs  map[Skill]time.Time
	BuffBy map[Skill]BuffCaster
	Blocks Blocks
}
