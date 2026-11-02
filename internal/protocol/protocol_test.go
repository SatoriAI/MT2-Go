package protocol

import (
	"bytes"
	"encoding/binary"
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

func TestDecodeCreateCharacterReadsSlotNameRaceAndShape(t *testing.T) {
	body := append([]byte{2}, fixed("Hendra", 25)...)
	body = append(body, 0x06, 0x00, 0x01, 0, 0, 0, 0) // race 6, shape 1, four stat points

	c, err := DecodeCreateCharacter(body)

	if err != nil || c != (CreateCharacter{Slot: 2, Name: "Hendra", Race: 6, Shape: 1}) {
		t.Fatalf("got %+v %v", c, err)
	}
}

func TestACreatedCharacterIsSentAsItsSlotAndTheListsRecord(t *testing.T) {
	c := CharacterSummary{ID: 7, Name: "Hendra", Race: 6, Level: 1, ST: 5, HT: 3, DX: 3, IQ: 5, X: 1, Y: 2}
	game := netip.MustParseAddrPort("127.0.0.1:13001")
	var slots [CharacterSlots]CharacterSummary
	slots[2] = c

	created := EncodeCharacterCreated(2, c, game)

	list := EncodeCharacterList(slots, game)
	if len(created) != 65 || created[0] != 0x08 || created[1] != 2 {
		t.Fatalf("header %x, length %d", created[:2], len(created))
	}
	if !bytes.Equal(created[2:], list[1+2*63:1+3*63]) {
		t.Errorf("record differs from the list's:\n got %x\nwant %x", created[2:], list[1+2*63:1+3*63])
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
		"time":          {EncodeTime(0x01020304), "6a04030201"},
		"channel":       {EncodeChannel(1), "7901"},
		"name taken":    {EncodeCreateFailure(CreateNameTaken), "0901"},
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

func TestPointsAreSentWholeWithEachValueAtItsIndex(t *testing.T) {
	points := EncodePoints(CharacterPoints{Level: 1, NextExp: 300, HP: 950, MaxHP: 950, ST: 5, MoveSpeed: 150})

	if len(points) != 1021 || points[0] != 0x10 {
		t.Fatalf("header %x, length %d", points[0], len(points))
	}
	at := func(index int) uint32 { return binary.LittleEndian.Uint32(points[1+4*index:]) }
	for index, want := range map[int]uint32{1: 1, 4: 300, 5: 950, 6: 950, 12: 5, 19: 150, 2: 0, 254: 0} {
		if got := at(index); got != want {
			t.Errorf("index %d: got %d, want %d", index, got, want)
		}
	}
}

func TestSkillLevelsAreSentAllUnlearnt(t *testing.T) {
	skills := EncodeSkillLevels()

	if len(skills) != 1531 || skills[0] != 0x4C || !bytes.Equal(skills[1:], make([]byte, 1530)) {
		t.Fatalf("got %d bytes starting %x", len(skills), skills[:2])
	}
}

func TestDecodeSelectCharacterReadsTheSlot(t *testing.T) {
	if slot, err := DecodeSelectCharacter([]byte{2}); err != nil || slot != 2 {
		t.Fatalf("got %d %v", slot, err)
	}
}

func TestMainCharacterPlacesEveryFieldWhereTheClientReadsIt(t *testing.T) {
	got := EncodeMainCharacter(MainCharacter{VID: 7, Race: 2, Name: "Hendra", X: 963400, Y: 278200, Empire: 3, SkillGroup: 1})

	want := mustHex(t, "71"+"07000000"+"0200"+ // header, vid, race
		hex.EncodeToString(fixed("Hendra", 25))+
		"48b30e00"+"b83e0400"+"00000000"+ // x, y, height
		"03"+"01") // empire, skill group
	if !bytes.Equal(got, want) {
		t.Errorf("got  %x\nwant %x", got, want)
	}
}

func TestCharacterAddPlacesEveryFieldWhereTheClientReadsIt(t *testing.T) {
	got := EncodeCharacterAdd(CharacterAdd{VID: 7, Rotation: 90, X: 963400, Y: 278200, Type: EntityPlayer,
		Race: 2, MoveSpeed: 150, AttackSpeed: 140})

	want := mustHex(t, "01"+"07000000"+"0000b442"+ // header, vid, rotation 90.0
		"48b30e00"+"b83e0400"+"00000000"+ // x, y, height
		"06"+"0200"+"96"+"8c"+"00"+ // type, race, move and attack speed, state
		"0000000000000000") // effects
	if !bytes.Equal(got, want) {
		t.Errorf("got  %x\nwant %x", got, want)
	}
}

func TestCharacterInfoSendsThePartsAsBodyWeaponHeadHair(t *testing.T) {
	got := EncodeCharacterInfo(CharacterInfo{VID: 7, Name: "Hendra", BodyPart: 1, HairPart: 3, Empire: 3, Level: 21})

	want := mustHex(t, "88"+"07000000"+hex.EncodeToString(fixed("Hendra", 25))+
		"0100"+"0000"+"0000"+"0300"+ // body, weapon, head, hair
		"03"+"00000000"+"15000000"+ // empire, guild, level
		"0000"+"00"+"00000000") // rank, player-kill mode, mount
	if !bytes.Equal(got, want) {
		t.Errorf("got  %x\nwant %x", got, want)
	}
}

func TestDecodeVersionReadsNameAndTimestamp(t *testing.T) {
	v, err := DecodeVersion(append(fixed("metin2client.bin", 33), fixed("1215955205", 33)...))

	if err != nil || v != (Version{Name: "metin2client.bin", Timestamp: "1215955205"}) {
		t.Fatalf("got %+v %v", v, err)
	}
}

func TestDecodeMoveReadsTypeDirectionPositionAndTime(t *testing.T) {
	body := []byte{1, 0, 64}
	body = binary.LittleEndian.AppendUint32(body, 963500)
	body = binary.LittleEndian.AppendUint32(body, 278300)
	body = binary.LittleEndian.AppendUint32(body, 12345)

	m, err := DecodeMove(body)

	if err != nil || m != (Move{Type: 1, Rotation: 64, X: 963500, Y: 278300, Time: 12345}) {
		t.Fatalf("got %+v %v", m, err)
	}
}
