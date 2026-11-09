package server

import (
	"context"
	"encoding/binary"
	"testing"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// exchange is what the client sends in a trade: an item offered from cell 0 is dropped on
// the trade window's first cell.
func exchange(sub byte, arg uint32) []byte {
	body := binary.LittleEndian.AppendUint32([]byte{byte(protocol.ClientExchange), sub}, arg)
	return append(body, 0, 1, 0, 0, sequenceByte)
}

func exchangeNews(n protocol.ExchangeNews) []byte { return protocol.EncodeExchange(n) }

// Hendra trades her two red potions to Kira beside her: both accept, and the potions are
// Kira's, stored, with the trade logged; both windows close.
func TestATradeMovesItemsBetweenPlayersAndIsKept(t *testing.T) {
	h := start(t)
	hendra, kira := h.hendraAndKira()
	hendra.changeCharacter()
	kira.read(5) // Hendra leaves her view
	h.give(27001, 2)
	hendra.startAgain()
	hendra.read(appearanceSize + 42) // Kira, then the potions
	kira.read(appearanceSize)        // Hendra again, as vid 3

	hendra.send(exchange(protocol.ExchangeStart, 2))
	hendra.expect(exchangeNews(protocol.ExchangeNews{Sub: protocol.ExchangeStarted, Arg: 2}))
	kira.expect(exchangeNews(protocol.ExchangeNews{Sub: protocol.ExchangeStarted, Arg: 3}))
	hendra.send(exchange(protocol.ExchangeAdd, 0))
	hendra.expect(exchangeNews(protocol.ExchangeNews{Sub: protocol.ExchangeAdded, Mine: true, Arg: 27001, Count: 2}))
	kira.expect(exchangeNews(protocol.ExchangeNews{Sub: protocol.ExchangeAdded, Arg: 27001, Count: 2}))
	time.Sleep(2 * time.Second) // an accept counts only 2 s after the last change
	kira.send(exchange(protocol.ExchangeAccept, 0))
	kira.expect(exchangeNews(protocol.ExchangeNews{Sub: protocol.ExchangeAccepted, Mine: true, Arg: 1}))
	hendra.expect(exchangeNews(protocol.ExchangeNews{Sub: protocol.ExchangeAccepted, Arg: 1}))
	hendra.send(exchange(protocol.ExchangeAccept, 0))

	ended := exchangeNews(protocol.ExchangeNews{Sub: protocol.ExchangeEnded})
	hendra.expect(exchangeNews(protocol.ExchangeNews{Sub: protocol.ExchangeAccepted, Mine: true, Arg: 1}))
	hendra.expect(inInventory(0, 0, 0))
	hendra.expect(ended)
	kira.expect(exchangeNews(protocol.ExchangeNews{Sub: protocol.ExchangeAccepted, Arg: 1}))
	kira.expect(inInventory(0, 27001, 2))
	kira.expect(ended)
	var kiras, trades int
	if err := h.accounts.Pool.QueryRow(context.Background(), `SELECT
		(SELECT coalesce(sum(i.count), 0) FROM items i JOIN characters c ON c.id = i.character_id WHERE c.name = 'Kira'),
		(SELECT count(*) FROM trades)`).Scan(&kiras, &trades); err != nil || kiras != 2 || trades != 1 {
		t.Fatalf("Kira keeps %d potions, %d trades logged, %v", kiras, trades, err)
	}
}

// A player changing character in a trade ends it: both windows close, and the other side
// is told.
func TestChangingCharacterEndsATrade(t *testing.T) {
	h := start(t)
	hendra, kira := h.hendraAndKira()
	hendra.send(exchange(protocol.ExchangeStart, 2))
	hendra.expect(exchangeNews(protocol.ExchangeNews{Sub: protocol.ExchangeStarted, Arg: 2}))
	kira.expect(exchangeNews(protocol.ExchangeNews{Sub: protocol.ExchangeStarted, Arg: 1}))

	hendra.say("/phase_select")

	hendra.expect(exchangeNews(protocol.ExchangeNews{Sub: protocol.ExchangeEnded}))
	hendra.expect(protocol.EncodePhase(protocol.PhaseSelect))
	kira.expect(exchangeNews(protocol.ExchangeNews{Sub: protocol.ExchangeEnded}))
	kira.expect(protocol.EncodeChat(protocol.ChatInfo, 0, 0, "Handel zosta\xb3 anulowany."))
}
