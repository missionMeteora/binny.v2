package binny

import (
	"encoding"
	"encoding/gob"
	"fmt"
	"reflect"
	"sync"
)

type DecoderTypeError struct {
	Expected string
	Actual   Type
}

func (dte DecoderTypeError) Error() string {
	return "expected " + dte.Expected + ", got " + dte.Actual.String()
}

type decoderFunc func(dec *Decoder, v reflect.Value) error

var (
	decCache = struct {
		sync.RWMutex
		m map[reflect.Type]decoderFunc
	}{m: map[reflect.Type]decoderFunc{}}
)

func typeDecoder(t reflect.Type) (fn decoderFunc) {
	decCache.RLock()
	fn = decCache.m[t]
	decCache.RUnlock()
	if fn != nil {
		return
	}

	decCache.Lock()
	decCache.m[t] = nopeDec
	decCache.Unlock()

	fn = newTypeDecoder(t)
	decCache.Lock()
	decCache.m[t] = fn
	if k := t.Kind(); k == reflect.Struct || k == reflect.Ptr && t.Elem().Kind() == reflect.Struct {
		updateCachedFields(false)
	}
	decCache.Unlock()
	return
}

func nopeDec(*Decoder, reflect.Value) error { panic("Fly, you fools!") }

func newTypeDecoder(t reflect.Type) decoderFunc {
	k := t.Kind()
	if t.Implements(marshalerType) {
		if k == reflect.Ptr {
			return newPtrDecoder(unmarshalerDecoder, false)
		}
		return unmarshalerDecoder
	}

	if t.Implements(binaryMarshalerType) {
		if k == reflect.Ptr {
			return newPtrDecoder(binaryUnmarshalerDecoder, false)
		}
		return binaryUnmarshalerDecoder
	}
	if t.Implements(gobDecoderType) {
		if k == reflect.Ptr {
			return newPtrDecoder(gobDecoder, false)
		}
		return gobDecoder
	}

	if k != reflect.Ptr {
		t := reflect.PtrTo(t)
		if t.Implements(marshalerType) {
			return newTypeDecoder(t)
		}
		if t.Implements(binaryMarshalerType) {
			return newTypeDecoder(t)
		}
		if t.Implements(gobEncoderType) {
			return newTypeDecoder(t)
		}
	}
	switch k {
	case reflect.Bool:
		return boolDecoder
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return intDecoder
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return uintDecoder
	case reflect.Float32:
		return float32Decoder
	case reflect.Float64:
		return float64Decoder
	case reflect.Complex64:
		return complex64Decoder
	case reflect.Complex128:
		return complex128Decoder
	case reflect.String:
		return stringDecoder
	case reflect.Map:
		if !isNative(t.Key().Kind(), true) { // eventually will support pointer keys and structs, but not gonna happen until it's needed
			return invalidDecoder
		}
		return newMapDecoder(t)
	case reflect.Slice, reflect.Array:
		return newSliceDecoder(t.Elem())
	case reflect.Struct:
		return newStructDecoder(t)
	case reflect.Ptr:
		return newPtrDecoder(typeDecoder(t.Elem()), true)
		// case reflect.Interface:
		// 	return ifaceEncoder
	}
	return invalidDecoder
}

func unmarshalerDecoder(d *Decoder, v reflect.Value) error {
	return v.Interface().(Unmarshaler).UnmarshalBinny(d)
}

func binaryUnmarshalerDecoder(d *Decoder, v reflect.Value) error {
	return d.ReadBinary(v.Interface().(encoding.BinaryUnmarshaler))
}

func gobDecoder(d *Decoder, v reflect.Value) error {
	return d.ReadGob(v.Interface().(gob.GobDecoder))
}

func boolDecoder(d *Decoder, v reflect.Value) error {
	b, err := d.ReadBool()
	v.SetBool(b)
	return err
}

func intDecoder(d *Decoder, v reflect.Value) error {
	i, _, err := d.ReadInt()
	v.SetInt(i)
	return err
}

func uintDecoder(d *Decoder, v reflect.Value) error {
	i, _, err := d.ReadUint()
	v.SetUint(i)
	return err
}

func float32Decoder(d *Decoder, v reflect.Value) error {
	f, err := d.ReadFloat32()
	v.SetFloat(float64(f))
	return err
}

func float64Decoder(d *Decoder, v reflect.Value) error {
	f, err := d.ReadFloat64()
	v.SetFloat(f)
	return err
}

func complex64Decoder(d *Decoder, v reflect.Value) error {
	c, err := d.ReadComplex64()
	v.SetComplex(complex128(c))
	return err
}

func complex128Decoder(d *Decoder, v reflect.Value) error {
	c, err := d.ReadComplex128()
	v.SetComplex(c)
	return err
}

func stringDecoder(d *Decoder, v reflect.Value) error {
	s, err := d.ReadString()
	v.SetString(s)
	return err
}

func bytesDecoder(d *Decoder, v reflect.Value) error {
	b, err := d.ReadBytes()
	v.SetBytes(b)
	return err
}

func invalidDecoder(d *Decoder, v reflect.Value) error {
	return fmt.Errorf("%v is not supported", v.Type().String())
}

type sliceDecoder struct {
	dec decoderFunc
}

func (sd sliceDecoder) decode(d *Decoder, v reflect.Value) error {
	if err := d.expectType(Slice); err != nil {
		return err
	}

	ln, _, err := d.ReadUint()

	if err != nil {
		return err
	}

	if v.Kind() == reflect.Slice {
		if ln := int(ln); v.Cap() < ln {
			v.Set(reflect.MakeSlice(v.Type(), ln, ln))
		} else {
			v.SetLen(ln)
			v.SetCap(ln)
		}
	}

	for i := 0; i < int(ln); i++ {
		// this is a bug
		if err = sd.dec(d, v.Index(i)); err != nil {
			return err
		}
	}

	return d.expectType(EOV)
}

func newSliceDecoder(t reflect.Type) decoderFunc {
	if t.Kind() == reflect.Uint8 {
		return bytesDecoder
	}
	d := sliceDecoder{dec: typeDecoder(t)}
	return d.decode
}

type structDecoder struct {
	fields map[string]*field
}

func (sd structDecoder) decode(d *Decoder, v reflect.Value) error {
	if err := d.expectType(Struct); err != nil {
		return err
	}
	for {
		n, err := d.ReadString()
		if err != nil {
			if err, ok := err.(DecoderTypeError); ok && err.Actual == EOV {
				return nil
			}
			return err
		}
		if f, ok := sd.fields[n]; ok {
			fld := fieldByIndex(v, f.index, true)
			if err := f.dec(d, fld); err != nil {
				return err
			}
		}
	}
}

func newStructDecoder(t reflect.Type) decoderFunc {
	var (
		flds = cachedTypeFields(t)
		sd   = structDecoder{fields: make(map[string]*field, len(flds))}
	)
	for i := range flds {
		f := &flds[i] // need a pointer so when we later update fields .dec it'd get picked up.
		sd.fields[f.name] = f
	}
	return sd.decode
}

type mapDecoder struct {
	kdec, vdec decoderFunc
}

func (md mapDecoder) decode(d *Decoder, v reflect.Value) error {
	if err := d.expectType(Map); err != nil {
		return err
	}
	ln, _, err := d.ReadUint()

	if err != nil {
		return err
	}

	t := v.Type()
	if v.IsNil() {
		v.Set(reflect.MakeMap(t))
	}

	for i, kt, vt := 0, t.Key(), t.Elem(); i < int(ln); i++ {
		key := reflect.New(kt).Elem()
		if err = md.kdec(d, key); err != nil {
			return err
		}
		val := reflect.New(vt).Elem()
		if err = md.vdec(d, val); err != nil {
			return err
		}
		v.SetMapIndex(key, val)
	}
	return d.expectType(EOV)
}

func newMapDecoder(t reflect.Type) decoderFunc {
	md := mapDecoder{typeDecoder(t.Key()), typeDecoder(t.Elem())}
	return md.decode
}

type ptrDecoder struct {
	dec decoderFunc
}

func (pd ptrDecoder) decodeElem(d *Decoder, v reflect.Value) error {
	if v.IsNil() {
		v.Set(reflect.New(v.Type().Elem()))
	}
	return pd.dec(d, v.Elem())
}

func (pd ptrDecoder) decode(d *Decoder, v reflect.Value) error {
	if v.IsNil() {
		v.Set(reflect.New(v.Type().Elem()))
	}
	return pd.dec(d, v)
}

func newPtrDecoder(fn decoderFunc, elem bool) decoderFunc {
	pd := ptrDecoder{fn}
	if elem {
		return pd.decodeElem
	}
	return pd.decode
}
