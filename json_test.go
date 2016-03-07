// +build json

package binny

import (
	"bytes"
	"encoding/json"
	"testing"
)

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
