// Package protocol turns Metin2 packets into bytes and back. It knows nothing about
// sockets or the database: every function here is a pure translation, so each one can be
// checked against the byte layout written down in docs/mechanics/.
//
// The wire format is little-endian and unpadded, because the client declares its
// structs with #pragma pack(1).
package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
)

// ErrShort means a packet body ended before all of its fields were read.
var ErrShort = errors.New("protocol: packet shorter than its layout")

// Reader reads fields in order and remembers the first failure, so a decoder can read
// every field and check the error once at the end.
type Reader struct {
	data []byte
	err  error
}

func NewReader(data []byte) *Reader {
	return &Reader{data: data}
}

func (r *Reader) take(n int) []byte {
	if r.err != nil || len(r.data) < n {
		r.err = ErrShort
		return make([]byte, n)
	}
	field := r.data[:n]
	r.data = r.data[n:]
	return field
}

func (r *Reader) U8() uint8   { return r.take(1)[0] }
func (r *Reader) U16() uint16 { return binary.LittleEndian.Uint16(r.take(2)) }
func (r *Reader) U32() uint32 { return binary.LittleEndian.Uint32(r.take(4)) }
func (r *Reader) I32() int32  { return int32(binary.LittleEndian.Uint32(r.take(4))) }
func (r *Reader) Skip(n int)  { r.take(n) }

// String reads a fixed-size, NUL-padded text field and returns the text before the
// first NUL.
func (r *Reader) String(size int) string {
	field := r.take(size)
	if end := bytes.IndexByte(field, 0); end >= 0 {
		field = field[:end]
	}
	return string(field)
}

func (r *Reader) Err() error { return r.err }

// Writer builds a packet field by field.
type Writer struct {
	buf []byte
}

func NewWriter(header byte, size int) *Writer {
	w := &Writer{buf: make([]byte, 0, size)}
	w.U8(header)
	return w
}

func (w *Writer) U8(v uint8)   { w.buf = append(w.buf, v) }
func (w *Writer) U16(v uint16) { w.buf = binary.LittleEndian.AppendUint16(w.buf, v) }
func (w *Writer) U32(v uint32) { w.buf = binary.LittleEndian.AppendUint32(w.buf, v) }
func (w *Writer) I32(v int32)  { w.buf = binary.LittleEndian.AppendUint32(w.buf, uint32(v)) }
func (w *Writer) Raw(b []byte) { w.buf = append(w.buf, b...) }

// String writes text into a fixed-size field, cut to leave room for the terminating NUL
// the client expects.
func (w *Writer) String(s string, size int) {
	field := make([]byte, size)
	copy(field[:size-1], s)
	w.buf = append(w.buf, field...)
}

func (w *Writer) Bytes() []byte { return w.buf }
