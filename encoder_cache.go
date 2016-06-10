package binny

import (
	"encoding"
	"encoding/gob"
	"reflect"
	"sync"
)

// inspired by json
type encoderFunc func(enc *Encoder, v reflect.Value) error

var (
	encCache = struct {
		sync.RWMutex
		m map[reflect.Type]encoderFunc
	}{m: map[reflect.Type]encoderFunc{}}
)

func typeEncoder(t reflect.Type) (fn encoderFunc) {
	encCache.RLock()
	fn = encCache.m[t]
	encCache.RUnlock()
	if fn != nil {
		return
	}

	encCache.Lock()
	encCache.m[t] = nopeEnc
	encCache.Unlock()

	fn = newTypeEncoder(t, true)
	encCache.Lock()
	encCache.m[t] = fn
	if k := t.Kind(); k == reflect.Struct || k == reflect.Ptr && t.Elem().Kind() == reflect.Struct {
		updateCachedFields(true)
	}
	encCache.Unlock()
	return
}

func nopeEnc(e *Encoder, v reflect.Value) error { panic(v.Type().String() + " Fly, you fools!") }

// newTypeEncoder constructs an encoderFunc for a type.
// The returned encoder only checks CanAddr when allowAddr is true.
func newTypeEncoder(t reflect.Type, allowAddr bool) encoderFunc {
	if t.Implements(marshalerType) {
		return marshalerEncoder
	}

	if t.Implements(binaryMarshalerType) {
		return binaryMarshalerEncoder
	}

	if t.Implements(gobEncoderType) {
		return gobEncoder
	}

	if t.Kind() != reflect.Ptr && allowAddr {
		ft := reflect.PtrTo(t)
		if ft.Implements(marshalerType) {
			return addrEncoder(marshalerEncoder, newTypeEncoder(t, false))
		}
		if ft.Implements(binaryMarshalerType) {
			return addrEncoder(binaryMarshalerEncoder, newTypeEncoder(t, false))
		}
		if ft.Implements(gobEncoderType) {
			return addrEncoder(gobEncoder, newTypeEncoder(t, false))
		}
	}

	switch t.Kind() {
	case reflect.Bool:
		return boolEncoder
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return intEncoder
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return uintEncoder
	case reflect.Float32:
		return float32Encoder
	case reflect.Float64:
		return float64Encoder
	case reflect.Complex64:
		return complex64Encoder
	case reflect.Complex128:
		return complex128Encoder
	case reflect.String:
		return stringEncoder
	case reflect.Map:
		if !isNative(t.Key().Kind(), true) { // eventually will support pointer keys and structs, but not gonna happen until it's needed
			return invalidEncoder
		}
		return newMapEncoder(t)
	case reflect.Slice, reflect.Array:
		return newSliceEncoder(t.Elem())
	case reflect.Struct:
		return newStructEncoder(t)
	case reflect.Ptr:
		return ptrEncoder(newTypeEncoder(t.Elem(), false))
	case reflect.Interface:
		return ifaceEncoder
	}
	return invalidEncoder
}

func marshalerEncoder(e *Encoder, v reflect.Value) error {
	return v.Interface().(Marshaler).MarshalBinny(e)
}

func binaryMarshalerEncoder(e *Encoder, v reflect.Value) error {
	return e.WriteBinary(v.Interface().(encoding.BinaryMarshaler))
}

func gobEncoder(e *Encoder, v reflect.Value) error {
	return e.WriteGob(v.Interface().(gob.GobEncoder))
}

func stringEncoder(e *Encoder, v reflect.Value) error {
	return e.WriteString(v.String())
}

func bytesEncoder(e *Encoder, v reflect.Value) error {
	return e.WriteBytes(v.Bytes())
}

func intEncoder(e *Encoder, v reflect.Value) error {
	return e.WriteInt(v.Int())
}

func uintEncoder(e *Encoder, v reflect.Value) error {
	return e.WriteUint(v.Uint())
}

func float32Encoder(e *Encoder, v reflect.Value) error {
	return e.WriteFloat32(float32(v.Float()))
}

func float64Encoder(e *Encoder, v reflect.Value) error {
	return e.WriteFloat64(v.Float())
}

func complex64Encoder(e *Encoder, v reflect.Value) error {
	return e.WriteComplex64(complex64(v.Complex()))
}

func complex128Encoder(e *Encoder, v reflect.Value) error {
	return e.WriteComplex128(v.Complex())
}

func boolEncoder(e *Encoder, v reflect.Value) error {
	return e.WriteBool(v.Bool())
}

func emptyStructEncoder(e *Encoder, v reflect.Value) error {
	e.writeType(EmptyStruct)
	return nil
}

func ifaceEncoder(e *Encoder, v reflect.Value) error {
	v = v.Elem()
	encFunc := typeEncoder(v.Type())
	return encFunc(e, v)
}

func invalidEncoder(*Encoder, reflect.Value) error { return ErrUnsupportedType }

type sliceEncoder struct {
	t    reflect.Type
	zero func(reflect.Value) bool
}

func (se sliceEncoder) encode(e *Encoder, v reflect.Value) (err error) {
	ln := v.Len()
	e.writeType(Slice)
	e.writeLen(ln)
	enc := typeEncoder(se.t)
	for i := 0; i < ln; i++ {
		vv := v.Index(i)
		if !vv.IsValid() || se.zero(vv) { // fill the holes in an array/slice
			e.writeType(Nil)
			continue
		}
		if err = enc(e, vv); err != nil {
			return
		}
	}
	e.writeType(EOV)
	return
}

func newSliceEncoder(t reflect.Type) encoderFunc {
	if t.Kind() == reflect.Uint8 {
		return bytesEncoder
	}
	typeEncoder(t) // cache the type
	se := sliceEncoder{t: t, zero: zeroCache[t.Kind()]}
	return se.encode
}

type mapEncoder struct {
	kt, vt reflect.Type
	zero   func(reflect.Value) bool
}

func (me mapEncoder) encode(e *Encoder, v reflect.Value) (err error) {
	kenc, venc := typeEncoder(me.kt), typeEncoder(me.vt)
	keys := v.MapKeys()
	e.writeType(Map)
	e.writeLen(len(keys))
	//sort.Sort(byString(keys))
	for _, k := range keys {
		vv := v.MapIndex(k)
		if err = kenc(e, k); err != nil {
			return
		}
		if !vv.IsValid() || me.zero(vv) {
			e.writeType(Nil)
			continue
		}
		if err = venc(e, vv); err != nil {
			return
		}
	}
	e.writeType(EOV)
	return
}

type byString []reflect.Value

func (bs byString) Len() int           { return len(bs) }
func (bs byString) Swap(i, j int)      { bs[i], bs[j] = bs[j], bs[i] }
func (bs byString) Less(i, j int) bool { return bs[i].String() < bs[j].String() }

func newMapEncoder(t reflect.Type) encoderFunc {
	typeEncoder(t.Key()) // cache the type
	typeEncoder(t.Elem())
	me := mapEncoder{t.Key(), t.Elem(), zeroCache[t.Elem().Kind()]}
	return me.encode
}

type structEncoder struct {
	fields []field
}

func (se structEncoder) encode(e *Encoder, v reflect.Value) (err error) {
	e.writeType(Struct)
	for i := range se.fields {
		tf := &se.fields[i]
		tf.RLock()
		vf := indirect(fieldByIndex(v, tf.index, false))
		if !vf.IsValid() || tf.zero(vf) {
			tf.RUnlock()
			continue
		}
		e.WriteString(tf.name)
		err = tf.enc(e, vf)
		tf.RUnlock()
		if err != nil {
			return
		}
	}
	e.writeType(EOV)
	return
}

func newStructEncoder(t reflect.Type) encoderFunc {
	se := structEncoder{fields: cachedTypeFields(t)}
	if len(se.fields) == 0 {
		return emptyStructEncoder
	}
	return se.encode
}

func ptrEncoder(fn encoderFunc) encoderFunc {
	return func(e *Encoder, v reflect.Value) error {
		return fn(e, v.Elem())
	}
}
func addrEncoder(canAddrFn, noAddrFn encoderFunc) encoderFunc {
	return func(e *Encoder, v reflect.Value) error {
		if v.CanAddr() {
			return canAddrFn(e, v.Addr())
		}
		return noAddrFn(e, v)
	}
}
