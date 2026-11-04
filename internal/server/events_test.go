package server

import (
	"testing"
	"time"

	"golang.org/x/text/encoding/charmap"

	"github.com/SatoriAI/MT2-Go/internal/event"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// An event stored before the server starts is running when Hendra enters, and she is told
// in her own code page; the end shows in Polish time, whatever the machine's zone.
func TestAPlayerEnteringDuringAStoredEventIsTold(t *testing.T) {
	end := time.Date(2999, 12, 27, 18, 30, 0, 0, event.Zone)
	h := launch(t, nil, `INSERT INTO events (name, exp_percent, starts_at, ends_at)
		VALUES ('Święta', 200, now() - interval '1 hour', '`+end.Format(time.RFC3339)+`')`)
	hendra, _ := h.hendraInWorld()

	text, err := charmap.Windows1250.NewEncoder().String("Wydarzenie „Święta” trwa do 27.12 18:30: doświadczenie ×2.")
	if err != nil {
		t.Fatal(err)
	}
	hendra.expect(protocol.EncodeChat(protocol.ChatNotice, 0, 0, text))
}
