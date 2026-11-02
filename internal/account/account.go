// Package account holds what the server knows about a player's account: the account
// itself, its characters, and the keys that carry a login from the auth port to the game
// port. These are the game's own types; how they look on the wire is the sessions' job.
package account

import "errors"

// Limits the client's login fields impose, in bytes.
const (
	MaxNameLength     = 30
	MaxPasswordLength = 16
)

var (
	// ErrNotFound means no account has that name or id.
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

// Character is one of an account's characters, as far as the game knows it so far.
type Character struct {
	ID          uint32
	Slot        int
	Name        string
	Class       Class
	Sex         Sex
	Level       uint8
	PlayMinutes uint32
	Stats
	Shape      uint8 // which of the class's two looks the player picked: 0 or 1
	Hair       uint16
	X, Y       int32
	SkillGroup SkillGroup
}
