// +build v1

package binny

import (
	"bytes"
	"testing"

	v1 "github.com/missionMeteora/binny"
)

func benchEncoderV1(b *testing.B, o interface{}) {
	buf := bytes.NewBuffer(make([]byte, 0, 4096))
	enc := v1.NewEncoder(buf)
	b.ResetTimer()
	var ln int64
	for i := 0; i < b.N; i++ {
		if err := enc.Encode(o); err != nil {
			b.Fatal(err)
		}
		ln = int64(buf.Len())
		buf.Reset()
	}
	b.SetBytes(ln)
}

func benchDecoderV1(b *testing.B, o interface{}) {
	bin, _ := v1.Marshal(o)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dec := v1.NewDecoder(bytes.NewReader(bin))
		var s S
		if err := dec.Decode(&s); err != nil {
			b.Fatal(err)
		}
	}
	b.SetBytes(int64(len(bin)))
}

func BenchmarkDecoderV1Big(b *testing.B)   { benchDecoderV1(b, &benchVal) }
func BenchmarkDecoderV1Small(b *testing.B) { benchDecoderV1(b, benchVal.S.S.S) }

func BenchmarkEncoderV1Big(b *testing.B)   { benchEncoderV1(b, &benchVal) }
func BenchmarkEncoderV1Small(b *testing.B) { benchEncoderV1(b, benchVal.S.S.S) }
