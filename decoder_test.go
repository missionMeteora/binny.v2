package binny

import (
	"bytes"
	"reflect"
	"strconv"
	"testing"
	"time"
)

var (
	uintVal = uint64(0x666)

	decoderTests = []struct {
		name string
		in   interface{}
	}{
		{"string", "hello world"},
		{"int8", int8(10)},
		{"int16", int16(1<<15 - 1)},
		{"int32", int32(1 << 30)},
		{"int64", int64(1 << 60)},
		{"*int64", &uintVal},
		{"*big.Int", bigIntVal},
		{"[]uint64", []uint64{1<<8 - 1, 1<<16 - 1, 1<<32 - 1, 1<<64 - 1}},
		{"[...]uint64", [...]uint64{1<<8 - 1, 1<<16 - 1, 1<<32 - 1, 1<<64 - 1}},
		{"time", time.Now().UTC()},
		{"map[string]int", map[string]int{"hi": 1, "bye": 5}},
		{"S", S{U64: 64}},
		{"*S", &S{U64: 64}},
		{"S.s.s", benchVal},
		{"*S.s.s", &benchVal},
	}
)

func Val(v interface{}) reflect.Value {
	t := reflect.TypeOf(v)
	return reflect.New(t)
}

func TestDecoder(t *testing.T) {
	for _, dt := range decoderTests {
		b, err := Marshal(dt.in)
		if err != nil {
			t.Fatalf("%15s (encode): %v", dt.name, err)
		}
		val := reflect.New(reflect.TypeOf(dt.in))
		if err := Unmarshal(b, val.Interface()); err != nil {
			t.Fatalf("%15s (decode): %v", dt.name, err)
		}
		v := val.Elem()
		if !reflect.DeepEqual(dt.in, v.Interface()) {
			if s, ok := dt.in.(S); ok {
				s.Ignore = ""
				if reflect.DeepEqual(s, v.Interface()) {
					goto OK
				}
			}
			if s, ok := dt.in.(*S); ok {
				cp := *s
				cp.Ignore = ""
				if reflect.DeepEqual(&cp, v.Interface()) {
					goto OK
				}
			}
			t.Fatalf("%15s: failed\nexp: %+v\ngot: %+v", dt.name, dt.in, v)
		}
	OK:
		t.Logf("%15s: %T(%+v)", dt.name, v, v)
	}
}

func BenchmarkDecodeMap(b *testing.B) {
	m := map[string]int{}
	for i := 0; i < 1000; i++ {
		m["x"+strconv.Itoa(i)+"x"] = i
	}
	bin, err := Marshal(m)
	if err != nil {
		b.Fatal(err)
	}
	rd := bytes.NewReader(bin)
	dec := NewDecoder(rd)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var t map[string]int
		if err := dec.Decode(&t); err != nil {
			//b.Log(t)
			b.Fatal(err)
		}
		if len(t) != len(m) {
			b.Fatal("len(t) != len(m) ")
		}
		rd.Seek(0, 0)
	}
	b.SetBytes(int64(len(bin)))
}

func benchDecoder(b *testing.B, o interface{}) {
	bin, _ := Marshal(o)
	dec := NewDecoder(nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dec.Reset(bytes.NewReader(bin))
		var s S
		if err := dec.Decode(&s); err != nil {
			b.Fatal(err)
		}
	}
	b.SetBytes(int64(len(bin)))
}

func BenchmarkDecoderBig(b *testing.B) { benchDecoder(b, &benchVal) }

func BenchmarkDecoderSmall(b *testing.B) { benchDecoder(b, benchVal.S.S.S) }
