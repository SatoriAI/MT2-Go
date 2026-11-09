package protocol

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"net/netip"
	"strings"
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

// chat is a chat packet as the client frames it: header, total size, type, the text and
// its NUL, then the sequence byte.
func chat(text string) []byte {
	size := 1 + 2 + 1 + len(text) + 1
	packet := binary.LittleEndian.AppendUint16([]byte{byte(ClientChat)}, uint16(size)) //nolint:gosec // test lines are short
	return append(append(append(packet, 0), text...), 0, 0xAF)
}

func TestNextFrameReadsASizedPacketsLengthFromIt(t *testing.T) {
	packet := chat("hi")

	frame, used, err := NextFrame(append(packet, byte(ClientStateChecker)))

	if err != nil || used != len(packet) || frame.Header != ClientChat || !bytes.Equal(frame.Body, []byte{0, 'h', 'i', 0}) {
		t.Fatalf("got %v %d %v", frame, used, err)
	}
}

func TestNextFrameWaitsForTheRestOfASplitSizedPacket(t *testing.T) {
	packet := chat("hello")
	for _, cut := range []int{1, 2, len(packet) - 1} {
		if _, _, err := NextFrame(packet[:cut]); !errors.Is(err, ErrIncomplete) {
			t.Errorf("cut at %d: got %v", cut, err)
		}
	}
}

func TestNextFrameRefusesASizeNoPacketCanHave(t *testing.T) {
	for _, size := range []uint16{0, 2, 1025, 0xFFFF} {
		packet := binary.LittleEndian.AppendUint16([]byte{byte(ClientChat)}, size)
		if _, _, err := NextFrame(packet); !errors.Is(err, ErrBadSize) {
			t.Errorf("size %d: got %v", size, err)
		}
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

// SKILL_LEVEL_NEW is 255 skills of 6 bytes, each at the client's number for it: master
// type 0, the level, a reading time of 0. Dragon Swirl (62) at 3 is the only one learnt.
func TestSkillLevelsAreSentAtEachSkillsNumber(t *testing.T) {
	var levels SkillLevels
	levels[62] = 3

	skills := EncodeSkillLevels(levels)

	want := make([]byte, 1531)
	want[0], want[1+62*6+1] = 0x4C, 3
	if !bytes.Equal(skills, want) {
		t.Fatalf("got %d bytes, 62 at %x", len(skills), skills[1+62*6:1+63*6])
	}
}

// Skill points go at the client's index for them, 28.
func TestSkillPointsAreSentAtTheirIndex(t *testing.T) {
	points := EncodePoints(CharacterPoints{SkillPoints: 2})[1:]

	if got := binary.LittleEndian.Uint32(points[4*PointSkillPoints:]); got != 2 || PointSkillPoints != 28 {
		t.Fatalf("skill points %d at %d", got, PointSkillPoints)
	}
}

// SCRIPT is its size, the skin, the text's length, then the text; the client reads it by
// the size.
func TestAnNPCsDialogIsItsSkinAndText(t *testing.T) {
	got := EncodeScript(ScriptDialog, []byte("Hi[ENTER][DONE]"))
	want := append(mustHex(t, "2d"+"1500"+"01"+"0f00"), []byte("Hi[ENTER][DONE]")...)
	if !bytes.Equal(got, want) {
		t.Fatalf("got  %x\nwant %x", got, want)
	}
}

// The skill group chosen is one byte; the answer to a dialog is the button's index, framed
// with its sequence byte.
func TestTheSkillGroupAndADialogsAnswer(t *testing.T) {
	if got := EncodeChangeSkillGroup(1); !bytes.Equal(got, mustHex(t, "7001")) {
		t.Fatalf("group: got %x", got)
	}
	frame, used, err := NextFrame([]byte{0x1D, 1, 0xAF})
	if err != nil || used != 3 {
		t.Fatalf("framed %d, %v", used, err)
	}
	if answer, err := DecodeScriptAnswer(frame.Body); err != nil || answer != 1 {
		t.Fatalf("answer %d, %v", answer, err)
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
		Race: 2, MoveSpeed: 150, AttackSpeed: 140}, 0)

	want := mustHex(t, "01"+"07000000"+"0000b442"+ // header, vid, rotation 90.0
		"48b30e00"+"b83e0400"+"00000000"+ // x, y, height
		"06"+"0200"+"96"+"8c"+"00"+ // type, race, move and attack speed, state
		"0000000000000000") // effects
	if !bytes.Equal(got, want) {
		t.Errorf("got  %x\nwant %x", got, want)
	}
}

func TestCharacterInfoSendsThePartsAsBodyWeaponHeadHair(t *testing.T) {
	got := EncodeCharacterInfo(CharacterInfo{VID: 7, Name: "Hendra", BodyPart: 11600, WeaponPart: 10, HairPart: 3,
		Empire: 3, Level: 21})

	want := mustHex(t, "88"+"07000000"+hex.EncodeToString(fixed("Hendra", 25))+
		"502d"+"0a00"+"0000"+"0300"+ // body (the armour 11600), weapon (the sword 10), head, hair
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

func TestAMoveIsPassedOnWithTheMoversVIDAndNoDuration(t *testing.T) {
	got := EncodeMove(7, Move{Type: 1, Arg: 2, Rotation: 18, X: 963500, Y: 278300, Time: 12345})

	want := mustHex(t, "03"+"01"+"02"+"12"+"07000000"+ // header, type, arg, rotation, vid
		"acb30e00"+"1c3f0400"+"39300000"+"00000000") // x, y, time, duration
	if !bytes.Equal(got, want) {
		t.Errorf("got  %x\nwant %x", got, want)
	}
}

func TestCharacterDeleteNamesTheCharacter(t *testing.T) {
	if got, want := EncodeCharacterDelete(7), mustHex(t, "02"+"07000000"); !bytes.Equal(got, want) {
		t.Errorf("got  %x\nwant %x", got, want)
	}
}

func TestDecodeChatKeepsTheClientsBytes(t *testing.T) {
	c, err := DecodeChat([]byte{0, 'z', 0xB9, 'b', 0}) // "ząb" in the client's Polish code page

	if err != nil || c != (Chat{Type: ChatNormal, Text: "z\xb9b"}) {
		t.Fatalf("got %+q %v", c, err)
	}
}

func TestDecodeChatRefusesTextWithoutItsTerminator(t *testing.T) {
	if _, err := DecodeChat([]byte{0, 'h', 'i'}); !errors.Is(err, ErrNoTerminator) {
		t.Fatalf("got %v", err)
	}
}

func TestChatIsSentWithItsTotalSizeTypeSpeakerAndText(t *testing.T) {
	got := EncodeChat(ChatNormal, 7, 3, "Hendra : hi")

	want := mustHex(t, "04"+"1500"+"00"+"07000000"+"03"+ // header, size 21, type, vid, kingdom
		hex.EncodeToString([]byte("Hendra : hi"))+"00")
	if !bytes.Equal(got, want) {
		t.Errorf("got  %x\nwant %x", got, want)
	}
}

func TestDecodeWhisperReadsTheNameAndKeepsTheClientsBytes(t *testing.T) {
	body := append([]byte("Kira"), make([]byte, characterNameSize-4)...)
	body = append(body, 'z', 0xB9, 'b', 0)

	w, err := DecodeWhisper(body)

	if err != nil || w != (Whisper{To: "Kira", Text: "z\xb9b"}) {
		t.Fatalf("got %+q %v", w, err)
	}
}

// The client copies at most 24 bytes of the name and leaves the 25th as it was in its
// memory, so that byte is never part of the name.
func TestDecodeWhisperIgnoresTheLastByteOfTheNameField(t *testing.T) {
	body := append(bytes.Repeat([]byte{'a'}, characterNameSize-1), 0x7F, 'h', 'i', 0)

	w, err := DecodeWhisper(body)

	if err != nil || w != (Whisper{To: strings.Repeat("a", characterNameSize-1), Text: "hi"}) {
		t.Fatalf("got %+q %v", w, err)
	}
}

func TestDecodeWhisperRefusesTextWithoutItsTerminator(t *testing.T) {
	body := append(make([]byte, characterNameSize), 'h', 'i')
	if _, err := DecodeWhisper(body); !errors.Is(err, ErrNoTerminator) {
		t.Fatalf("got %v", err)
	}
}

func TestDecodeWhisperRefusesABodyShorterThanTheName(t *testing.T) {
	if _, err := DecodeWhisper([]byte("Kira")); !errors.Is(err, ErrShort) {
		t.Fatalf("got %v", err)
	}
}

func TestAWhisperIsSentWithItsSizeTypeNameAndText(t *testing.T) {
	got := EncodeWhisper(WhisperLine, "Hendra", "hi")

	want := mustHex(t, "22"+"2000"+"00"+ // header, size 32, type
		hex.EncodeToString([]byte("Hendra"))+strings.Repeat("00", characterNameSize-6)+
		hex.EncodeToString([]byte("hi"))+"00")
	if !bytes.Equal(got, want) {
		t.Errorf("got  %x\nwant %x", got, want)
	}
}

func TestNotOnlineIsSentWithTheNameAndNoText(t *testing.T) {
	got := EncodeWhisper(WhisperNotOnline, "Nobody", "")

	want := mustHex(t, "22"+"1e00"+"01"+ // header, size 30, type
		hex.EncodeToString([]byte("Nobody"))+strings.Repeat("00", characterNameSize-6)+"00")
	if !bytes.Equal(got, want) {
		t.Errorf("got  %x\nwant %x", got, want)
	}
}

func TestDecodeAttackReadsTheSkillAndVictim(t *testing.T) {
	a, err := DecodeAttack([]byte{0, 0x2A, 0, 0, 0, 0x11, 0x22})

	if err != nil || a != (Attack{Skill: 0, Victim: 42}) {
		t.Fatalf("got %+v %v", a, err)
	}
}

func TestDecodeAttackRefusesAShortBody(t *testing.T) {
	if _, err := DecodeAttack([]byte{0, 1, 0, 0, 0}); !errors.Is(err, ErrShort) {
		t.Fatalf("got %v", err)
	}
}

func TestDecodeTargetReadsTheVid(t *testing.T) {
	if vid, err := DecodeTarget([]byte{7, 0, 0, 0}); err != nil || vid != 7 {
		t.Fatalf("got %d %v", vid, err)
	}
}

func TestTheTargetsHitPointsAreSentAsAPercentage(t *testing.T) {
	if got, want := EncodeTarget(7, 64), mustHex(t, "3f"+"07000000"+"40"); !bytes.Equal(got, want) {
		t.Errorf("got  %x\nwant %x", got, want)
	}
}

func TestDamageIsSentWithItsVictimFlagsAndAmount(t *testing.T) {
	if got, want := EncodeDamage(7, DamageNormal, 13), mustHex(t, "87"+"07000000"+"01"+"0d000000"); !bytes.Equal(got, want) {
		t.Errorf("got  %x\nwant %x", got, want)
	}
}

func TestDeathNamesTheCharacter(t *testing.T) {
	if got, want := EncodeDead(7), mustHex(t, "0e"+"07000000"); !bytes.Equal(got, want) {
		t.Errorf("got  %x\nwant %x", got, want)
	}
}

// A body is sent with the client's dead state, so it is drawn lying.
func TestABodyIsAddedWithTheDeadState(t *testing.T) {
	if got := EncodeCharacterAdd(CharacterAdd{VID: 7, Dead: true}, 0)[26]; got != 1 {
		t.Fatalf("state %d, want 1", got)
	}
}

// The header is an int in the client's struct: its byte, three zeros, then the rest.
func TestAPointChangeNamesTheCharacterPointAmountAndValue(t *testing.T) {
	got := EncodePointChange(7, PointExp, 15, 305)

	want := mustHex(t, "11000000"+"07000000"+"03"+"0f000000"+"31010000")
	if !bytes.Equal(got, want) {
		t.Errorf("got  %x\nwant %x", got, want)
	}
}

func TestUnspentStatPointsGoWhereTheCharacterWindowReadsThem(t *testing.T) {
	got := EncodePoints(CharacterPoints{StatPoints: 6})

	if v := binary.LittleEndian.Uint32(got[1+4*PointStatPoints:]); v != 6 {
		t.Fatalf("stat points %d at index %d", v, PointStatPoints)
	}
}

// Each packet the inventory window can send is cut at the size the client sends it with,
// so none of them ends the connection: the sizes of a move, a double-click, a drag to the
// taskbar and a drop to the ground were read off the real client.
func TestNextFrameCutsEveryItemPacketAtItsSize(t *testing.T) {
	sizes := map[ClientHeader]int{
		ClientItemUse: 4, ClientItemDrop: 8, ClientItemMove: 8, ClientQuickslotAdd: 4,
		ClientQuickslotDel: 2, ClientQuickslotSwap: 3, ClientItemDrop2: 9, ClientExchange: 10,
		ClientItemUseToItem: 7, ClientGiveItem: 9, ClientItemPickup: 5,
	}
	for header, size := range sizes {
		packet := append(append([]byte{byte(header)}, make([]byte, size-1)...), 0xAF, byte(ClientEnterGame))

		frame, used, err := NextFrame(packet)

		if err != nil || used != size+1 || frame.Header != header || len(frame.Body) != size-1 {
			t.Errorf("header 0x%02X: used %d of %d, body %d, err %v", byte(header), used, size+1, len(frame.Body), err)
		}
	}
}

// A drag reads as the real client sent it: the sword from cell 0 to cell 17 of the
// inventory, whole.
func TestDecodeItemMoveReadsBothCellsAndTheCount(t *testing.T) {
	m, err := DecodeItemMove(mustHex(t, "010000011100"+"01"))

	if err != nil || m != (ItemMove{From: ItemPos{Window: 1, Cell: 0}, To: ItemPos{Window: 1, Cell: 17}, Count: 1}) {
		t.Fatalf("got %+v %v", m, err)
	}
}

func TestDecodeItemMoveRefusesAShortBody(t *testing.T) {
	if _, err := DecodeItemMove(mustHex(t, "010000011100")); !errors.Is(err, ErrShort) {
		t.Fatalf("got %v", err)
	}
}

// ITEM_SET: the cell, the kind and count, then three empty sockets and seven empty bonuses.
func TestAnItemIsSetInACell(t *testing.T) {
	want := mustHex(t, "14"+"01"+"1100"+"79690000"+"32"+strings.Repeat("00", 12+21))

	if got := EncodeItemSet(ItemPos{Window: 1, Cell: 17}, 27001, 50); !bytes.Equal(got, want) {
		t.Errorf("got  %x\nwant %x", got, want)
	}
}

// CHARACTER_UPDATE re-dresses Hendra in the armour 11600 and the sword 10, with her speeds;
// the rest is 0.
func TestACharacterIsReDressed(t *testing.T) {
	got := EncodeCharacterUpdate(CharacterInfo{VID: 7, BodyPart: 11600, WeaponPart: 10, HairPart: 3}, CharacterAdd{MoveSpeed: 150, AttackSpeed: 150}, 0)

	want := mustHex(t, "13"+"07000000"+"502d"+"0a00"+"0000"+"0300"+ // vid, armour, weapon, head, hair
		"96"+"96"+"00"+ // move and attack speed, state
		strings.Repeat("00", 8+4+2+1+4)) // affects, guild, alignment, player-kill mode, mount
	if !bytes.Equal(got, want) {
		t.Errorf("got  %x\nwant %x", got, want)
	}
}

// DEF goes at the client's index for it, 20.
func TestTheDefenceIsSentAtItsIndex(t *testing.T) {
	points := EncodePoints(CharacterPoints{Defence: 16})[1:]

	if got := binary.LittleEndian.Uint32(points[4*PointDefence:]); got != 16 {
		t.Fatalf("DEF %d", got)
	}
}

// Yang goes at the client's index for gold, 11.
func TestYangIsSentAtItsIndex(t *testing.T) {
	points := EncodePoints(CharacterPoints{Gold: 1234})[1:]

	if got := binary.LittleEndian.Uint32(points[4*PointGold:]); got != 1234 || PointGold != 11 {
		t.Fatalf("yang %d at %d", got, PointGold)
	}
}

// GROUND_ADD as the real client took it: a red potion at (977491, 272599), height 0.
func TestAnItemIsLaidOnTheGround(t *testing.T) {
	want := mustHex(t, "1a"+"53ea0e00"+"d7280400"+"00000000"+"a1bb0d00"+"79690000")

	if got := EncodeGroundAdd(900001, 977491, 272599, 27001); !bytes.Equal(got, want) {
		t.Errorf("got  %x\nwant %x", got, want)
	}
}

func TestAnItemIsTakenOffTheGround(t *testing.T) {
	if got, want := EncodeGroundDel(900001), mustHex(t, "1b"+"a1bb0d00"); !bytes.Equal(got, want) {
		t.Errorf("got %x, want %x", got, want)
	}
}

// OWNERSHIP names the owner in 25 bytes, or nobody with an empty name.
func TestAnItemsOwnerIsNamedOrCleared(t *testing.T) {
	if got, want := EncodeItemOwner(900001, "Szepta"), mustHex(t, "1f"+"a1bb0d00"+hex.EncodeToString(fixed("Szepta", 25))); !bytes.Equal(got, want) {
		t.Errorf("got  %x\nwant %x", got, want)
	}
	if got, want := EncodeItemOwner(900001, ""), mustHex(t, "1f"+"a1bb0d00"+strings.Repeat("00", 25)); !bytes.Equal(got, want) {
		t.Errorf("got  %x\nwant %x", got, want)
	}
}

// A pick-up reads as the real client sent it: vid 900002.
func TestDecodeItemPickupReadsTheVid(t *testing.T) {
	if vid, err := DecodeItemPickup(mustHex(t, "a2bb0d00")); err != nil || vid != 900002 {
		t.Fatalf("got %d %v", vid, err)
	}
}

// SPECIAL_EFFECT names the effect, then the character it is drawn on.
func TestAPotionsEffectIsDrawnOnTheCharacter(t *testing.T) {
	if got, want := EncodeSpecialEffect(EffectRedPotion, 900001), mustHex(t, "72"+"01"+"a1bb0d00"); !bytes.Equal(got, want) {
		t.Errorf("got %x, want %x", got, want)
	}
	if got, want := EncodeSpecialEffect(EffectBluePotion, 900001), mustHex(t, "72"+"02"+"a1bb0d00"); !bytes.Equal(got, want) {
		t.Errorf("got %x, want %x", got, want)
	}
}

// A SHOP from the client is as long as its subheader says, then its sequence byte: a buy
// of cell 7, a sale of 5 from inventory cell 12, and the window closing.
func TestAShopRequestIsFramedByItsSubheader(t *testing.T) {
	stream := []byte{0x32, 1, 1, 7, 0xAF, 0x32, 3, 12, 5, 0xAF, 0x32, 0, 0xAF}
	want := []ShopRequest{{Sub: ShopBuy, Pos: 7}, {Sub: ShopSell2, Cell: 12, Count: 5}, {Sub: ShopEnd}}
	for _, w := range want {
		frame, used, err := NextFrame(stream)
		if err != nil {
			t.Fatal(err)
		}
		got, err := DecodeShop(frame.Body)
		if err != nil || got != w {
			t.Fatalf("got %+v, %v; want %+v", got, err, w)
		}
		stream = stream[used:]
	}
	if len(stream) != 0 {
		t.Fatalf("%d bytes left", len(stream))
	}
}

// A SHOP with a subheader the server does not know cannot be delimited, and one cut
// before its subheader waits for it.
func TestAShopRequestWithAnUnknownSubheaderIsRefused(t *testing.T) {
	if _, _, err := NextFrame([]byte{0x32, 9, 0, 0, 0xAF}); !errors.Is(err, ErrUnknownSubheader) {
		t.Fatalf("got %v", err)
	}
	if _, _, err := NextFrame([]byte{0x32}); !errors.Is(err, ErrIncomplete) {
		t.Fatalf("got %v", err)
	}
}

// START is 1728 bytes: the header, its size, subheader 0 and the owner, then forty cells
// of 43 bytes, each where its index puts it.
func TestAShopOpensWithItsCells(t *testing.T) {
	var cells [ShopCells]ShopItem
	cells[0] = ShopItem{VNum: 27001, Price: 32, Count: 1}
	cells[5] = ShopItem{VNum: 27001, Price: 640, Count: 20}

	got := EncodeShopStart(900001, cells)

	head := mustHex(t, "26"+"c006"+"00"+"a1bb0d00")
	entry := func(vnum, price uint32, count uint8) []byte {
		e := binary.LittleEndian.AppendUint32(nil, vnum)
		e = binary.LittleEndian.AppendUint32(e, price)
		return append(append(e, count), make([]byte, 34)...)
	}
	if len(got) != 1728 || !bytes.Equal(got[:8], head) || !bytes.Equal(got[8:8+43], entry(27001, 32, 1)) ||
		!bytes.Equal(got[8+5*43:8+6*43], entry(27001, 640, 20)) || !bytes.Equal(got[8+43:8+2*43], make([]byte, 43)) {
		t.Fatalf("got %d bytes: %x", len(got), got[:8+6*43])
	}
}

func TestAShopRefusalIsItsReason(t *testing.T) {
	if got, want := EncodeShopError(ShopNotEnoughYang), mustHex(t, "26"+"0400"+"05"); !bytes.Equal(got, want) {
		t.Fatalf("got %x, want %x", got, want)
	}
}

// ON_CLICK names the character clicked.
func TestAClickNamesTheCharacter(t *testing.T) {
	if vid, err := DecodeOnClick(mustHex(t, "a1bb0d00")); err != nil || vid != 900001 {
		t.Fatalf("got %d, %v", vid, err)
	}
}

// The taskbar's packets each way: a slot filled with inventory cell 2, emptied, and swapped.
func TestTheTaskbarsPacketsAreReadAndWritten(t *testing.T) {
	if q, err := DecodeQuickslotAdd([]byte{3, 1, 2}); err != nil || q != (QuickslotAdd{Slot: 3, Type: 1, Position: 2}) {
		t.Fatalf("add %+v, %v", q, err)
	}
	if slot, err := DecodeQuickslotDel([]byte{3}); err != nil || slot != 3 {
		t.Fatalf("del %d, %v", slot, err)
	}
	if a, b, err := DecodeQuickslotSwap([]byte{3, 9}); err != nil || a != 3 || b != 9 {
		t.Fatalf("swap %d %d, %v", a, b, err)
	}
	for got, want := range map[string]string{
		string(EncodeQuickslotAdd(QuickslotAdd{Slot: 3, Type: 1, Position: 2})): "1c030102",
		string(EncodeQuickslotDel(3)):                                           "1d03",
		string(EncodeQuickslotSwap(3, 9)):                                       "1e0309",
	} {
		if got != string(mustHex(t, want)) {
			t.Errorf("got %x, want %s", got, want)
		}
	}
}

// ITEM_DROP2 drops part of a stack from a bag cell, or yang from the reserved window 0.
func TestADropNamesAnItemOrYang(t *testing.T) {
	if d, err := DecodeItemDrop2(mustHex(t, "01"+"0700"+"00000000"+"03")); err != nil ||
		d != (ItemDrop{Pos: ItemPos{Window: 1, Cell: 7}, Count: 3}) {
		t.Fatalf("item: %+v, %v", d, err)
	}
	if d, err := DecodeItemDrop2(mustHex(t, "00"+"0000"+"05000000"+"00")); err != nil || d != (ItemDrop{Gold: 5}) {
		t.Fatalf("yang: %+v, %v", d, err)
	}
}

// USE_SKILL names the skill and its target, then the sequence byte.
func TestASkillUsedNamesItAndItsTarget(t *testing.T) {
	frame, used, err := NextFrame(mustHex(t, "34"+"3e000000"+"00000000"+"af"))
	if err != nil || used != 10 {
		t.Fatalf("framed %d, %v", used, err)
	}
	if u, err := DecodeUseSkill(frame.Body); err != nil || u != (UseSkill{VNum: 62}) {
		t.Fatalf("got %+v, %v", u, err)
	}
}

// The effects on a character go as two words, in ADD and UPDATE alike: bit 19 is the low
// word's.
func TestACharactersEffectsGoInItsAddAndUpdate(t *testing.T) {
	add := CharacterAdd{VID: 7}
	if got := EncodeCharacterAdd(add, 1<<19)[27:35]; !bytes.Equal(got, mustHex(t, "00000800"+"00000000")) {
		t.Fatalf("ADD effects %x", got)
	}
	if got := EncodeCharacterUpdate(CharacterInfo{VID: 7}, add, 1<<19)[16:24]; !bytes.Equal(got, mustHex(t, "00000800"+"00000000")) {
		t.Fatalf("UPDATE effects %x", got)
	}
}

// GIVE_ITEM names the NPC, the item's place and its count, then the sequence byte.
func TestAGiveNamesTheNPCAndTheItem(t *testing.T) {
	frame, used, err := NextFrame(mustHex(t, "53"+"07000000"+"01"+"0c00"+"01"+"af"))
	if err != nil || used != 10 {
		t.Fatalf("framed %d, %v", used, err)
	}
	if g, err := DecodeGiveItem(frame.Body); err != nil || g != (GiveItem{To: 7, Pos: ItemPos{Window: 1, Cell: 12}, Count: 1}) {
		t.Fatalf("got %+v, %v", g, err)
	}
}

// REFINE names the bag cell and the dialog's type, then the sequence byte; closing the
// dialog sends 255 for both.
func TestARefineNamesTheCellAndType(t *testing.T) {
	frame, used, err := NextFrame(mustHex(t, "60"+"0c"+"05"+"af"))
	if err != nil || used != 4 {
		t.Fatalf("framed %d, %v", used, err)
	}
	if q, err := DecodeRefine(frame.Body); err != nil || q != (Refine{Cell: 12, Type: RefineNoQuestion}) {
		t.Fatalf("got %+v, %v", q, err)
	}
	if q, _ := DecodeRefine(mustHex(t, "ffff")); q != (Refine{Cell: RefineCancel, Type: RefineCancel}) {
		t.Fatalf("closed: %+v", q)
	}
}

// REFINE_INFORMATION_NEW is 60 bytes: type, cell, the kinds from and into, the count of
// materials, cost, chance, and five places of material, the unused ones empty.
func TestTheRefineDialogIsSixtyBytes(t *testing.T) {
	got := EncodeRefineInfo(RefineInfo{Type: RefineMayDowngrade, Cell: 12, From: 14, Into: 15, Cost: 3000, Chance: 60,
		Materials: []Material{{VNum: 30028, Count: 1}}})
	want := mustHex(t, "77"+"02"+"0c"+"0e000000"+"0f000000"+"01"+"b80b0000"+"3c000000"+
		"4c750000"+"01000000"+strings.Repeat("00", 32))
	if !bytes.Equal(got, want) {
		t.Fatalf("got  %x\nwant %x", got, want)
	}
}

// FLY_TARGETING and ADD_FLY_TARGETING name the arrow's target and its point, and SHOOT the
// skill letting it go, 0 for a plain shot, each then the sequence byte.
func TestAnArrowsTargetAndItsShotAreRead(t *testing.T) {
	for _, header := range []string{"33", "35"} {
		frame, used, err := NextFrame(mustHex(t, header+"07000000"+"fbffffff"+"09000000"+"af"))
		if err != nil || used != 14 {
			t.Fatalf("0x%s framed %d, %v", header, used, err)
		}
		if f, err := DecodeFlyTarget(frame.Body); err != nil || f != (FlyTarget{Target: 7, X: -5, Y: 9}) {
			t.Fatalf("0x%s got %+v, %v", header, f, err)
		}
	}
	frame, used, err := NextFrame(mustHex(t, "36"+"2e"+"af"))
	if err != nil || used != 3 {
		t.Fatalf("shoot framed %d, %v", used, err)
	}
	if sk, err := DecodeShoot(frame.Body); err != nil || sk != 46 {
		t.Fatalf("shoot got %d, %v", sk, err)
	}
}

// What another's arrow flies at goes as the shooter, the target and the point.
func TestAnArrowsTargetIsShownWithItsShooter(t *testing.T) {
	got := EncodeFlyTarget(ServerFlyTarget, 1000, FlyTarget{Target: 7, X: -5, Y: 9})
	if want := mustHex(t, "47"+"e8030000"+"07000000"+"fbffffff"+"09000000"); !bytes.Equal(got, want) {
		t.Fatalf("got  %x\nwant %x", got, want)
	}
	if got := EncodeFlyTarget(ServerAddFlyTarget, 1000, FlyTarget{}); got[0] != 0x45 || len(got) != 17 {
		t.Fatalf("add got %x", got)
	}
}

// SYNC_POSITION carries its length, then each character's vid and where the server has it.
func TestASyncedPositionIsSent(t *testing.T) {
	got := EncodeSyncPosition(1000, 963400, -5)
	if want := mustHex(t, "05"+"0f00"+"e8030000"+"48b30e00"+"fbffffff"); !bytes.Equal(got, want) {
		t.Fatalf("got  %x\nwant %x", got, want)
	}
}
