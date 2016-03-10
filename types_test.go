package binny

import (
	"bytes"
	"encoding"
	"fmt"
	"log"
	"math"
	"math/big"
	"math/rand"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"unsafe"

	"encoding/binary"
	"encoding/gob"
	"testing/quick"
)

func init() {
	log.SetFlags(log.Lshortfile)
}

var useQuick = !strings.HasPrefix(runtime.Version(), "go1.5") && !strings.HasPrefix(runtime.Version(), "go1.4")

type S struct {
	Str    string   `json:",omitempty"`
	Ignore string   `json:",omitempty" binny:"-"`
	I8     int8     `json:",omitempty"`
	U8     uint8    `json:",omitempty"`
	I16    int16    `json:",omitempty"`
	U16    uint16   `json:",omitempty"`
	I32    int32    `json:",omitempty"`
	U32    uint32   `json:",omitempty"`
	I64    int64    `json:",omitempty"`
	U64    uint64   `json:",omitempty"`
	F32    float32  `json:",omitempty"`
	F64    float64  `json:",omitempty"`
	Bi     *big.Int `json:",omitempty"`
	S      *S       `binny:"s"`
	Z      uint     `json:",omitempty"`
}

type SI S

var _ interface {
	Marshaler
	Unmarshaler
} = (*SI)(nil)

func (s *SI) MarshalBinny(enc *Encoder) error {
	enc.WriteUint8(0x01) // version
	enc.WriteString(s.Str)
	enc.WriteInt8(s.I8)
	enc.WriteUint8(s.U8)
	enc.WriteInt16(s.I16)
	enc.WriteUint16(s.U16)
	enc.WriteInt32(s.I32)
	enc.WriteUint32(s.U32)
	enc.WriteInt64(s.I64)
	enc.WriteUint64(s.U64)
	enc.WriteFloat32(s.F32)
	enc.WriteFloat64(s.F64)
	enc.WriteGob(s.Bi)
	if s.S == nil {
		enc.WriteBool(false)
	} else {
		enc.WriteBool(true)
		tmp := SI(*s.S)
		tmp.MarshalBinny(enc)
	}
	return enc.WriteUint(uint64(s.Z))
}

func (s *SI) UnmarshalBinny(dec *Decoder) (err error) {
	var v uint8
	if v, err = dec.ReadUint8(); err != nil {
		return err
	}
	if v != 0x01 {
		return fmt.Errorf("invalid version")
	}
	if s.Str, err = dec.ReadString(); err != nil {
		return
	}
	if s.I8, err = dec.ReadInt8(); err != nil {
		return
	}
	if s.U8, err = dec.ReadUint8(); err != nil {
		return
	}
	if s.I16, err = dec.ReadInt16(); err != nil {
		return
	}
	if s.U16, err = dec.ReadUint16(); err != nil {
		return
	}
	if s.I32, err = dec.ReadInt32(); err != nil {
		return
	}
	if s.U32, err = dec.ReadUint32(); err != nil {
		return
	}
	if s.I64, err = dec.ReadInt64(); err != nil {
		return
	}
	if s.U64, err = dec.ReadUint64(); err != nil {
		return
	}
	if s.F32, err = dec.ReadFloat32(); err != nil {
		return
	}
	if s.F64, err = dec.ReadFloat64(); err != nil {
		return
	}
	s.Bi = big.NewInt(0)
	if err = dec.ReadGob(s.Bi); err != nil {
		return
	}
	b, _ := dec.ReadBool()
	if b {
		tmp := SI{}
		if err = tmp.UnmarshalBinny(dec); err != nil {
			return
		}
		v := S(tmp)
		s.S = &v
	}
	z, _, err := dec.ReadUint()
	s.Z = uint(z)
	return err
}

var le = binary.LittleEndian

type expValue struct {
	in []string
	b  []byte
}

type Len uint64

func Exp(in ...interface{}) (ev expValue) {
L:
	for _, v := range in {
		switch v := v.(type) {
		case Type:
			ev.b = append(ev.b, byte(v))
			ev.in = append(ev.in, v.String())
			continue L
		case string:
			ev.b = append(ev.b, autoUint(uint64(len(v)), true)...)
			ev.b = append(ev.b, v...)
			ev.in = append(ev.in, fmt.Sprintf("%q", v))
			continue L
		case int:
			ev.b = append(ev.b, byte(v))
			ev.in = append(ev.in, fmt.Sprintf("%v", v))
			continue L
		case int64:
			ev.b = append(ev.b, autoInt(v)...)
			ev.in = append(ev.in, fmt.Sprintf("%v", v))
			continue L
		case uint64:
			ev.b = append(ev.b, autoUint(v, false)...)
			ev.in = append(ev.in, fmt.Sprintf("%v", v))
			continue L
		case Len:
			ev.b = append(ev.b, autoUint(uint64(v), true)...)
			ev.in = append(ev.in, fmt.Sprintf("%v", v))
			continue L
		case int32:
			i := make([]byte, 10)
			ev.b = append(ev.b, i[:binary.PutVarint(i, int64(v))]...)
		case uint32:
			i := make([]byte, 10)
			ev.b = append(ev.b, i[:binary.PutUvarint(i, uint64(v))]...)
		case float32:
			i := make([]byte, 4)
			le.PutUint32(i, math.Float32bits(v))
			ev.b = append(ev.b, i...)
			ev.in = append(ev.in, fmt.Sprintf("%v", v))
			continue L
		case float64:
			i := make([]byte, 8)
			le.PutUint64(i, math.Float64bits(v))
			ev.b = append(ev.b, i...)
			ev.in = append(ev.in, fmt.Sprintf("%v", v))
			continue L
		case gob.GobEncoder:
			b, _ := v.GobEncode()
			ev.b = append(ev.b, autoUint(uint64(len(b)), true)...)
			ev.b = append(ev.b, b...)
		case encoding.BinaryMarshaler:
			b, _ := v.MarshalBinary()
			ev.b = append(ev.b, autoUint(uint64(len(b)), true)...)
			ev.b = append(ev.b, b...)
		default:
			panic(v)
		}
		ev.in = append(ev.in, fmt.Sprintf("%T(%+v)", v, v))
	}
	return
}

func autoUint(u uint64, ln bool) (v []byte) {
	switch {
	case u <= math.MaxUint8:
		v = []byte{byte(Uint8), byte(u)}
	case u <= math.MaxUint16:
		v = append([]byte{byte(Uint16)}, (*[2]byte)(unsafe.Pointer(&u))[:2:2]...)
	case u <= math.MaxUint32:
		v = append([]byte{byte(Uint32)}, (*[4]byte)(unsafe.Pointer(&u))[:4:4]...)
	default:
		v = append([]byte{byte(Uint64)}, (*[8]byte)(unsafe.Pointer(&u))[:8:8]...)
	}
	if ln {
		return v
	}
	return v[1:]

}

func autoInt(v int64) []byte {
	u := v
	if u < 0 {
		u = -u
	}
	if u <= math.MaxInt8 {
		return []byte{byte(u)}
	}
	if u <= math.MaxInt16 {
		return (*[8]byte)(unsafe.Pointer(&v))[:2:2]
	}
	if u <= math.MaxInt32 {
		return (*[8]byte)(unsafe.Pointer(&v))[:4:4]
	}
	return (*[8]byte)(unsafe.Pointer(&v))[:8:8]
}

var SLen = len(cachedTypeFields(reflect.TypeOf(S{})))

var benchVal = S{
	I8:     1,
	U16:    2,
	Str:    "hello",
	Ignore: "xczczcasdsa",
	S: &S{
		I32: 3,
		Str: "bye",
		S: &S{
			U64: math.MaxUint64,
			S: &S{
				F32: math.MaxFloat32,
				F64: math.MaxFloat64,
				U64: math.MaxUint64,
				Bi:  bigIntVal,
				Str: "w00t",
			},
		},
	},
}

type SAll struct {
	I    int
	U    uint
	I8   int8
	U8   uint8
	I16  int16
	U16  uint16
	I32  int32
	U32  uint32
	I64  int64
	U64  uint64
	F32  float32
	F64  float64
	C64  complex64
	C128 complex128
	S    string
	BS   []byte
	M    map[string]*SAll
	M2   map[MapKey]struct{}
}

type MapKey struct {
	A uint16
	B int8
	C int8
}

func (s *SAll) NotEq(t *testing.T, o *SAll) (errored bool) {
	if s == nil && o == nil || o == s {
		return false
	}
	if s == nil || o == nil {
		t.Logf("s == nil || o == nil\n%+v\n%+v", s, o)
		return true
	}
	if s.I != o.I {
		t.Logf("I wanted %v, got %v.", s.I, o.I)
		errored = true
	}

	if s.U != o.U {
		t.Logf("U wanted %v, got %v.", s.U, o.U)
		errored = true
	}

	if s.I8 != o.I8 {
		t.Logf("I8 wanted %v, got %v.", s.I8, o.I8)
		errored = true
	}

	if s.U8 != o.U8 {
		t.Logf("U8 wanted %v, got %v.", s.U8, o.U8)
		errored = true
	}

	if s.I16 != o.I16 {
		t.Logf("I16 wanted %v, got %v.", s.I16, o.I16)
		errored = true
	}

	if s.U16 != o.U16 {
		t.Logf("U16 wanted %v, got %v.", s.U16, o.U16)
		errored = true
	}

	if s.I32 != o.I32 {
		t.Logf("I32 wanted %v, got %v.", s.I32, o.I32)
		errored = true
	}

	if s.U32 != o.U32 {
		t.Logf("U32 wanted %v, got %v.", s.U32, o.U32)
		errored = true
	}

	if s.I64 != o.I64 {
		t.Logf("I64 wanted %v, got %v.", s.I64, o.I64)
		errored = true
	}

	if s.U64 != o.U64 {
		t.Logf("U64 wanted %v, got %v.", s.U64, o.U64)
		errored = true
	}

	if s.F32 != o.F32 {
		t.Logf("F32 wanted %v, got %v.", s.F32, o.F32)
		errored = true
	}

	if s.F64 != o.F64 {
		t.Logf("F64 wanted %v, got %v.", s.F64, o.F64)
		errored = true
	}

	if s.C64 != o.C64 {
		t.Logf("C64 wanted %v, got %v.", s.C64, o.C64)
		errored = true
	}

	if s.C128 != o.C128 {
		t.Logf("C128 wanted %v, got %v.", s.C128, o.C128)
		errored = true
	}

	if s.S != o.S {
		t.Logf("S wanted %v, got %v.", s.S, o.S)
		errored = true
	}

	if bytes.Compare(s.BS, o.BS) != 0 {
		t.Logf("BS wanted %v, got %v.", s.BS, o.BS)
		errored = true
	}

	if len(s.M) != len(o.M) {
		t.Logf("M wanted %v, got %v.", s.M, o.M)
		errored = true
	}

	for k, v := range s.M {
		errored = errored || v.NotEq(t, o.M[k])
	}

	if len(s.M2) != len(o.M2) {
		t.Logf("M wanted %v, got %v.", s.M, o.M)
		errored = true
	}
	for k := range s.M2 {
		if _, ok := o.M2[k]; !ok {
			t.Logf("M2 wanted %v, got none", k)
		}
	}
	return
}

func TestMortalKombat(t *testing.T) {
	if !useQuick {
		t.Skipf("doesn't work on %v", runtime.Version())
	}
	cfg := &quick.Config{
		MaxCount: 10000,
		Rand:     rand.New(rand.NewSource(42)),
	}
	if testing.Short() {
		cfg.MaxCount = 1000
	}
	check := func(s *SAll) bool {
		if s == nil {
			return true
		}
		b, err := Marshal(s)
		if err != nil {
			t.Error(err)
			return false
		}
		var s2 SAll
		if err = Unmarshal(b, &s2); err != nil {
			t.Error(err)
			return false
		}
		return !s.NotEq(t, &s2)
	}
	if err := quick.Check(check, cfg); err != nil {
		t.Fatal(err)
	}
}
