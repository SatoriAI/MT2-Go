package protocol

import (
	"bytes"
	"errors"
)

// Packets the client sends, decoded from a Frame body (the bytes after the header).

// Field sizes the client uses: names are 30 characters plus a NUL, passwords 16 plus a
// NUL. The 16-byte client key is ignored because packet encryption stays off.
const (
	accountNameSize = 31
	passwordSize    = 17
	clientKeySize   = 16
)

// Handshake is the time-sync packet; both sides send the same layout.
type Handshake struct {
	ID    uint32
	Time  uint32
	Delta int32
}

func DecodeHandshake(body []byte) (Handshake, error) {
	r := NewReader(body)
	h := Handshake{ID: r.U32(), Time: r.U32(), Delta: r.I32()}
	return h, r.Err()
}

// Login is LOGIN3: name and password, sent to the auth port.
type Login struct {
	Name     string
	Password string
}

func DecodeLogin(body []byte) (Login, error) {
	r := NewReader(body)
	l := Login{Name: r.String(accountNameSize), Password: r.String(passwordSize)}
	r.Skip(clientKeySize)
	return l, r.Err()
}

// LoginByKey is LOGIN2: the key the auth port handed out, presented on the game port.
type LoginByKey struct {
	Name string
	Key  uint32
}

func DecodeLoginByKey(body []byte) (LoginByKey, error) {
	r := NewReader(body)
	l := LoginByKey{Name: r.String(accountNameSize), Key: r.U32()}
	r.Skip(clientKeySize)
	return l, r.Err()
}

// CreateCharacter is what the create screen sends. The client also sends four extra stat
// points, which it always leaves at 0; the server decides a new character's stats, so
// they are skipped.
type CreateCharacter struct {
	Slot  uint8
	Name  string
	Race  uint16
	Shape uint8
}

func DecodeCreateCharacter(body []byte) (CreateCharacter, error) {
	r := NewReader(body)
	c := CreateCharacter{Slot: r.U8(), Name: r.String(characterNameSize), Race: r.U16(), Shape: r.U8()}
	r.Skip(4)
	return c, r.Err()
}

// DecodeSelectCharacter reads the slot of the character the player pressed Start on.
func DecodeSelectCharacter(body []byte) (uint8, error) {
	r := NewReader(body)
	slot := r.U8()
	return slot, r.Err()
}

// Move is the player's character starting, changing or ending a movement.
type Move struct {
	Type     uint8 // walking, stopping, attacking and the like
	Arg      uint8
	Rotation uint8 // the direction faced, in steps of 5 degrees
	X, Y     int32
	Time     uint32 // the client's clock, as the handshake set it
}

// DecodeTarget reads the vid of the character the player picked as target, 0 for none.
func DecodeTarget(body []byte) (uint32, error) {
	r := NewReader(body)
	vid := r.U32()
	return vid, r.Err()
}

// Attack is a blow the attacker's client saw land. Skill is 0 for a plain or combo blow;
// the two bytes after the victim are a check of the client's files, which is not read.
type Attack struct {
	Skill  uint8
	Victim uint32
}

func DecodeAttack(body []byte) (Attack, error) {
	r := NewReader(body)
	a := Attack{Skill: r.U8(), Victim: r.U32()}
	r.Skip(2)
	return a, r.Err()
}

// MoveWait is the move a server sends to walk a character to a point, where it stops
// (the client's FUNC_WAIT).
const MoveWait uint8 = 0

// MoveAttack plays the character's normal attack where the move puts it, facing its
// rotation (the client's FUNC_ATTACK); its arg is ignored.
const MoveAttack uint8 = 2

func DecodeMove(body []byte) (Move, error) {
	r := NewReader(body)
	m := Move{Type: r.U8(), Arg: r.U8(), Rotation: r.U8(), X: r.I32(), Y: r.I32(), Time: r.U32()}
	return m, r.Err()
}

// ChatType is how a line is meant to be heard. The numbers are open-mt2's; only the ones
// the server uses are named.
type ChatType uint8

const (
	ChatNormal  ChatType = 0
	ChatInfo    ChatType = 1 // from vid 0, a line from the game in chat, in its own colour
	ChatNotice  ChatType = 2 // from vid 0, scrolls over the top of the screen and stays in chat
	ChatCommand ChatType = 5 // an instruction to the client itself, such as "quit"
)

// ErrNoTerminator means a text field has no NUL before the packet ends.
var ErrNoTerminator = errors.New("protocol: text without its terminator")

// Chat is a line the player typed, or a command a button sent as one. The text stays the
// client's bytes: it is in the client's single-byte code page, and decoding it as UTF-8
// or ASCII would garble letters such as ą.
type Chat struct {
	Type ChatType
	Text string
}

func DecodeChat(body []byte) (Chat, error) {
	r := NewReader(body)
	t := ChatType(r.U8())
	if err := r.Err(); err != nil {
		return Chat{}, err
	}
	text, err := terminated(body[1:])
	return Chat{Type: t, Text: text}, err
}

// Whisper is a line for one player, by name. The text stays the client's bytes, as in
// Chat.
type Whisper struct {
	To   string
	Text string
}

// DecodeWhisper reads the name from the first 24 bytes of its field only: the client
// copies at most 24 and leaves the 25th as whatever its memory held.
func DecodeWhisper(body []byte) (Whisper, error) {
	r := NewReader(body)
	to := r.String(characterNameSize - 1)
	r.Skip(1)
	if err := r.Err(); err != nil {
		return Whisper{}, err
	}
	text, err := terminated(body[characterNameSize:])
	return Whisper{To: to, Text: text}, err
}

// terminated reads text that runs to a NUL, as the client ends a line it typed.
func terminated(b []byte) (string, error) {
	text, _, found := bytes.Cut(b, []byte{0})
	if !found {
		return "", ErrNoTerminator
	}
	return string(text), nil
}

// Version is the build the client reports while it loads the world.
type Version struct {
	Name      string
	Timestamp string
}

func DecodeVersion(body []byte) (Version, error) {
	r := NewReader(body)
	v := Version{Name: r.String(33), Timestamp: r.String(33)}
	return v, r.Err()
}

// DecodeEmpire reads the kingdom a new account picked: 1 Shinsoo, 2 Chunjo, 3 Jinno.
func DecodeEmpire(body []byte) (uint8, error) {
	r := NewReader(body)
	e := r.U8()
	return e, r.Err()
}

// ItemPos is a cell in one of the client's item windows.
type ItemPos struct {
	Window uint8
	Cell   uint16
}

func (r *Reader) itemPos() ItemPos { return ItemPos{Window: r.U8(), Cell: r.U16()} }

// ItemMove is an item dragged from one cell to another; Count is how many of a stack go,
// all of it unless the player split it.
type ItemMove struct {
	From, To ItemPos
	Count    uint8
}

func DecodeItemMove(body []byte) (ItemMove, error) {
	r := NewReader(body)
	m := ItemMove{From: r.itemPos(), To: r.itemPos(), Count: r.U8()}
	return m, r.Err()
}

// DecodeItemUse reads the position of the item double-clicked.
func DecodeItemUse(body []byte) (ItemPos, error) {
	r := NewReader(body)
	at := r.itemPos()
	return at, r.Err()
}
