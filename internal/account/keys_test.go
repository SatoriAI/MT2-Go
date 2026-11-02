package account

import (
	"testing"
	"time"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time { return c.t }

func newKeys() (*Keys, *fakeClock) {
	clock := &fakeClock{t: time.Unix(1_000_000, 0)}
	return NewKeys(clock.now), clock
}

func issue(t *testing.T, keys *Keys, id int64, name string) uint32 {
	t.Helper()
	key, ok := keys.Issue(id, name)
	if !ok {
		t.Fatalf("no key for %s", name)
	}
	return key
}

func TestAKeyIsRedeemedOnlyWithTheNameItWasIssuedFor(t *testing.T) {
	keys, _ := newKeys()
	key := issue(t, keys, 1, "admin")

	if _, ok := keys.Redeem(key, "other"); ok {
		t.Fatal("redeemed with another name")
	}
	if id, ok := keys.Redeem(key, "admin"); !ok || id != 1 {
		t.Fatalf("got %d %v", id, ok)
	}
}

func TestAKeyWorksForASecondConnectionAfterTheFirstCloses(t *testing.T) {
	keys, clock := newKeys()
	key := issue(t, keys, 1, "admin")
	keys.Redeem(key, "admin")
	keys.Release(1)
	clock.t = clock.t.Add(KeyGrace - time.Second)

	if _, ok := keys.Redeem(key, "admin"); !ok {
		t.Fatal("the world connection could not reuse the key")
	}
}

func TestAKeyExpiresAfterTheGracePeriodWithNoConnection(t *testing.T) {
	keys, clock := newKeys()
	key := issue(t, keys, 1, "admin")
	keys.Redeem(key, "admin")
	keys.Release(1)
	clock.t = clock.t.Add(KeyGrace + time.Second)

	if _, ok := keys.Redeem(key, "admin"); ok {
		t.Fatal("expired key accepted")
	}
}

func TestAKeyDoesNotExpireWhileAConnectionHoldsIt(t *testing.T) {
	keys, clock := newKeys()
	key := issue(t, keys, 1, "admin")
	keys.Redeem(key, "admin")
	clock.t = clock.t.Add(10 * KeyGrace)

	if _, ok := keys.Redeem(key, "admin"); !ok {
		t.Fatal("key held by an open connection expired")
	}
}

func TestAKeyIsRedeemedWhateverTheCaseOfTheName(t *testing.T) {
	keys, _ := newKeys()
	key := issue(t, keys, 1, "admin")

	if _, ok := keys.Redeem(key, "ADMIN"); !ok {
		t.Fatal("refused the name typed in another case")
	}
}

func TestNoKeyIsIssuedWhileTheAccountHoldsAGameConnection(t *testing.T) {
	keys, _ := newKeys()
	keys.Redeem(issue(t, keys, 1, "admin"), "admin")

	if _, ok := keys.Issue(1, "admin"); ok {
		t.Fatal("issued a second key to an account in the game")
	}
}

func TestANewKeyRevokesTheOneIssuedBefore(t *testing.T) {
	keys, _ := newKeys()
	first := issue(t, keys, 1, "admin")
	second := issue(t, keys, 1, "admin")

	if _, ok := keys.Redeem(first, "admin"); ok {
		t.Fatal("the older key still works, so two clients could share the account")
	}
	if _, ok := keys.Redeem(second, "admin"); !ok {
		t.Fatal("the newer key was refused")
	}
}

func TestAKeyIsIssuedAgainOnceEveryConnectionHasClosed(t *testing.T) {
	keys, _ := newKeys()
	key := issue(t, keys, 1, "admin")
	keys.Redeem(key, "admin")
	keys.Redeem(key, "admin")

	keys.Release(1)
	if _, ok := keys.Issue(1, "admin"); ok {
		t.Fatal("issued with one connection still open")
	}
	keys.Release(1)
	if _, ok := keys.Issue(1, "admin"); !ok {
		t.Fatal("refused with no connection open")
	}
}

func TestIssuingForOneAccountKeepsAnotherAccountsHeldKey(t *testing.T) {
	keys, clock := newKeys()
	held := issue(t, keys, 1, "admin")
	keys.Redeem(held, "admin")
	clock.t = clock.t.Add(10 * KeyGrace)

	issue(t, keys, 2, "other")

	if _, ok := keys.Redeem(held, "admin"); !ok {
		t.Fatal("a held key was dropped while issuing another account's")
	}
}
