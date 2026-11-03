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

func EncodeEmpire(e uint8) []byte {
	w := NewWriter(byte(ServerEmpire), 2)
	w.U8(e)
	return w.Bytes()
}

// CharacterSlots is how many characters an account holds.
const CharacterSlots = 4

const characterNameSize = 25 // 24 characters plus a NUL

// CharacterSummary is what the selection screen shows for one slot.
type CharacterSummary struct {
	ID          uint32 // 0 marks an empty slot
	Name        string
	Race        uint8 // class and sex in one number; internal/game/wire.go has the table
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
	for _, c := range slots {
		writeCharacter(w, c, game)
	}
	w.Raw(make([]byte, 4*4+4*13)) // guild ids and guild names
	w.U32(0)                      // guild mark handle
	w.U32(0)                      // random key
	return w.Bytes()
}

// writeCharacter writes the 63-byte record one character slot has, in the list and in
// the reply to a creation.
func writeCharacter(w *Writer, c CharacterSummary, game netip.AddrPort) {
	ip := game.Addr().As4()
	w.U32(c.ID)
	w.String(c.Name, characterNameSize)
	w.U8(c.Race)
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

// EncodeCharacterCreated tells the create screen the character exists; the client puts
// it in its slot and returns to the selection screen.
func EncodeCharacterCreated(slot uint8, c CharacterSummary, game netip.AddrPort) []byte {
	w := NewWriter(byte(ServerCharacterCreated), 65)
	w.U8(slot)
	writeCharacter(w, c, game)
	return w.Bytes()
}

// CreateFailure is why a creation was refused. The create screen shows "name taken" for
// CreateNameTaken and a general failure for anything else.
type CreateFailure uint8

const (
	CreateRefused   CreateFailure = 0
	CreateNameTaken CreateFailure = 1
)

func EncodeCreateFailure(reason CreateFailure) []byte {
	w := NewWriter(byte(ServerCreateFailure), 2)
	w.U8(uint8(reason))
	return w.Bytes()
}

// CharacterPoints are the values the client's character window and gauges show.
type CharacterPoints struct {
	Level, Exp, NextExp uint32
	HP, MaxHP           uint32
	SP, MaxSP           uint32
	Stamina, MaxStamina uint32
	Gold                uint32
	ST, HT, DX, IQ      uint32
	AttackSpeed         uint32
	MoveSpeed           uint32
}

// pointCount is how many entries the client's points array has. The client reads each
// value by its index, so the array is always sent whole.
const pointCount = 255

// EncodePoints places each value at the index the client reads it from. The indices are
// open-mt2's (PointsEnum); the rest stay 0.
func EncodePoints(p CharacterPoints) []byte {
	var points [pointCount]uint32
	for index, v := range map[int]uint32{
		1: p.Level, 3: p.Exp, 4: p.NextExp,
		5: p.HP, 6: p.MaxHP, 7: p.SP, 8: p.MaxSP, 9: p.Stamina, 10: p.MaxStamina,
		11: p.Gold, 12: p.ST, 13: p.HT, 14: p.DX, 15: p.IQ,
		17: p.AttackSpeed, 19: p.MoveSpeed,
	} {
		points[index] = v
	}
	w := NewWriter(byte(ServerPoints), 1+4*pointCount)
	for _, v := range points {
		w.U32(v)
	}
	return w.Bytes()
}

// skillCount is how many skills the client's skill-level packet covers.
const skillCount = 255

// EncodeSkillLevels sends every skill unlearnt: master type, level and next reading time
// all 0, 6 bytes per skill.
func EncodeSkillLevels() []byte {
	w := NewWriter(byte(ServerSkillLevels), 1+6*skillCount)
	w.Raw(make([]byte, 6*skillCount))
	return w.Bytes()
}

// MainCharacter is who the player is and where the world loads.
type MainCharacter struct {
	VID        uint32 // the character's id in the world, for this session
	Race       uint16
	Name       string
	X, Y       int32
	Empire     uint8
	SkillGroup uint8
}

// EncodeMainCharacter starts the loading screen: the client loads the map the coordinates
// fall in, so no map is named.
func EncodeMainCharacter(m MainCharacter) []byte {
	w := NewWriter(byte(ServerMainCharacter), 46)
	w.U32(m.VID)
	w.U16(m.Race)
	w.String(m.Name, characterNameSize)
	w.I32(m.X)
	w.I32(m.Y)
	w.I32(0) // height: the client places characters on the ground itself
	w.U8(m.Empire)
	w.U8(m.SkillGroup)
	return w.Bytes()
}

func EncodeTime(t uint32) []byte {
	w := NewWriter(byte(ServerTime), 5)
	w.U32(t)
	return w.Bytes()
}

func EncodeChannel(channel uint8) []byte {
	w := NewWriter(byte(ServerChannel), 2)
	w.U8(channel)
	return w.Bytes()
}

// EntityPlayer is the entity type of a player character (open-mt2's EntityTypeEnum).
const EntityPlayer uint8 = 6

// CharacterAdd is a character appearing in the client's view, the player included.
type CharacterAdd struct {
	VID         uint32
	Rotation    float32 // degrees
	X, Y        int32
	Type        uint8
	Race        uint16
	MoveSpeed   uint8
	AttackSpeed uint8
}

// EncodeCharacterAdd sends a character standing still with no effects on it.
func EncodeCharacterAdd(c CharacterAdd) []byte {
	w := NewWriter(byte(ServerCharacterAdd), 35)
	w.U32(c.VID)
	w.F32(c.Rotation)
	w.I32(c.X)
	w.I32(c.Y)
	w.I32(0) // height
	w.U8(c.Type)
	w.U16(c.Race)
	w.U8(c.MoveSpeed)
	w.U8(c.AttackSpeed)
	w.U8(0)                // state
	w.Raw(make([]byte, 8)) // effects
	return w.Bytes()
}

// CharacterInfo is what the client shows of a character beyond its model: its name, the
// parts it wears and its level.
type CharacterInfo struct {
	VID      uint32
	Name     string
	BodyPart uint16
	HairPart uint16
	Empire   uint8
	Level    uint32
}

// EncodeCharacterInfo sends the parts as body, weapon, head and hair; the weapon and head
// stay 0 until characters carry items. Guild, rank, player-kill mode and mount are 0.
func EncodeCharacterInfo(c CharacterInfo) []byte {
	w := NewWriter(byte(ServerCharacterInfo), 54)
	w.U32(c.VID)
	w.String(c.Name, characterNameSize)
	w.U16(c.BodyPart)
	w.U16(0) // weapon
	w.U16(0) // head
	w.U16(c.HairPart)
	w.U8(c.Empire)
	w.U32(0) // guild
	w.U32(c.Level)
	w.U16(0) // rank
	w.U8(0)  // player-kill mode
	w.U32(0) // mount
	return w.Bytes()
}

// EncodeChat sends a line to the client's chat. vid is whose line it is: the client drops
// a line whose vid is a character it does not know, so a line from no one carries 0.
func EncodeChat(t ChatType, vid uint32, empire uint8, text string) []byte {
	size := 1 + 2 + 1 + 4 + 1 + len(text) + 1
	w := NewWriter(byte(ServerChat), size)
	w.U16(uint16(size)) //nolint:gosec // a line read in is at most maxSized (1024) bytes, plus a name
	w.U8(uint8(t))
	w.U32(vid)
	w.U8(empire)
	w.Raw([]byte(text))
	w.U8(0)
	return w.Bytes()
}

// ChannelStatus is one channel the login screen lists.
type ChannelStatus struct {
	Port   uint16
	Status uint8
}

// ChannelNormal is the "NORM" state; 3 ("FULL") would stop the client connecting.
const ChannelNormal uint8 = 1

// EncodeChannelStatus ends with a success byte, as the original server's reply does; the
// client reads past the entries and discards it.
func EncodeChannelStatus(channels []ChannelStatus) []byte {
	w := NewWriter(byte(ServerChannelStatus), 6+3*len(channels))
	w.I32(int32(len(channels))) //nolint:gosec // one entry per channel this server runs
	for _, c := range channels {
		w.U16(c.Port)
		w.U8(c.Status)
	}
	w.U8(1)
	return w.Bytes()
}
