package binny

import (
	"bytes"
	"sync"
)

var pools = struct {
	p1  sync.Pool
	p4  sync.Pool
	enc sync.Pool
}{
	p1: sync.Pool{
		New: func() interface{} {
			return make([]byte, 1024)
		},
	},
	p4: sync.Pool{
		New: func() interface{} {
			return make([]byte, 4096)
		},
	},
	enc: sync.Pool{
		New: func() interface{} {
			buf := bytes.NewBuffer(make([]byte, 0, DefaultEncoderBufferSize))
			return &encBuffer{b: buf, e: NewEncoderSize(nil, 32)}
		},
	},
}

type encBuffer struct {
	b *bytes.Buffer
	e *Encoder
}

func getEncBuffer() *encBuffer {
	eb := pools.enc.Get().(*encBuffer)
	eb.e.Reset(eb.b)
	return eb
}

func putEncBuffer(eb *encBuffer) {
	eb.e.Reset(nil)
	eb.b.Reset()
	pools.enc.Put(eb)
}
