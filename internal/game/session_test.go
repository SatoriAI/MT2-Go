package game

import (
	"bytes"
	"strings"
	"testing"

	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

func TestAnUnexpectedPacketsErrorShowsOnlyTheStartOfItsBody(t *testing.T) {
	body := append(bytes.Repeat([]byte{0xAA}, maxLoggedBody), 0xBB)

	err := unexpected(protocol.Frame{Header: protocol.ClientMove, Body: body}, protocol.PhaseSelect)

	if msg := err.Error(); !strings.Contains(msg, "aa aa") || strings.Contains(msg, "bb") {
		t.Fatalf("error %q, want the first %d bytes only", msg, maxLoggedBody)
	}
}

// A login packet holds a key or a password, which must not reach the log.
func TestAnUnexpectedLoginPacketsErrorShowsNoneOfItsBody(t *testing.T) {
	for _, header := range []protocol.ClientHeader{protocol.ClientLogin, protocol.ClientLoginByKey} {
		err := unexpected(protocol.Frame{Header: header, Body: []byte("secret")}, protocol.PhaseGame)

		if msg := err.Error(); strings.Contains(msg, "73 65") {
			t.Fatalf("error %q shows the body of 0x%02X", msg, byte(header))
		}
	}
}
