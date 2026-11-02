package protocol

import (
	"errors"
	"fmt"
)

// frameSpec is how the client frames one header: its fixed size including the header
// byte, and whether a sequence byte follows it.
type frameSpec struct {
	size      int
	sequenced bool
}

// The client turns sequencing on for its game and account streams, then appends one
// byte from a fixed table to most packets. The handshake reply, the login screen's state
// checker and the guild mark login are sent without it; counting a byte that never comes
// stalls the connection. The value itself is not checked yet (docs/mechanics).
var clientFrames = map[ClientHeader]frameSpec{
	ClientHandshake:       {size: 13},
	ClientStateChecker:    {size: 1},
	ClientMarkLogin:       {size: 9},
	ClientLogin:           {size: 65, sequenced: true},
	ClientLoginByKey:      {size: 52, sequenced: true},
	ClientEmpire:          {size: 2, sequenced: true},
	ClientCreateCharacter: {size: 34, sequenced: true},
	ClientSelectCharacter: {size: 2, sequenced: true},
	ClientEnterGame:       {size: 1, sequenced: true},
	ClientMove:            {size: 16, sequenced: true},
	ClientVersion:         {size: 67, sequenced: true},
}

// ErrIncomplete means the buffer holds only part of the next packet; read more first.
var ErrIncomplete = errors.New("protocol: incomplete packet")

// UnknownHeaderError is a header this server does not frame yet. Nothing after it can
// be delimited, so the connection has to end.
type UnknownHeaderError struct{ Header byte }

func (e UnknownHeaderError) Error() string {
	return fmt.Sprintf("protocol: unknown client header 0x%02X", e.Header)
}

// Frame is one client packet: its header and the bytes after the header, without the
// sequence byte.
type Frame struct {
	Header ClientHeader
	Body   []byte
}

// NextFrame cuts the first complete packet off buf and reports how many bytes it used.
func NextFrame(buf []byte) (Frame, int, error) {
	if len(buf) == 0 {
		return Frame{}, 0, ErrIncomplete
	}
	header := ClientHeader(buf[0])
	spec, ok := clientFrames[header]
	if !ok {
		return Frame{}, 0, UnknownHeaderError{Header: buf[0]}
	}
	used := spec.size
	if spec.sequenced {
		used++
	}
	if len(buf) < used {
		return Frame{}, 0, ErrIncomplete
	}
	return Frame{Header: header, Body: buf[1:spec.size]}, used, nil
}
