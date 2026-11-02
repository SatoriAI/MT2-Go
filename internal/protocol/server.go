package protocol

import "net/netip"

// Packets the server sends, encoded to the exact bytes the client reads. The client
// knows each header's size from its own table, so a server packet carries no length.

func EncodePhase(p Phase) []byte {
	w := NewWriter(byte(ServerPhase), 2)
	w.U8(byte(p))
	return w.Bytes()
}

func EncodeHandshake(h Handshake) []byte {
	w := NewWriter(byte(ServerHandshake), 13)
	w.U32(h.ID)
	w.U32(h.Time)
	w.I32(h.Delta)
	return w.Bytes()
}

// EncodeAuthSuccess hands the client the key it will present on the game port. A result
// of 0 would make the client report a duplicate key, so success is always 1.
func EncodeAuthSuccess(key uint32) []byte {
	w := NewWriter(byte(ServerAuthSuccess), 6)
	w.U32(key)
	w.U8(1)
	return w.Bytes()
}

// LoginStatus is a reason string the client's login screen knows how to show.
type LoginStatus string

const (
	LoginWrongPassword LoginStatus = "WRONGPWD"
	LoginAlreadyOnline LoginStatus = "ALREADY"
)

func EncodeLoginFailure(status LoginStatus) []byte {
	w := NewWriter(byte(ServerLoginFailure), 10)
	w.String(string(status), 9)
	return w.Bytes()
}

func EncodeEmpire(e Empire) []byte {
	w := NewWriter(byte(ServerEmpire), 2)
	w.U8(byte(e))
	return w.Bytes()
}

// CharacterSlots is how many characters an account holds.
const CharacterSlots = 4

const characterNameSize = 25 // 24 characters plus a NUL

// CharacterSummary is what the selection screen shows for one slot.
type CharacterSummary struct {
	ID          uint32 // 0 marks an empty slot
	Name        string
	Job         uint8
	Level       uint8
	PlayMinutes uint32
	ST, HT, DX  uint8
	IQ          uint8
	BodyPart    uint16
	HairPart    uint16
	X, Y        int32
	SkillGroup  uint8
}

// EncodeCharacterList sends the four slots, each with the game server address the client
// connects to when that character starts. Guilds, the mark handle and the random key are
// sent as zero until something uses them.
func EncodeCharacterList(slots [CharacterSlots]CharacterSummary, game netip.AddrPort) []byte {
	w := NewWriter(byte(ServerCharacterList), 329)
	ip := game.Addr().As4()
	for _, c := range slots {
		w.U32(c.ID)
		w.String(c.Name, characterNameSize)
		w.U8(c.Job)
		w.U8(c.Level)
		w.U32(c.PlayMinutes)
		w.U8(c.ST)
		w.U8(c.HT)
		w.U8(c.DX)
		w.U8(c.IQ)
		w.U16(c.BodyPart)
		w.U8(0) // change-name flag
		w.U16(c.HairPart)
		w.U32(0) // unused
		w.I32(c.X)
		w.I32(c.Y)
		w.Raw(ip[:]) // the client reads the address in network byte order
		w.U16(game.Port())
		w.U8(c.SkillGroup)
	}
	w.Raw(make([]byte, 4*4+4*13)) // guild ids and guild names
	w.U32(0)                      // guild mark handle
	w.U32(0)                      // random key
	return w.Bytes()
}

// ChannelStatus is one channel the login screen lists.
type ChannelStatus struct {
	Port   uint16
	Status uint8
}

// ChannelNormal is the "NORM" state; 3 ("FULL") would stop the client connecting.
const ChannelNormal uint8 = 1

func EncodeChannelStatus(channels []ChannelStatus) []byte {
	w := NewWriter(byte(ServerChannelStatus), 5+3*len(channels))
	w.I32(int32(len(channels))) //nolint:gosec // one entry per channel this server runs
	for _, c := range channels {
		w.U16(c.Port)
		w.U8(c.Status)
	}
	return w.Bytes()
}
