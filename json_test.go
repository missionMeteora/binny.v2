// +build json

package binny

import (
	"bytes"
	"encoding/json"
	"strconv"
	"testing"
)

func BenchmarkJSONEncodeMap(b *testing.B) {
	m := map[string]int{}
	for i := 0; i < 1000; i++ {
		m["x"+strconv.Itoa(i)+"x"] = i
	}
	buf := bytes.NewBuffer(nil)
	enc := json.NewEncoder(buf)
	enc.Encode(m)
	b.SetBytes(int64(buf.Len()))
	buf.Reset()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := enc.Encode(m); err != nil {
			b.Fatal(err)
		}
		buf.Reset()
	}
}

func BenchmarkJSONDecodeMap(b *testing.B) {
	m := map[string]int{}
	for i := 0; i < 1000; i++ {
		m["x"+strconv.Itoa(i)+"x"] = i
	}
	bin, err := json.Marshal(m)
	if err != nil {
		b.Fatal(err)
	}
	rd := bytes.NewReader(bin)
	dec := json.NewDecoder(rd)
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

func benchEncodeJSON(b *testing.B, o interface{}) {
	buf := bytes.NewBuffer(make([]byte, 0, 4096))
	enc := json.NewEncoder(buf)
	var ln int64
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := enc.Encode(o); err != nil {
			b.Fatal(err)
		}
		ln = int64(buf.Len())
		buf.Reset()
	}
	b.SetBytes(ln)
}

func benchDecodeJSON(b *testing.B, o interface{}) {
	j, _ := json.Marshal(o)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var s S
		if err := json.NewDecoder(bytes.NewReader(j)).Decode(&s); err != nil {
			b.Fatal(err)
		}
	}
	b.SetBytes(int64(len(j)))
}

func BenchmarkEncoderJSONBig(b *testing.B)   { benchEncodeJSON(b, &benchVal) }
func BenchmarkEncoderJSONSmall(b *testing.B) { benchEncodeJSON(b, benchVal.S.S.S) }
func BenchmarkDecoderJSONBig(b *testing.B)   { benchDecodeJSON(b, &benchVal) }
func BenchmarkDecoderJSONSmall(b *testing.B) { benchDecodeJSON(b, benchVal.S.S.S) }
