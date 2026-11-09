package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// frameSpec is how the client frames one header: its fixed size including the header
// byte, or, when sized, a u16 total size right after the header, or a size for each
// subheader, the byte after the header; and whether a sequence byte follows it.
type frameSpec struct {
	size      int
	sized     bool
	bySub     map[byte]int
	sequenced bool
}

// A sized packet's length counts its header, the size field and the body, but not the
// sequence byte. The bound is far above what the client's chat box allows (512 bytes),
// and keeps a lying length from making the server buffer without end.
const (
	minSized = 3
	maxSized = 1024
)

// ErrBadSize means a sized packet declared a length no packet can have.
var ErrBadSize = errors.New("protocol: impossible packet size")

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
	ClientAttack:          {size: 8, sequenced: true},
	ClientOnClick:         {size: 5, sequenced: true},
	ClientTarget:          {size: 5, sequenced: true},
	ClientMove:            {size: 16, sequenced: true},
	ClientChat:            {sized: true, sequenced: true},
	ClientWhisper:         {sized: true, sequenced: true},
	ClientVersion:         {size: 67, sequenced: true},
	ClientItemUse:         {size: 4, sequenced: true},
	ClientItemDrop:        {size: 8, sequenced: true},
	ClientItemMove:        {size: 8, sequenced: true},
	ClientQuickslotAdd:    {size: 4, sequenced: true},
	ClientQuickslotDel:    {size: 2, sequenced: true},
	ClientQuickslotSwap:   {size: 3, sequenced: true},
	ClientItemDrop2:       {size: 9, sequenced: true},
	ClientExchange:        {size: 10, sequenced: true},
	ClientItemUseToItem:   {size: 7, sequenced: true},
	ClientGiveItem:        {size: 9, sequenced: true},
	ClientRefine:          {size: 3, sequenced: true},
	ClientItemPickup:      {size: 5, sequenced: true},
	ClientScriptAnswer:    {size: 2, sequenced: true},
	ClientUseSkill:        {size: 9, sequenced: true},
	ClientFlyTarget:       {size: 13, sequenced: true},
	ClientAddFlyTarget:    {size: 13, sequenced: true},
	ClientShoot:           {size: 2, sequenced: true},
	ClientShop: {bySub: map[byte]int{ShopEnd: 2, ShopBuy: 4, ShopSell: 3, ShopSell2: 4},
		sequenced: true},
}

// ErrIncomplete means the buffer holds only part of the next packet; read more first.
var ErrIncomplete = errors.New("protocol: incomplete packet")

// UnknownHeaderError is a header this server does not frame yet. Nothing after it can
// be delimited, so the connection has to end.
type UnknownHeaderError struct{ Header byte }

func (e UnknownHeaderError) Error() string {
	return fmt.Sprintf("protocol: unknown client header 0x%02X", e.Header)
}

// ErrUnknownSubheader means a packet framed by its subheader has one this server does not
// know, so, as with an unknown header, nothing after it can be delimited.
var ErrUnknownSubheader = errors.New("protocol: unknown subheader")

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
	size, bodyStart := spec.size, 1
	if spec.bySub != nil {
		if len(buf) < 2 {
			return Frame{}, 0, ErrIncomplete
		}
		if size, ok = spec.bySub[buf[1]]; !ok {
			return Frame{}, 0, fmt.Errorf("%w: 0x%02X after header 0x%02X", ErrUnknownSubheader, buf[1], buf[0])
		}
	}
	if spec.sized {
		if len(buf) < minSized {
			return Frame{}, 0, ErrIncomplete
		}
		size, bodyStart = int(binary.LittleEndian.Uint16(buf[1:])), minSized
		if size < minSized || size > maxSized {
			return Frame{}, 0, fmt.Errorf("%w: %d for header 0x%02X", ErrBadSize, size, buf[0])
		}
	}
	used := size
	if spec.sequenced {
		used++
	}
	if len(buf) < used {
		return Frame{}, 0, ErrIncomplete
	}
	return Frame{Header: header, Body: buf[bodyStart:size]}, used, nil
}
