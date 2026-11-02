package protocol

// Header numbers are reused across directions (0x5A is the empire packet both ways, 0xFD
// is the phase going out and the client version coming in), so the two directions have
// separate types and cannot be mixed up.

// ClientHeader is the first byte of a packet the client sends.
type ClientHeader byte

const (
	ClientCreateCharacter ClientHeader = 0x04
	ClientSelectCharacter ClientHeader = 0x06
	ClientMove            ClientHeader = 0x07
	ClientEnterGame       ClientHeader = 0x0A
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
	ServerPoints           ServerHeader = 0x10
	ServerTime             ServerHeader = 0x6A
	ServerMainCharacter    ServerHeader = 0x71 // who the player is and where
	ServerChannel          ServerHeader = 0x79
	ServerCharacterInfo    ServerHeader = 0x88 // a character's name and parts
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
	PhaseHandshake Phase = 1
	PhaseLogin     Phase = 2
	PhaseSelect    Phase = 3
	PhaseLoading   Phase = 4
	PhaseGame      Phase = 5
	PhaseAuth      Phase = 10
)
