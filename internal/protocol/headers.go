package protocol

// Header numbers are reused across directions (0x5A is the empire packet both ways, 0xFD
// is the phase going out and the client version coming in), so the two directions have
// separate types and cannot be mixed up.

// ClientHeader is the first byte of a packet the client sends.
type ClientHeader byte

const (
	ClientAttack          ClientHeader = 0x02 // a blow landing on a character, as the attacker's client saw it
	ClientChat            ClientHeader = 0x03
	ClientCreateCharacter ClientHeader = 0x04
	ClientSelectCharacter ClientHeader = 0x06
	ClientMove            ClientHeader = 0x07
	ClientEnterGame       ClientHeader = 0x0A
	ClientItemUse         ClientHeader = 0x0B // a double-click on an item
	ClientItemDrop        ClientHeader = 0x0C
	ClientItemMove        ClientHeader = 0x0D // an item dragged to another cell
	ClientQuickslotAdd    ClientHeader = 0x10 // an item or skill dragged to the taskbar
	ClientQuickslotDel    ClientHeader = 0x11
	ClientQuickslotSwap   ClientHeader = 0x12
	ClientItemDrop2       ClientHeader = 0x14 // an item dragged out onto the ground
	ClientExchange        ClientHeader = 0x1B // a trade with another player, from dragging an item onto them
	ClientItemUseToItem   ClientHeader = 0x3C // an item dropped onto another
	ClientGiveItem        ClientHeader = 0x53 // an item dragged onto an NPC
	ClientWhisper         ClientHeader = 0x13
	ClientOnClick         ClientHeader = 0x1A // a click on a character, for talking to it
	ClientTarget          ClientHeader = 0x3D // a character picked as target, or 0 for none
	ClientMarkLogin       ClientHeader = 0x64
	ClientEmpire          ClientHeader = 0x5A
	ClientLoginByKey      ClientHeader = 0x6D // LOGIN2, on the game port
	ClientLogin           ClientHeader = 0x6F // LOGIN3, on the auth port
	ClientStateChecker    ClientHeader = 0xCE
	ClientVersion         ClientHeader = 0xF1
	ClientHandshake       ClientHeader = 0xFF
)

// ServerHeader is the first byte of a packet the server sends.
type ServerHeader byte

const (
	ServerLoginFailure     ServerHeader = 0x07
	ServerCharacterCreated ServerHeader = 0x08
	ServerCreateFailure    ServerHeader = 0x09
	ServerCharacterAdd     ServerHeader = 0x01 // a character appears in view
	ServerCharacterDelete  ServerHeader = 0x02 // a character leaves view
	ServerMove             ServerHeader = 0x03 // another character moves
	ServerChat             ServerHeader = 0x04
	ServerPoints           ServerHeader = 0x10
	ServerPointChange      ServerHeader = 0x11 // one of a character's points changes
	ServerItemSet          ServerHeader = 0x14 // an item in a cell, or vnum 0 for none
	ServerWhisper          ServerHeader = 0x22
	ServerTime             ServerHeader = 0x6A
	ServerMainCharacter    ServerHeader = 0x71 // who the player is and where
	ServerChannel          ServerHeader = 0x79
	ServerCharacterInfo    ServerHeader = 0x88 // a character's name and parts
	ServerDead             ServerHeader = 0x0E // a character dies
	ServerTarget           ServerHeader = 0x3F // the target's hit points, as a percentage
	ServerDamage           ServerHeader = 0x87 // damage dealt, drawn over the victim
	ServerSkillLevels      ServerHeader = 0x4C
	ServerCharacterList    ServerHeader = 0x20 // LOGIN_SUCCESS4: four character slots
	ServerEmpire           ServerHeader = 0x5A
	ServerAuthSuccess      ServerHeader = 0x96
	ServerChannelStatus    ServerHeader = 0xD2
	ServerPhase            ServerHeader = 0xFD
	ServerHandshake        ServerHeader = 0xFF
)

// Phase is the connection stage the server announces; the client switches screens and
// packet handlers on it.
type Phase byte

const (
	PhaseClose     Phase = 0 // the connection is ending; the client stops sending
	PhaseHandshake Phase = 1
	PhaseLogin     Phase = 2
	PhaseSelect    Phase = 3
	PhaseLoading   Phase = 4
	PhaseGame      Phase = 5
	PhaseAuth      Phase = 10
)
