package account

import (
	"crypto/rand"
	"encoding/binary"
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

// Issue returns a new key for the account. Until a game connection redeems it, it
// expires like a key whose connections have all closed.
func (k *Keys) Issue(accountID int64, name string) uint32 {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.forgetExpired()
	for {
		key := randomKey()
		if _, taken := k.keys[key]; key != 0 && !taken {
			k.keys[key] = issuedKey{accountID: accountID, name: name, expires: k.now().Add(KeyGrace)}
			return key
		}
	}
}

// Redeem checks a key presented with an account name, and on success counts one more
// open game connection for the account. Call Release when that connection closes.
func (k *Keys) Redeem(key uint32, name string) (accountID int64, ok bool) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.forgetExpired()
	issued, found := k.keys[key]
	if !found || issued.name != name {
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

// Online reports whether the account holds a game connection.
func (k *Keys) Online(accountID int64) bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.online[accountID] > 0
}

func (k *Keys) setExpiry(accountID int64, at time.Time) {
	for key, issued := range k.keys {
		if issued.accountID == accountID {
			issued.expires = at
			k.keys[key] = issued
		}
	}
}

func (k *Keys) forgetExpired() {
	now := k.now()
	for key, issued := range k.keys {
		if !issued.expires.IsZero() && now.After(issued.expires) {
			delete(k.keys, key)
		}
	}
}

// randomKey comes from crypto/rand because the key, with the account name, is all the
// game port asks for: a predictable key would let anyone take over a login in progress.
func randomKey() uint32 {
	var b [4]byte
	_, _ = rand.Read(b[:]) // crypto/rand.Read never fails
	return binary.LittleEndian.Uint32(b[:])
}
