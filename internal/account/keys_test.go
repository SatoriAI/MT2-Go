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

func TestAKeyIsRedeemedOnlyWithTheNameItWasIssuedFor(t *testing.T) {
	keys, _ := newKeys()
	key := keys.Issue(1, "admin")

	if _, ok := keys.Redeem(key, "other"); ok {
		t.Fatal("redeemed with another name")
	}
	if id, ok := keys.Redeem(key, "admin"); !ok || id != 1 {
		t.Fatalf("got %d %v", id, ok)
	}
}

func TestAKeyWorksForASecondConnectionAfterTheFirstCloses(t *testing.T) {
	keys, clock := newKeys()
	key := keys.Issue(1, "admin")
	keys.Redeem(key, "admin")
	keys.Release(1)
	clock.t = clock.t.Add(KeyGrace - time.Second)

	if _, ok := keys.Redeem(key, "admin"); !ok {
		t.Fatal("the world connection could not reuse the key")
	}
}

func TestAKeyExpiresAfterTheGracePeriodWithNoConnection(t *testing.T) {
	keys, clock := newKeys()
	key := keys.Issue(1, "admin")
	keys.Redeem(key, "admin")
	keys.Release(1)
	clock.t = clock.t.Add(KeyGrace + time.Second)

	if _, ok := keys.Redeem(key, "admin"); ok {
		t.Fatal("expired key accepted")
	}
}

func TestAKeyDoesNotExpireWhileAConnectionHoldsIt(t *testing.T) {
	keys, clock := newKeys()
	key := keys.Issue(1, "admin")
	keys.Redeem(key, "admin")
	clock.t = clock.t.Add(10 * KeyGrace)

	if _, ok := keys.Redeem(key, "admin"); !ok {
		t.Fatal("key held by an open connection expired")
	}
}

func TestOnlineCountsOpenGameConnections(t *testing.T) {
	keys, _ := newKeys()
	key := keys.Issue(1, "admin")
	keys.Redeem(key, "admin")
	keys.Redeem(key, "admin")

	keys.Release(1)
	if !keys.Online(1) {
		t.Fatal("offline with one connection still open")
	}
	keys.Release(1)
	if keys.Online(1) {
		t.Fatal("online with no connection")
	}
}
