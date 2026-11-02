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
		"empire":        {EncodeEmpire(2), "5a02"},
		"channel status": {
			EncodeChannelStatus([]ChannelStatus{{Port: 13001, Status: ChannelNormal}}),
			"d201000000c9320101",
		},
	}
	for name, c := range cases {
		if !bytes.Equal(c.got, mustHex(t, c.want)) {
			t.Errorf("%s: got %x, want %s", name, c.got, c.want)
		}
	}
}

func TestCharacterListPlacesEveryFieldOfASlotWhereTheClientReadsIt(t *testing.T) {
	var slots [CharacterSlots]CharacterSummary
	slots[1] = CharacterSummary{
		ID: 7, Name: "Hendra", Race: 5, Level: 21, PlayMinutes: 0x0102,
		ST: 6, HT: 4, DX: 3, IQ: 2, BodyPart: 0x1122, HairPart: 0x3344,
		X: 469_300, Y: -2, SkillGroup: 1,
	}
	game := netip.MustParseAddrPort("192.168.1.200:13001")

	list := EncodeCharacterList(slots, game)

	if len(list) != 329 {
		t.Fatalf("length %d", len(list))
	}
	want := mustHex(t, "07000000"+ // id
		hex.EncodeToString(fixed("Hendra", 25))+
		"05"+"15"+ // job, level
		"02010000"+ // minutes played
		"06040302"+ // ST HT DX IQ
		"2211"+"00"+"4433"+ // body part, change-name flag, hair part
		"00000000"+ // unused
		"34290700"+"feffffff"+ // x, y
		"c0a801c8"+"c932"+ // address in network order, port
		"01") // skill group
	if got := list[1+63 : 1+2*63]; !bytes.Equal(got, want) {
		t.Errorf("slot 1:\n got %x\nwant %x", got, want)
	}
	if empty := list[1 : 1+63]; !bytes.Equal(empty[:4], []byte{0, 0, 0, 0}) {
		t.Errorf("slot 0 should be empty: %x", empty[:4])
	}
}
