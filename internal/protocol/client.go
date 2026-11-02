package protocol

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

// DecodeEmpire reads the kingdom a new account picked: 1 Shinsoo, 2 Chunjo, 3 Jinno.
func DecodeEmpire(body []byte) (uint8, error) {
	r := NewReader(body)
	e := r.U8()
	return e, r.Err()
}
