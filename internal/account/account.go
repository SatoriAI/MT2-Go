// Package account holds what the server knows about a player's account: the account
// itself, its characters as the selection screen shows them, and the keys that carry a
// login from the auth port to the game port.
package account

import (
	"context"
	"errors"

	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// Limits the client's login fields impose.
const (
	MaxNameLength     = 30
	MaxPasswordLength = 16
)

// ErrNotFound means no account has that name or id.
var ErrNotFound = errors.New("account: not found")

type Account struct {
	ID           int64
	Name         string
	PasswordHash []byte
	Empire       protocol.Empire
}

// Store is the persistence the login flow needs.
type Store interface {
	AccountByName(ctx context.Context, name string) (Account, error)
	AccountByID(ctx context.Context, id int64) (Account, error)
	SetEmpire(ctx context.Context, id int64, empire protocol.Empire) error
	// Characters returns the account's slots; an empty slot has ID 0.
	Characters(ctx context.Context, accountID int64) ([protocol.CharacterSlots]protocol.CharacterSummary, error)
}
