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
	StatPoints          uint32
	AttackSpeed         uint32
	MoveSpeed           uint32
	Defence             uint32 // the character window's DEF
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
		PointGold: p.Gold, PointST: p.ST, PointHT: p.HT, PointDX: p.DX, PointIQ: p.IQ,
		17: p.AttackSpeed, 19: p.MoveSpeed, PointDefence: p.Defence, PointStatPoints: p.StatPoints,
	} {
		points[index] = v
	}
	w := NewWriter(byte(ServerPoints), 1+4*pointCount)
	for _, v := range points {
		w.U32(v)
	}
	return w.Bytes()
}

// Point is an index in the client's points array, which POINT_CHANGE names one by one.
type Point = int

const (
	PointLevel      Point = 1
	PointExp        Point = 3
	PointNextExp    Point = 4
	PointHP         Point = 5
	PointMaxHP      Point = 6
	PointSP         Point = 7
	PointMaxSP      Point = 8
	PointGold       Point = 11 // yang
	PointST         Point = 12
	PointHT         Point = 13
	PointDX         Point = 14
	PointIQ         Point = 15
	PointDefence    Point = 20 // DEF_GRADE, the character window's DEF
	PointStatPoints Point = 26 // earned and not yet spent; the character window's + buttons show while above 0
)

// EncodePointChange sets one of a character's points to value, amount being how much it
// changed. For the player's own vid the client takes the value into its points; for
// another's it takes only the level, shown over their head. A change of the level plays
// the level-up effect. The header is a 4-byte int in the client's struct.
func EncodePointChange(vid uint32, point Point, amount, value int32) []byte {
	w := NewWriter(byte(ServerPointChange), 17)
	w.Raw(make([]byte, 3))
	w.U32(vid)
	w.U8(uint8(point)) //nolint:gosec // the client's indices are under 255
	w.I32(amount)
	w.I32(value)
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

// The kinds of character the client builds differently (its ActorInstance.h).
const (
	EntityMonster uint8 = 0
	EntityNPC     uint8 = 1
	EntityPlayer  uint8 = 6
)

// CharacterAdd is a character appearing in the client's view, the player included.
type CharacterAdd struct {
	VID         uint32
	Rotation    float32 // degrees
	X, Y        int32
	Type        uint8
	Race        uint16
	MoveSpeed   uint8
	AttackSpeed uint8
	Dead        bool // shown lying, as a body: the client's ADD_CHARACTER_STATE_DEAD
}

func state(dead bool) uint8 {
	if dead {
		return 1
	}
	return 0
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
	w.U8(state(c.Dead))
	w.Raw(make([]byte, 8)) // effects
	return w.Bytes()
}

// CharacterInfo is what the client shows of a character beyond its model: its name, the
// parts it wears and its level.
type CharacterInfo struct {
	VID        uint32
	Name       string
	BodyPart   uint16 // the armour worn, by its vnum, or the class's own look, 0 or 1
	WeaponPart uint16 // the weapon held, by its vnum, or 0
	HairPart   uint16
	Empire     uint8
	Level      uint32
}

// EncodeCharacterInfo sends the parts as body, weapon, head and hair; the head stays 0
// until helmets are worn. Guild, rank, player-kill mode and mount are 0.
func EncodeCharacterInfo(c CharacterInfo) []byte {
	w := NewWriter(byte(ServerCharacterInfo), 54)
	w.U32(c.VID)
	w.String(c.Name, characterNameSize)
	w.U16(c.BodyPart)
	w.U16(c.WeaponPart)
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

// EncodeMove passes another character's move on, as its own client sent it. The client
// ignores a move of the player's own character. The duration, which the client uses
// only for characters it has not drawn yet, is left 0.
func EncodeMove(vid uint32, m Move) []byte {
	w := NewWriter(byte(ServerMove), 24)
	w.U8(m.Type)
	w.U8(m.Arg)
	w.U8(m.Rotation)
	w.U32(vid)
	w.I32(m.X)
	w.I32(m.Y)
	w.U32(m.Time)
	w.U32(0) // duration
	return w.Bytes()
}

// EncodeTarget tells the player how many of their target's hit points are left, as a
// percentage; the client draws its target window's bar from it.
func EncodeTarget(vid uint32, hpPercent uint8) []byte {
	w := NewWriter(byte(ServerTarget), 6)
	w.U32(vid)
	w.U8(hpPercent)
	return w.Bytes()
}

// DamageNormal is a plain blow, drawn as a number over the victim.
const DamageNormal uint8 = 1

// EncodeDamage tells the attacker how much a blow took. The client draws it only over its
// own target or the player.
func EncodeDamage(vid uint32, flags uint8, damage int32) []byte {
	w := NewWriter(byte(ServerDamage), 10)
	w.U32(vid)
	w.U8(flags)
	w.I32(damage)
	return w.Bytes()
}

// EncodeDead tells the client the character died; it plays the death and keeps the body
// until the character is deleted.
func EncodeDead(vid uint32) []byte {
	w := NewWriter(byte(ServerDead), 5)
	w.U32(vid)
	return w.Bytes()
}

func EncodeCharacterDelete(vid uint32) []byte {
	w := NewWriter(byte(ServerCharacterDelete), 5)
	w.U32(vid)
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

// WhisperType is what a whisper packet carries. The numbers are the client's; only the
// ones the server sends are named.
type WhisperType uint8

const (
	WhisperLine          WhisperType = 0 // a line from the named player
	WhisperNotOnline     WhisperType = 1 // the named player is not in the game
	WhisperTargetBlocked WhisperType = 2 // the named player blocks whispers
	WhisperSenderBlocked WhisperType = 3 // the sender blocks whispers, so may not send them
	WhisperError         WhisperType = 4 // the text is shown as it is
)

// EncodeWhisper sends a whisper about the named player: their line, or what became of a
// line to them.
func EncodeWhisper(t WhisperType, name, text string) []byte {
	size := 1 + 2 + 1 + characterNameSize + len(text) + 1
	w := NewWriter(byte(ServerWhisper), size)
	w.U16(uint16(size)) //nolint:gosec // a line read in is at most maxSized (1024) bytes
	w.U8(uint8(t))
	w.String(name, characterNameSize)
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

// EncodeItemSet puts count of item vnum in the cell, or empties it with vnum 0. The
// sockets and bonuses that follow are not used yet, so they are sent empty.
func EncodeItemSet(at ItemPos, vnum uint32, count uint8) []byte {
	w := NewWriter(byte(ServerItemSet), 42)
	w.U8(at.Window)
	w.U16(at.Cell)
	w.U32(vnum)
	w.U8(count)
	for range 3 { // sockets
		w.I32(0)
	}
	for range 7 { // bonuses: a type and a value
		w.U8(0)
		w.U16(0)
	}
	return w.Bytes()
}

// EncodeCharacterUpdate re-dresses a character the client already shows: its armour,
// weapon and hair, as CHARACTER_INFO names them, and its speeds. The rest, state, affects,
// guild, alignment, player-kill mode and mount, are 0 for now.
func EncodeCharacterUpdate(c CharacterInfo, moveSpeed, attackSpeed uint8) []byte {
	w := NewWriter(byte(ServerCharacterUpdate), 35)
	w.U32(c.VID)
	w.U16(c.BodyPart)
	w.U16(c.WeaponPart)
	w.U16(0) // head
	w.U16(c.HairPart)
	w.U8(moveSpeed)
	w.U8(attackSpeed)
	w.U8(0)  // state
	w.U32(0) // affects
	w.U32(0)
	w.U32(0) // guild
	w.U16(0) // alignment
	w.U8(0)  // player-kill mode
	w.U32(0) // mount
	return w.Bytes()
}

// EncodeGroundAdd shows an item of kind vnum lying at (x, y); the client sets it on the
// terrain itself, so the height is sent as 0.
func EncodeGroundAdd(vid uint32, x, y int32, vnum uint32) []byte {
	w := NewWriter(byte(ServerGroundAdd), 21)
	w.I32(x)
	w.I32(y)
	w.I32(0) // height
	w.U32(vid)
	w.U32(vnum)
	return w.Bytes()
}

// EncodeGroundDel takes an item off the ground.
func EncodeGroundDel(vid uint32) []byte {
	w := NewWriter(byte(ServerGroundDel), 5)
	w.U32(vid)
	return w.Bytes()
}

// EncodeItemOwner names who alone may pick the item on the ground up; "" frees it for
// anyone. The client shows the name over the item and refuses everyone else by itself.
func EncodeItemOwner(vid uint32, name string) []byte {
	w := NewWriter(byte(ServerItemOwner), 30)
	w.U32(vid)
	w.String(name, characterNameSize)
	return w.Bytes()
}

// Effect is one of the effects the client draws on a character.
type Effect uint8

// The potions' effects; the client plays the potion sound with either on the player's own
// character.
const (
	EffectRedPotion  Effect = 1
	EffectBluePotion Effect = 2
)

// EncodeSpecialEffect draws the effect on the character vid.
func EncodeSpecialEffect(e Effect, vid uint32) []byte {
	w := NewWriter(byte(ServerSpecialEffect), 6)
	w.U8(uint8(e))
	w.U32(vid)
	return w.Bytes()
}
