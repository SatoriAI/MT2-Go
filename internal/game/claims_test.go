package game

import (
	"context"
	"testing"
	"time"
)

func closed(c chan struct{}) bool {
	select {
	case <-c:
		return true
	default:
		return false
	}
}

func TestTakingACharacterOverStopsTheHolderAndWaitsForIt(t *testing.T) {
	svc := &Service{claims: map[uint32]*claim{}}
	stopped := make(chan struct{})
	holder, _, _ := svc.claimCharacter(context.Background(), 7, func() { close(stopped) })
	go func() {
		<-stopped
		svc.release(holder) // the holder saves, then lets go
	}()

	mine, displaced, err := svc.claimCharacter(context.Background(), 7, func() {})

	if err != nil || !displaced || svc.claims[7] != mine {
		t.Fatalf("got %v, displaced %v", err, displaced)
	}
}

// A newcomer that gives up waiting must not let a later one past the holder still saving:
// its own release waits for the holder's.
func TestAClaimGivenUpLetsGoOnlyAfterTheHolderDoes(t *testing.T) {
	svc := &Service{claims: map[uint32]*claim{}}
	holder, _, _ := svc.claimCharacter(context.Background(), 7, func() {}) // never lets go by itself
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, _, err := svc.claimCharacter(ctx, 7, func() {})

	if err == nil {
		t.Fatal("expected the newcomer to give up")
	}
	svc.mu.Lock()
	given := svc.claims[7]
	svc.mu.Unlock()
	if given == nil || given == holder || closed(given.done) {
		t.Fatal("the given-up claim let go while the holder still held the character")
	}
	svc.release(holder)
	select {
	case <-given.done:
	case <-time.After(time.Second):
		t.Fatal("the given-up claim never let go")
	}
	svc.mu.Lock()
	defer svc.mu.Unlock()
	if len(svc.claims) != 0 {
		t.Errorf("claims left behind: %v", svc.claims)
	}
}
