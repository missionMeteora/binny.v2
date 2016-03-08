package binny

import (
	"bytes"
	"io"
	"sync"
)

var pools = struct {
	enc sync.Pool
	dec sync.Pool
}{
	enc: sync.Pool{
		New: func() interface{} {
			buf := bytes.NewBuffer(make([]byte, 0, DefaultEncoderBufferSize))
			return &encBuffer{b: buf, e: NewEncoder(buf)}
		},
	},
	dec: sync.Pool{
		New: func() interface{} {
			return NewDecoder(nil)
		},
	},
}

type encBuffer struct {
	b *bytes.Buffer
	e *Encoder
}

func getEncBuffer() *encBuffer {
	eb := pools.enc.Get().(*encBuffer)
	return eb
}

func putEncBuffer(eb *encBuffer) {
	eb.b.Reset()
	pools.enc.Put(eb)
}

func getDec(r io.Reader) *Decoder {
	dec := pools.dec.Get().(*Decoder)
	dec.Reset(r)
	return dec
}

func putDec(dec *Decoder) {
	dec.Reset(nil)
	pools.dec.Put(dec)
}
