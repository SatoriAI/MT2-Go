package protocol

// Empire is the kingdom an account belongs to; 0 means not chosen yet.
type Empire uint8

const (
	EmpireNone   Empire = 0
	EmpireRed    Empire = 1 // Shinsoo
	EmpireYellow Empire = 2 // Chunjo
	EmpireBlue   Empire = 3 // Jinno
)

func (e Empire) Valid() bool { return e >= EmpireRed && e <= EmpireBlue }
