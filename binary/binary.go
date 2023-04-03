// Package binary provides a binary format for structured logging with slog.
package binary

import (
	"encoding"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"sync"
	"time"

	"golang.org/x/exp/slog"
)

type Encoder struct {
	buf  []byte
	abuf [1024]byte
	err  error
}

var pool = sync.Pool{New: func() any { return new(Encoder) }}

func GetEncoder() *Encoder {
	e := pool.Get().(*Encoder)
	e.err = nil
	e.buf = e.abuf[:0]
	return e
}

func PutEncoder(e *Encoder) { pool.Put(e) }

func (e *Encoder) EncodeKey(key string) {
	e.encodeString(key)
}

func (e *Encoder) EncodeValue(v slog.Value) {
	v = v.Resolve()
	switch v.Kind() {
	case slog.KindString:
		e.encodeString(v.String())
	case slog.KindInt64:
		e.encodeInt(v.Int64())
	case slog.KindUint64:
		e.encodeUint(v.Uint64())
	case slog.KindFloat64:
		e.encodeFloat(v.Float64())
	case slog.KindBool:
		e.encodeBool(v.Bool())
	case slog.KindDuration:
		e.encodeOp(opDuration)
		e.encodeInt(v.Duration().Nanoseconds())
	case slog.KindTime:
		e.encodeTime(v.Time())
	case slog.KindAny:
		e.encodeAny(v.Any())
	case slog.KindGroup:
		attrs := v.Group()
		e.encodeOp(opList)
		e.encodeInt(int64(len(attrs) * 2))
		for _, a := range attrs {
			e.EncodeKey(a.Key)
			e.EncodeValue(a.Value)
		}

	case slog.KindLogValuer:
		panic("impossible")
	default:
		panic("unknown kind")
	}
}

const magic uint32 = 0xBAFEDC01

func (e *Encoder) WriteTo(w io.Writer) (int, error) {
	if e.err != nil {
		return 0, e.err
	}
	if len(e.buf) > math.MaxUint32 {
		return 0, errors.New("buffer too big")
	}
	var header [8]byte
	binary.LittleEndian.PutUint32(header[0:4], magic)
	binary.LittleEndian.PutUint32(header[4:], uint32(len(e.buf)))
	if n, err := w.Write(header[:]); err != nil {
		return n, err
	}
	return w.Write(e.buf)
}

const smallIntEnd = 200

type op uint8

const (
	opInt op = iota + smallIntEnd
	opUint
	opFloat
	opTrue
	opFalse
	opString
	opBytes
	opDuration
	opTime
	opList
)

func (e *Encoder) encodeOp(o op) {
	e.buf = append(e.buf, byte(o))
}

func (e *Encoder) encodeInt(i int64) {
	if i >= 0 && i < smallIntEnd {
		e.buf = append(e.buf, byte(i))
	} else {
		e.encodeOp(opInt)
		e.buf = binary.AppendVarint(e.buf, i)
	}
}

func (e *Encoder) encodeUint(u uint64) {
	e.encodeOp(opUint)
	e.buf = binary.AppendUvarint(e.buf, u)
}

func (e *Encoder) encodeFloat(f float64) {
	e.encodeOp(opFloat)
	e.buf = binary.LittleEndian.AppendUint64(e.buf, math.Float64bits(f))
}

func (e *Encoder) encodeBool(b bool) {
	if b {
		e.encodeOp(opTrue)
	} else {
		e.encodeOp(opFalse)
	}
}

func (e *Encoder) encodeString(s string) {
	e.encodeOp(opString)
	e.encodeInt(int64(len(s)))
	e.buf = append(e.buf, s...)
}

func (e *Encoder) encodeBytes(b []byte) {
	e.encodeOp(opBytes)
	e.encodeInt(int64(len(b)))
	e.buf = append(e.buf, b...)
}

func (e *Encoder) encodeTime(t time.Time) {
	e.encodeOp(opTime)
	data, err := t.MarshalBinary()
	if err != nil {
		e.err = err
		return
	}
	e.buf = append(e.buf, data...)
}

func (e *Encoder) encodeAny(x any) {
	if tm, ok := x.(encoding.TextMarshaler); ok {
		data, err := tm.MarshalText()
		if err != nil {
			e.err = err
			return
		}
		e.encodeBytes(data)
		return
	}
	e.encodeString(fmt.Sprint(x))

}

////////////////////////////////////////////////////////////////

type DecodeVisitor interface {
	Int(key []byte, val int64)
	Uint(key []byte, val uint64)
	String(key, val []byte)
	Bytes(key, val []byte)
	Bool(key []byte, val bool)
	Float(key []byte, val float64)
	Duration(key []byte, val time.Duration)
	Time(key []byte, val time.Time)
	Group(n int)
}

func Decode(r io.Reader, v DecodeVisitor) error {
	buf, err := readHeader(r)
	if err != nil {
		return err
	}
	for len(buf) > 0 {
		// Decode key.
		if buf[0] != byte(opString) {
			return errors.New("key is not a string")
		}
		key, buf := decodeString(buf[1:])
		// Decode value.
		b, buf := buf[0], buf[1:]
		if b < smallIntEnd {
			v.Int(key, int64(b))
		} else {
			switch op(b) {
			case opInt:
				i, n := binary.Varint(buf)
				v.Int(key, i)
				buf = buf[n:]
			case opUint:
				u, n := binary.Uvarint(buf)
				v.Uint(key, u)
				buf = buf[n:]
			case opFloat:
				u := binary.LittleEndian.Uint64(buf)
				v.Float(key, math.Float64frombits(u))
				buf = buf[8:]
			case opTrue:
				v.Bool(key, true)
			case opFalse:
				v.Bool(key, false)
			case opString:
				l, n := binary.Varint(buf)
				buf = buf[n:]
				v.String(key, buf[:l])
				buf = buf[l:]
			default:
				panic(fmt.Sprintf("unknown op %v", op(b)))
			}
		}
	}
	return nil
}

// opBytes
// opDuration
// opTime
// opList

func decodeString(buf []byte) (str, newbuf []byte) {
	l, n := binary.Varint(buf)
	len := int(l)
	return buf[n : n+len], buf[n+len:]
}

func readHeader(r io.Reader) ([]byte, error) {
	var header [8]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, err
	}
	if m := binary.LittleEndian.Uint32(header[0:4]); m != magic {
		return nil, fmt.Errorf("got magic %x, want %x", m, magic)
	}
	length := binary.LittleEndian.Uint32(header[4:])
	buf := make([]byte, length) // TODO: pool
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}
