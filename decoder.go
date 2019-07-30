package binny

import (
	"bufio"
	"bytes"
	"encoding"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"reflect"
	"unsafe"
)

var (
	// ErrNoPointer gets returned if the user passes a non-pointer to Decode
	ErrNoPointer = errors.New("can't decode to a non-pointer")
)

const DefaultDecoderBufferSize = 4096

// Unmarshaler is the interface implemented by objects that can unmarshal a binary representation of themselves.
// Implementing this bypasses reflection and is generally faster.
type Unmarshaler interface {
	UnmarshalBinny(dec *Decoder) error
}

// A Decoder reads binary data from an input stream, it also does a little bit of buffering.
type Decoder struct {
	r *bufio.Reader

	buf [16]byte
}

// NewDecoder is an alias for NewDecoder(r, DefaultDecoderBufferSize)
func NewDecoder(r io.Reader) *Decoder {
	return NewDecoderSize(r, DefaultDecoderBufferSize)
}

// NewDecoder returns a new decoder that reads from r with specific buffer size.
//
// The decoder introduces its own buffering and may
// read data from r beyond the requested values.
func NewDecoderSize(r io.Reader, sz int) *Decoder {
	if sz < 16 {
		sz = 16
	}

	return &Decoder{
		r: bufio.NewReaderSize(r, sz),
	}
}

// Reset discards any buffered data, resets all state, and switches
// the buffered reader to read from r.
func (dec *Decoder) Reset(r io.Reader) {
	dec.r.Reset(r)
}

func (dec *Decoder) readType() (Type, error) {
	b, err := dec.r.ReadByte()
	return Type(b), err
}

func (dec *Decoder) peekType() Type {
	b, _ := dec.r.ReadByte()
	dec.r.UnreadByte()
	return Type(b)
}

func (dec *Decoder) expectType(et Type) error {
	if t, err := dec.readType(); t != et {
		if err != nil {
			return err
		}
		return DecoderTypeError{et.String(), t}
	}
	return nil
}

// ReadBool returns a bool or an error.
func (dec *Decoder) ReadBool() (bool, error) {
	ft, _ := dec.readType()
	switch ft {
	case BoolTrue:
		return true, nil
	case BoolFalse:
		return false, nil
	}
	return false, DecoderTypeError{"Bool", ft}
}

func (dec *Decoder) ReadInt8() (int8, error) {
	if err := dec.expectType(Int8); err != nil {
		return 0, err
	}
	b, err := dec.r.ReadByte()
	return int8(b), err
}

func (dec *Decoder) ReadInt16() (int16, error) {
	if err := dec.expectType(Int16); err != nil {
		return 0, err
	}
	buf := dec.buf[:2]
	_, err := dec.Read(buf)
	return *(*int16)(unsafe.Pointer(&buf[0])), err
}

func (dec *Decoder) ReadInt32() (int32, error) {
	if err := dec.expectType(Int32); err != nil {
		return 0, err
	}
	buf := dec.buf[:4]
	_, err := dec.Read(buf)
	return *(*int32)(unsafe.Pointer(&buf[0])), err
}

func (dec *Decoder) ReadInt64() (int64, error) {
	if err := dec.expectType(Int64); err != nil {
		return 0, err
	}
	buf := dec.buf[:8]
	_, err := dec.Read(buf)
	return *(*int64)(unsafe.Pointer(&buf[0])), err
}

func (dec *Decoder) ReadVarInt() (int64, error) {
	if err := dec.expectType(VarInt); err != nil {
		return 0, err
	}
	return binary.ReadVarint(dec.r)
}

// ReadInt retruns an int/varint value and the size of it (8, 16, 32, 64) or an error.
func (dec *Decoder) ReadInt() (int64, uint8, error) {
	ft := dec.peekType()
	switch ft {
	case Int8:
		v, err := dec.ReadInt8()
		return int64(v), 8, err
	case Int16:
		v, err := dec.ReadInt16()
		return int64(v), 16, err
	case Int32:
		v, err := dec.ReadInt32()
		return int64(v), 32, err
	case Int64:
		v, err := dec.ReadInt64()
		return v, 64, err
	case VarInt:
		v, err := dec.ReadVarInt()
		return v, 64, err
	}
	return 0, 0, DecoderTypeError{"int", ft}
}

func (dec *Decoder) ReadUint8() (uint8, error) {
	if err := dec.expectType(Uint8); err != nil {
		return 0, err
	}
	return dec.r.ReadByte()
}

func (dec *Decoder) ReadUint16() (uint16, error) {
	if err := dec.expectType(Uint16); err != nil {
		return 0, err
	}
	buf := dec.buf[:2]
	_, err := dec.Read(buf)
	return *(*uint16)(unsafe.Pointer(&buf[0])), err
}

func (dec *Decoder) ReadUint32() (uint32, error) {
	if err := dec.expectType(Uint32); err != nil {
		return 0, err
	}
	buf := dec.buf[:4]
	_, err := dec.Read(buf)
	return *(*uint32)(unsafe.Pointer(&buf[0])), err
}

func (dec *Decoder) ReadUint64() (uint64, error) {
	if err := dec.expectType(Uint64); err != nil {
		return 0, err
	}
	buf := dec.buf[:8]
	_, err := dec.Read(buf)
	return *(*uint64)(unsafe.Pointer(&buf[0])), err
}

func (dec *Decoder) ReadVarUint() (uint64, error) {
	if err := dec.expectType(VarUint); err != nil {
		return 0, err
	}
	return binary.ReadUvarint(dec.r)
}

// ReadUint retruns an uint/varuint value and the size of it (8, 16, 32, 64) or an error.
func (dec *Decoder) ReadUint() (v uint64, sz uint8, err error) {
	ft := dec.peekType()
	switch ft {
	case Uint8:
		v, err := dec.ReadUint8()
		return uint64(v), 8, err
	case Uint16:
		v, err := dec.ReadUint16()
		return uint64(v), 16, err
	case Uint32:
		v, err := dec.ReadUint32()
		return uint64(v), 32, err
	case Uint64:
		v, err := dec.ReadUint64()
		return v, 64, err
	case VarUint:
		v, err := dec.ReadVarUint()
		return v, 64, err
	}
	return 0, 0, DecoderTypeError{"uint", ft}
}

// ReadFloat32 returns a float32 or an error.
func (dec *Decoder) ReadFloat32() (float32, error) {
	if err := dec.expectType(Float32); err != nil {
		return 0, err
	}
	buf := dec.buf[:4]
	_, err := dec.Read(buf)
	return *(*float32)(unsafe.Pointer(&buf[0])), err
}

// ReadFloat64 returns a float64 or an error.
func (dec *Decoder) ReadFloat64() (float64, error) {
	if err := dec.expectType(Float64); err != nil {
		return 0, err
	}
	buf := dec.buf[:8]
	_, err := dec.Read(buf)
	return *(*float64)(unsafe.Pointer(&buf[0])), err
}

// ReadComplex64 returns a complex64 or an error.
func (dec *Decoder) ReadComplex64() (complex64, error) {
	if err := dec.expectType(Complex64); err != nil {
		return 0, err
	}

	buf := dec.buf[:8]
	_, err := dec.Read(buf)
	return *(*complex64)(unsafe.Pointer(&buf[0])), err
}

// ReadComplex128 returns a complex128 or an error.
func (dec *Decoder) ReadComplex128() (complex128, error) {
	if err := dec.expectType(Complex128); err != nil {
		return 0, err
	}
	buf := dec.buf[:16]
	_, err := dec.Read(buf)
	return *(*complex128)(unsafe.Pointer(&buf[0])), err
}

func (dec *Decoder) readBytes(exp Type) ([]byte, error) {
	if err := dec.expectType(exp); err != nil {
		return nil, err
	}

	sz, _, err := dec.ReadUint()
	if err != nil || sz == 0 {
		return nil, err
	}

	buf := make([]byte, sz)
	_, err = io.ReadFull(dec.r, buf)
	return buf, err
}

// ReadBytes returns a byte slice.
func (dec *Decoder) ReadBytes() ([]byte, error) {
	return dec.readBytes(ByteSlice)
}

// ReadBytes returns a string.
func (dec *Decoder) ReadString() (string, error) {
	b, err := dec.readBytes(String)

	return *(*string)(unsafe.Pointer(&b)), err
}

// ReadBinary decodes and reads an object that implements the `encoding.BinaryUnmarshaler` interface.
func (dec *Decoder) ReadBinary(v encoding.BinaryUnmarshaler) error {
	b, err := dec.readBytes(Binary)
	if err != nil {
		return err
	}
	return v.UnmarshalBinary(b)
}

// ReadGob decodes and reads an object that implements the `gob.GobDecoder` interface.
func (dec *Decoder) ReadGob(v gob.GobDecoder) error {
	b, err := dec.readBytes(Gob)
	if err != nil {
		return err
	}
	return v.GobDecode(b)
}

// Decode reads the next binny-encoded value from its
// input and stores it in the value pointed to by v.
func (dec *Decoder) Decode(v interface{}) (err error) {
	switch v := v.(type) {
	case Unmarshaler:
		return v.UnmarshalBinny(dec)
	case encoding.BinaryUnmarshaler:
		return dec.ReadBinary(v)
	case gob.GobDecoder:
		return dec.ReadGob(v)
	case *string:
		*v, err = dec.ReadString()
		return
	case *[]byte:
		*v, err = dec.ReadBytes()
		return
	case *int64:
		*v, _, err = dec.ReadInt()
		return
	case *int32:
		var i int64
		i, _, err = dec.ReadInt()
		*v = int32(i)
		return
	case *int16:
		var i int64
		i, _, err = dec.ReadInt()
		*v = int16(i)
		return
	case *int8:
		var i int64
		i, _, err = dec.ReadInt()
		*v = int8(i)
		return
	case *int:
		var i int64
		i, _, err = dec.ReadInt()
		*v = int(i)
		return
	case *uint64:
		*v, _, err = dec.ReadUint()
		return
	case *uint32:
		var i uint64
		i, _, err = dec.ReadUint()
		*v = uint32(i)
		return
	case *uint16:
		var i uint64
		i, _, err = dec.ReadUint()
		*v = uint16(i)
		return
	case *uint8:
		var i uint64
		i, _, err = dec.ReadUint()
		*v = uint8(i)
		return
	case *uint:
		var i uint64
		i, _, err = dec.ReadUint()
		*v = uint(i)
		return
	case *uintptr:
		var i uint64
		i, _, err = dec.ReadUint()
		*v = uintptr(i)
		return
	case *float32:
		*v, err = dec.ReadFloat32()
		return
	case *float64:
		*v, err = dec.ReadFloat64()
		return
	case *complex64:
		*v, err = dec.ReadComplex64()
		return
	case *complex128:
		*v, err = dec.ReadComplex128()
		return
	case *bool:
		*v, err = dec.ReadBool()
		return
	case nil:
		return fmt.Errorf("can't decode a nil value")
	}
	return dec.decodeValue(reflect.ValueOf(v))
}

func (dec *Decoder) decodeValue(v reflect.Value) error {
	if v.Kind() != reflect.Ptr || !v.Elem().CanSet() {
		return ErrNoPointer
	}
	if v.IsNil() {
		return fmt.Errorf("can't decode a nil value: %v", v.Type())
	}
	fn := typeDecoder(v.Type())
	return fn(dec, v)
}

// Read allows the Decoder to be used as an io.Reader, note that internally this calls io.ReadFull().
func (dec *Decoder) Read(p []byte) (int, error) {
	return io.ReadFull(dec.r, p)
}

// Unmarshal is an alias for (sync.Pool'ed) NewDecoder(bytes.NewReader(b)).Decode(v)
func Unmarshal(b []byte, v interface{}) error {
	dec := getDec(bytes.NewReader(b))
	err := dec.Decode(v)
	putDec(dec)
	return err
}
