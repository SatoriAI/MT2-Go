package protocol

import (
	"bytes"
	"encoding/hex"
	"errors"
	"net/netip"
	"testing"
)

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func fixed(s string, size int) []byte {
	field := make([]byte, size)
	copy(field, s)
	return field
}

func TestNextFrameDropsTheSequenceByteOfASequencedPacket(t *testing.T) {
	packet := append([]byte{byte(ClientEmpire), 2}, 0xAF)

	frame, used, err := NextFrame(packet)

	if err != nil || used != 3 || frame.Header != ClientEmpire || !bytes.Equal(frame.Body, []byte{2}) {
		t.Fatalf("got %v %d %v", frame, used, err)
	}
}

func TestNextFrameExpectsNoSequenceByteAfterTheHandshake(t *testing.T) {
	packet := make([]byte, 13)
	packet[0] = byte(ClientHandshake)

	_, used, err := NextFrame(append(packet, byte(ClientStateChecker)))

	if err != nil || used != 13 {
		t.Fatalf("used %d, err %v", used, err)
	}
}

func TestNextFrameWaitsForTheRestOfASplitPacket(t *testing.T) {
	_, _, err := NextFrame([]byte{byte(ClientLogin), 'a', 'd'})

	if !errors.Is(err, ErrIncomplete) {
		t.Fatalf("got %v", err)
	}
}

func TestNextFrameRejectsAnUnknownHeader(t *testing.T) {
	_, _, err := NextFrame([]byte{0x01})

	var unknown UnknownHeaderError
	if !errors.As(err, &unknown) || unknown.Header != 0x01 {
		t.Fatalf("got %v", err)
	}
}

func TestDecodeLoginReadsNameAndPasswordFromTheirFixedFields(t *testing.T) {
	body := append(append(fixed("admin", 31), fixed("secret", 17)...), make([]byte, 16)...)

	login, err := DecodeLogin(body)

	if err != nil || login != (Login{Name: "admin", Password: "secret"}) {
		t.Fatalf("got %+v %v", login, err)
	}
}

func TestDecodeLoginByKeyReadsTheKeyAfterTheName(t *testing.T) {
	body := append(append(fixed("admin", 31), 0x78, 0x56, 0x34, 0x12), make([]byte, 16)...)

	login, err := DecodeLoginByKey(body)

	if err != nil || login != (LoginByKey{Name: "admin", Key: 0x12345678}) {
		t.Fatalf("got %+v %v", login, err)
	}
}

func TestDecodeReportsATruncatedBody(t *testing.T) {
	if _, err := DecodeHandshake([]byte{1, 2, 3}); !errors.Is(err, ErrShort) {
		t.Fatalf("got %v", err)
	}
}

func TestEncodedPacketsMatchTheClientLayouts(t *testing.T) {
	cases := map[string]struct {
		got  []byte
		want string
	}{
		"phase":         {EncodePhase(PhaseAuth), "fd0a"},
		"handshake":     {EncodeHandshake(Handshake{ID: 1, Time: 2, Delta: -1}), "ff0100000002000000ffffffff"},
		"auth success":  {EncodeAuthSuccess(0x12345678), "967856341201"},
		"login failure": {EncodeLoginFailure(LoginWrongPassword), "0757524f4e47505744" + "00"},
		"empire":        {EncodeEmpire(EmpireYellow), "5a02"},
		"channel status": {
			EncodeChannelStatus([]ChannelStatus{{Port: 13001, Status: ChannelNormal}}),
			"d201000000c93201",
		},
	}
	for name, c := range cases {
		if !bytes.Equal(c.got, mustHex(t, c.want)) {
			t.Errorf("%s: got %x, want %s", name, c.got, c.want)
		}
	}
}

func TestCharacterListIs329BytesAndCarriesTheGameAddressPerSlot(t *testing.T) {
	var slots [CharacterSlots]CharacterSummary
	slots[1] = CharacterSummary{ID: 7, Name: "Hendra", Level: 21}
	game := netip.MustParseAddrPort("192.168.1.200:13001")

	list := EncodeCharacterList(slots, game)

	if len(list) != 329 {
		t.Fatalf("length %d", len(list))
	}
	second := list[1+63 : 1+2*63]
	if r := NewReader(second); r.U32() != 7 || r.String(25) != "Hendra" {
		t.Errorf("slot 1 does not hold the character: %x", second)
	}
	if !bytes.Equal(second[56:62], []byte{192, 168, 1, 200, 0xC9, 0x32}) {
		t.Errorf("address and port: got %x", second[56:62])
	}
}
