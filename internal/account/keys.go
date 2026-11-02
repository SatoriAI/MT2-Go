package account

import (
	"crypto/rand"
	"encoding/binary"
	"strings"
	"sync"
	"time"
)

// KeyGrace is how long a login key survives after the account's last game connection
// closes. The client opens a fresh game connection to enter the world and presents the
// same key again, so the key must outlive the connection that first used it.
const KeyGrace = 60 * time.Second

type issuedKey struct {
	accountID int64
	name      string
	expires   time.Time // zero while the account holds a game connection
}

// Keys hands out login keys on the auth port and redeems them on the game port. Both
// ports run in this process, so the keys live in memory.
type Keys struct {
	mu     sync.Mutex
	now    func() time.Time
	keys   map[uint32]issuedKey
	online map[int64]int // open game connections per account
}

func NewKeys(now func() time.Time) *Keys {
	return &Keys{now: now, keys: map[uint32]issuedKey{}, online: map[int64]int{}}
}

// Issue returns a new key for the account, or false if the account holds a game
// connection. A new key replaces any the account was issued before, so two logins in
// quick succession cannot both reach the game port. Until a game connection redeems it,
// the key expires like one whose connections have all closed.
func (k *Keys) Issue(accountID int64, name string) (uint32, bool) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.online[accountID] > 0 {
		return 0, false
	}
	for key, issued := range k.keys {
		if issued.accountID == accountID || k.expired(issued) {
			delete(k.keys, key)
		}
	}
	for {
		key := randomKey()
		if _, taken := k.keys[key]; key != 0 && !taken {
			k.keys[key] = issuedKey{accountID: accountID, name: name, expires: k.now().Add(KeyGrace)}
			return key, true
		}
	}
}

// Redeem checks a key presented with an account name, in any case, and on success counts
// one more open game connection for the account. Call Release when that connection closes.
func (k *Keys) Redeem(key uint32, name string) (accountID int64, ok bool) {
	k.mu.Lock()
	defer k.mu.Unlock()
	issued, found := k.keys[key]
	if !found || !strings.EqualFold(issued.name, name) || k.expired(issued) {
		return 0, false
	}
	k.online[issued.accountID]++
	k.setExpiry(issued.accountID, time.Time{})
	return issued.accountID, true
}

// Release counts one game connection of the account as closed. When it was the last,
// the account's keys start their grace period.
func (k *Keys) Release(accountID int64) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.online[accountID]--
	if k.online[accountID] > 0 {
		return
	}
	delete(k.online, accountID)
	k.setExpiry(accountID, k.now().Add(KeyGrace))
}

func (k *Keys) setExpiry(accountID int64, at time.Time) {
	for key, issued := range k.keys {
		if issued.accountID == accountID {
			issued.expires = at
			k.keys[key] = issued
		}
	}
}

func (k *Keys) expired(issued issuedKey) bool {
	return !issued.expires.IsZero() && k.now().After(issued.expires)
}

// randomKey comes from crypto/rand because the key, with the account name, is all the
// game port asks for: a predictable key would let anyone take over a login in progress.
func randomKey() uint32 {
	var b [4]byte
	_, _ = rand.Read(b[:]) // crypto/rand.Read never fails
	return binary.LittleEndian.Uint32(b[:])
}
