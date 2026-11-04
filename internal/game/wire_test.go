package game

import (
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/account"
)

// The numbers are the client's RACE_* constants in playersettingmodule.py.
func TestEveryRaceNumberIsTheClientsAndLeadsBack(t *testing.T) {
	cases := []struct {
		class account.Class
		sex   account.Sex
		want  uint8
	}{
		{account.Warrior, account.Male, 0},
		{account.Ninja, account.Female, 1},
		{account.Sura, account.Male, 2},
		{account.Shaman, account.Female, 3},
		{account.Warrior, account.Female, 4},
		{account.Ninja, account.Male, 5},
		{account.Sura, account.Female, 6},
		{account.Shaman, account.Male, 7},
	}
	for _, c := range cases {
		if got := raceNumber(c.class, c.sex); got != c.want {
			t.Errorf("%s %s: got %d, want %d", c.sex, c.class, got, c.want)
		}
		if class, sex, ok := raceFromNumber(uint16(c.want)); !ok || class != c.class || sex != c.sex {
			t.Errorf("%d: got %s %s", c.want, sex, class)
		}
	}
	if _, _, ok := raceFromNumber(8); ok {
		t.Error("8 is not a race")
	}
}

func TestASkillGroupIsNumberedByItsPlaceInTheClass(t *testing.T) {
	if got := skillGroupNumber(account.Ninja, "archery"); got != 2 {
		t.Errorf("archery: got %d", got)
	}
	if got := skillGroupNumber(account.Ninja, account.SkillGroup("")); got != 0 {
		t.Errorf("none chosen: got %d", got)
	}
}

func TestEveryKingdomNumberLeadsBackToItsKingdom(t *testing.T) {
	for empire, number := range empireNumbers {
		if got, ok := empireFromNumber(number); !ok || got != empire {
			t.Errorf("%d: got %q", number, got)
		}
	}
	if _, ok := empireFromNumber(0); ok {
		t.Error("0 is not a kingdom")
	}
}

// The bits are the client's BLOCK_* constants in Packet.h.
func TestEveryBlockBitIsTheClientsAndLeadsBack(t *testing.T) {
	all := account.Blocks{account.BlockExchange, account.BlockParty, account.BlockGuild,
		account.BlockWhisper, account.BlockFriend, account.BlockPartyRequest}
	for i, block := range all {
		mask := uint32(1) << i
		if got := blocksFromMask(mask); len(got) != 1 || got[0] != block {
			t.Errorf("%d: got %v, want %s", mask, got, block)
		}
		if got := blockMask(account.Blocks{block}); got != mask {
			t.Errorf("%s: got %d, want %d", block, got, mask)
		}
	}
	if got := blockMask(blocksFromMask(63)); got != 63 {
		t.Errorf("all six: got %d", got)
	}
}

func TestBitsTheClientDoesNotNameAreDropped(t *testing.T) {
	if got := blocksFromMask(1<<3 | 1<<6 | 1<<31); len(got) != 1 || got[0] != account.BlockWhisper {
		t.Fatalf("got %v, want whisper only", got)
	}
}

// No block is an empty list, not nil, which the database would store as NULL.
func TestNoBlocksIsAnEmptyList(t *testing.T) {
	if got := blocksFromMask(0); got == nil || len(got) != 0 {
		t.Fatalf("got %#v", got)
	}
}

func TestPolishTextWithItsQuotesAndTimesSignIsShownByTheClient(t *testing.T) {
	text := "Wydarzenie „Święta” ×2: zażółć gęślą jaźń"
	if !ClientCanShow(text) {
		t.Fatal("cp1250 has every one of these characters")
	}
	if got := textFromClient(textToClient(text)); got != text {
		t.Fatalf("sent as %q", got)
	}
}

func TestAnEmojiOrChineseCharacterIsNotShownByTheClient(t *testing.T) {
	for _, text := range []string{"Święta 🎄", "春节"} {
		if ClientCanShow(text) {
			t.Errorf("%q: cp1250 lacks it", text)
		}
	}
}
