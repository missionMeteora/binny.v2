package binny

import (
	"encoding"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"unsafe"

	"log"

	"encoding/binary"
	"encoding/gob"
)

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

func init() {
	log.SetFlags(log.Lshortfile)
}

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
