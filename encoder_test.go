package binny

import (
	"bytes"
	"math"
	"math/big"
	"strconv"
	"strings"
	"testing"
	"time"
)

var (
	bigIntVal     = big.NewInt(0).Mul(big.NewInt(0).SetUint64(math.MaxUint64), big.NewInt(15))
	bigIntValB, _ = bigIntVal.GobEncode()
	timeNow       = time.Now().UTC()
	timeNowB, _   = timeNow.MarshalBinary()
)

type uM map[uint64]interface{}
type sM map[sK]int
type sK struct {
	A int
	B int
}

type sKM struct{}

func (s *sKM) MarshalBinny(enc *Encoder) error {
	return enc.WriteUint(0xFFFF)
}

var encoderTests = []struct {
	name string
	in   interface{}
	exp  expValue
}{
	{"string", "test", Exp(String, "test")},
	{"int64", int64(math.MaxInt64) / 2, Exp(Int64, int64(math.MaxInt64)/2)},
	{"-int64", int64(math.MinInt64) / 2, Exp(Int64, int64(math.MinInt64)/2)},
	{"uint64", uint64(math.MaxUint64), Exp(Uint64, uint64(math.MaxUint64))},
	{"int8", math.MaxInt8, Exp(Int8, int64(math.MaxInt8))},
	{"float32", float32(math.MaxFloat32), Exp(Float32, float32(math.MaxFloat32))},
	{"-float32", float32(math.SmallestNonzeroFloat32), Exp(Float32, float32(math.SmallestNonzeroFloat32))},
	{"float64", float64(math.MaxFloat64), Exp(Float64, float64(math.MaxFloat64))},
	{"-float64", float64(math.SmallestNonzeroFloat64), Exp(Float64, float64(math.SmallestNonzeroFloat64))},
	{"bigInt", bigIntVal, Exp(Gob, bigIntVal)},
	{"time", timeNow, Exp(Binary, timeNow)},
	{"S", S{Str: "hi", U64: 25, Ignore: "booo"}, Exp(Struct, String, "Str", String, "hi", String, "U64", Uint8, 25, EOV)},
	// ptr should have the same value
	{"*S", &S{Str: "hi", U64: 25, Ignore: "booo"}, Exp(Struct, String, "Str", String, "hi", String, "U64", Uint8, 25, EOV)},
	{"*S.s", &S{Str: "hi", U64: 25, Ignore: "booo", Z: 55, S: &S{I32: 10}}, Exp(Struct, String, "Str", String, "hi", String, "U64", Uint8, 25, String, "s", Struct, String, "I32", Int8, 10, EOV, String, "Z", Uint8, 55, EOV)},
	{"*S.s.s", &S{Str: "hi", U64: 25, Ignore: "booo", Z: 55, S: &S{I32: 10, S: &S{U8: 10}}}, Exp(Struct, String, "Str", String, "hi", String, "U64", Uint8, 25, String, "s", Struct, String, "I32", Int8, 10, String, "s", Struct, String, "U8", Uint8, 10, EOV, EOV, String, "Z", Uint8, 55, EOV)},
	{"[]interface{}", []interface{}{uint64(10), nil, &S{U64: 5323131}, "str", float64(3123.22)}, Exp(Slice, Len(5), Uint8, uint64(10), Nil, Struct, String, "U64", Uint32, uint64(5323131), EOV, String, "str", Float64, float64(3123.22), EOV)},
	{"[...]interface{}", [...]interface{}{uint64(10), nil, &S{U64: 5323131}, "str", float64(3123.22)}, Exp(Slice, Len(5), Uint8, uint64(10), Nil, Struct, String, "U64", Uint32, uint64(5323131), EOV, String, "str", Float64, float64(3123.22), EOV)},
	{"map[uint64]interface{}", uM{math.MaxUint64: 555}, Exp(Map, Len(1), Uint64, uint64(math.MaxUint64), Int16, int64(555), EOV)},
	{"map[struct]int", sM{sK{20, 1 << 21}: 1 << 20}, Exp(Map, Len(1), Struct, String, "A", Int8, 20, String, "B", Int32, int64(1<<21), EOV, Int32, int64(1<<20), EOV)},
}

func TestEncoder(t *testing.T) {
	for _, et := range encoderTests {
		v, err := Marshal(et.in)
		if err != nil {
			t.Fatalf("%s: %v", et.name, err)
		}
		if bytes.Compare(et.exp.b, v) != 0 {
			t.Fatalf("%10s: failed\nexp: %v\ngot: %v", et.name, et.exp.b, v)
		}
		if !testing.Short() {
			t.Logf("%10s, blen=%-03d: %s", et.name, len(et.exp.b), "{"+strings.Join(et.exp.in, ", ")+"}")
		}
	}
}

func TestMarshalBinny(t *testing.T) {
	var s sKM
	var v uint64
	b, err := Marshal(&s)
	if err != nil {
		t.Fatal(err)
	}
	if err = Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	if v != 0xFFFF {
		t.Fatalf("expected 0xFFFF, got 0x%X", v)
	}
}

func BenchmarkEncodeMap(b *testing.B) {
	m := map[string]int{}
	for i := 0; i < 1000; i++ {
		m["x"+strconv.Itoa(i)+"x"] = i
	}
	buf := bytes.NewBuffer(nil)
	enc := NewEncoder(buf)
	enc.Encode(m)
	enc.Flush()
	b.SetBytes(int64(buf.Len()))
	buf.Reset()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := enc.Encode(m); err != nil {
			b.Fatal(err)
		}
		enc.Flush()
		buf.Reset()
	}
}

func benchEncoder(b *testing.B, o interface{}) {
	buf := bytes.NewBuffer(make([]byte, 0, 4096))
	enc := NewEncoder(buf)
	var ln int64
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := enc.Encode(o); err != nil {
			b.Fatal(err)
		}
		enc.Flush()
		ln = int64(buf.Len())
		buf.Reset()
	}
	b.SetBytes(ln)
}

func BenchmarkMarshalerBig(b *testing.B) {
	tmp := SI(benchVal)
	benchEncoder(b, &tmp)
}

func BenchmarkMarshalerSmall(b *testing.B) {
	tmp := SI(*benchVal.S.S.S)
	benchEncoder(b, &tmp)
}

func BenchmarkEncoderBig(b *testing.B) {
	if testing.Short() {
		b.Skip("not supported on short")
	}
	benchEncoder(b, &benchVal)
}

func BenchmarkEncoderSmall(b *testing.B) { benchEncoder(b, benchVal.S.S.S) }
