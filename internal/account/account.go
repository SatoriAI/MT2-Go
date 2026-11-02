// Package account holds what the server knows about a player's account: the account
// itself, its characters, and the keys that carry a login from the auth port to the game
// port. These are the game's own types; how they look on the wire is the sessions' job.
package account

import (
	"context"
	"errors"
)

// Limits the client's login fields impose, in bytes.
const (
	MaxNameLength     = 30
	MaxPasswordLength = 16
)

// MaxCharacters is how many characters an account holds.
const MaxCharacters = 4

// ErrNotFound means no account has that name or id.
var ErrNotFound = errors.New("account: not found")

// Empire is the kingdom an account belongs to; 0 means not chosen yet.
type Empire uint8

const (
	EmpireNone   Empire = 0
	EmpireRed    Empire = 1 // Shinsoo
	EmpireYellow Empire = 2 // Chunjo
	EmpireBlue   Empire = 3 // Jinno
)

func (e Empire) Valid() bool { return e >= EmpireRed && e <= EmpireBlue }

type Account struct {
	ID           int64
	Name         string
	PasswordHash []byte
	Empire       Empire
}

type Class uint8

const (
	Warrior Class = 0
	Ninja   Class = 1
	Sura    Class = 2
	Shaman  Class = 3
)

type Sex uint8

const (
	Male   Sex = 0
	Female Sex = 1
)

// Character is one of an account's characters, as far as the game knows it so far.
type Character struct {
	ID          uint32
	Slot        int
	Name        string
	Class       Class
	Sex         Sex
	Level       uint8
	PlayMinutes uint32
	ST, HT, DX  uint8
	IQ          uint8
	Hair        uint16
	Map         uint16
	X, Y        int32
	SkillGroup  uint8
}

// Store is the persistence the login flow needs.
type Store interface {
	// AccountByName ignores case: names are unique regardless of it.
	AccountByName(ctx context.Context, name string) (Account, error)
	AccountByID(ctx context.Context, id int64) (Account, error)
	// ChooseEmpire sets the kingdom of an account that has none, and reports whether it
	// did: a kingdom, once chosen, stays.
	ChooseEmpire(ctx context.Context, id int64, empire Empire) (bool, error)
	Characters(ctx context.Context, accountID int64) ([]Character, error)
}
