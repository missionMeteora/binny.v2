//go:generate stringer -type=Type
// this will screw up the string for EOV, so fix it by hand.

package binny

import (
	"encoding"
	"encoding/gob"
	"errors"
	"reflect"
	"sort"
	"sync"
)

var (
	ErrUnsupportedType = errors.New("unsupported encoding type")

	marshalerType   = reflect.TypeOf((*Marshaler)(nil)).Elem()
	unmarshalerType = reflect.TypeOf((*Unmarshaler)(nil)).Elem()

	binaryMarshalerType   = reflect.TypeOf((*encoding.BinaryMarshaler)(nil)).Elem()
	binaryUnmarshalerType = reflect.TypeOf((*encoding.BinaryUnmarshaler)(nil)).Elem()

	gobEncoderType = reflect.TypeOf((*gob.GobEncoder)(nil)).Elem()
	gobDecoderType = reflect.TypeOf((*gob.GobDecoder)(nil)).Elem()
)

// Type represents the field type
type Type byte

const (
	Nil         Type   = iota // nil/empty type
	BoolTrue                  // true
	BoolFalse                 //false
	EmptyStruct               // struct{}
	VarInt                    // Varint 1-10 bytes
	Int8                      //
	Int16                     //
	Int32                     //
	Int64                     //
	VarUint                   // VarUint 1-10 bytes
	Uint8                     //
	Uint16                    //
	Uint32                    //
	Uint64                    //
	Float32                   //
	Float64                   //
	Complex64                 //
	Complex128                //
	String                    //
	ByteSlice                 // []byte
	Struct                    //
	Map                       //
	Slice                     // or array
	Interface                 // interface{}
	Binary                    // encoding.BinaryMarshaler/BinaryUnmarshaler
	Gob                       // encoding/gob.GobEncoder/GobDecoder
	EOV         = ^Nil        // end-of-value, *any* new types must be added before this line.
)

func isFieldType(t Type, o ...Type) bool {
	for _, ot := range o {
		if t == ot {
			return true
		}
	}
	return false
}

// this part is shamelessly ninjaed and based on the encoder/json package.
var (
	fieldCache = struct {
		sync.RWMutex
		m map[reflect.Type][]field
	}{m: map[reflect.Type][]field{}}
)

type field struct {
	name   string
	index  []int
	tagged bool
	zero   func(v reflect.Value) bool
	enc    encoderFunc
	dec    decoderFunc
	typ    reflect.Type
}

func cachedTypeFields(t reflect.Type) []field {
	fieldCache.RLock()
	f := fieldCache.m[t]
	fieldCache.RUnlock()
	if f != nil {
		return f
	}

	// Compute fields without lock.
	// Might duplicate effort but won't hold other computations back.
	f = typeFields(t)
	if f == nil {
		f = []field{}
	}

	fieldCache.Lock()
	fieldCache.m[t] = f
	fieldCache.Unlock()
	return f
}

func updateCachedFields(enc bool) {
	fieldCache.Lock()
	for _, flds := range fieldCache.m {
		for i := range flds {
			f := &flds[i]
			if enc {
				f.enc = encCache.m[f.typ]
			} else {
				f.dec = decCache.m[f.typ]
			}
		}
	}
	fieldCache.Unlock()
}

func typeFields(t reflect.Type) []field {
	// Anonymous fields to explore at the current level and the next.
	current := []field{}
	next := []field{{typ: t}}

	// Count of queued names for current level and the next.
	count := map[reflect.Type]int{}
	nextCount := map[reflect.Type]int{}

	// Types already visited at an earlier level.
	visited := map[reflect.Type]bool{}

	// Fields found.
	var fields []field

	for len(next) > 0 {
		current, next = next, current[:0]
		count, nextCount = nextCount, map[reflect.Type]int{}

		for _, f := range current {
			if visited[f.typ] {
				continue
			}
			visited[f.typ] = true

			// Scan f.typ for fields to include.
			for i := 0; i < f.typ.NumField(); i++ {
				sf := f.typ.Field(i)
				if sf.PkgPath != "" && !sf.Anonymous { // unexported
					continue
				}
				name, ignore, tagged := getTagValues(sf)
				if ignore {
					continue
				}
				index := make([]int, len(f.index)+1)
				copy(index, f.index)
				index[len(f.index)] = i

				ft := sf.Type
				if ft.Name() == "" && ft.Kind() == reflect.Ptr {
					// Follow pointer.
					ft = ft.Elem()
				}

				// Record found field and index sequence.
				if name != "" || !sf.Anonymous || ft.Kind() != reflect.Struct {
					if name == "" {
						name = sf.Name
					}
					zeroFn := zeroCache[ft.Kind()]
					if zeroFn == nil {
						zeroFn = isZero
					}
					fields = append(fields, field{
						name:   name,
						index:  index,
						typ:    ft,
						tagged: tagged,
						enc:    typeEncoder(ft),
						dec:    typeDecoder(ft),
						zero:   zeroFn,
					})
					if count[f.typ] > 1 {
						// If there were multiple instances, add a second,
						// so that the annihilation code will see a duplicate.
						// It only cares about the distinction between 1 or 2,
						// so don't bother generating any more copies.
						fields = append(fields, fields[len(fields)-1])
					}
					continue
				}

				// Record new anonymous struct to explore in next round.
				nextCount[ft]++
				if nextCount[ft] == 1 {
					next = append(next, field{name: ft.Name(), index: index, typ: ft})
				}
			}
		}
	}

	sort.Sort(byName(fields))

	// Delete all fields that are hidden by the Go rules for embedded fields,
	// except that fields with JSON tags are promoted.

	// The fields are sorted in primary order of name, secondary order
	// of field index length. Loop over names; for each name, delete
	// hidden fields by choosing the one dominant field that survives.
	out := fields[:0]
	for advance, i := 0, 0; i < len(fields); i += advance {
		// One iteration per name.
		// Find the sequence of fields with the name of this first field.
		fi := fields[i]
		name := fi.name
		for advance = 1; i+advance < len(fields); advance++ {
			fj := fields[i+advance]
			if fj.name != name {
				break
			}
		}
		if advance == 1 { // Only one field with this name
			out = append(out, fi)
			continue
		}
		dominant, ok := dominantField(fields[i : i+advance])
		if ok {
			out = append(out, dominant)
		}
	}

	fields = out
	sort.Sort(byIndex(fields))
	return fields
}

func dominantField(fields []field) (field, bool) {
	// The fields are sorted in increasing index-length order. The winner
	// must therefore be one with the shortest index length. Drop all
	// longer entries, which is easy: just truncate the slice.
	length := len(fields[0].index)
	tagged := -1 // Index of first tagged field.
	for i, f := range fields {
		if len(f.index) > length {
			fields = fields[:i]
			break
		}
		if f.tagged {
			if tagged >= 0 {
				// Multiple tagged fields at the same level: conflict.
				// Return no field.
				return field{}, false
			}
			tagged = i
		}
	}
	if tagged >= 0 {
		return fields[tagged], true
	}
	// All remaining fields have the same length. If there's more than one,
	// we have a conflict (two fields named "X" at the same level) and we
	// return no field.
	if len(fields) > 1 {
		return field{}, false
	}
	return fields[0], true
}

func fieldByIndex(v reflect.Value, index []int, setNew bool) reflect.Value {
	for _, i := range index {
		if v.Kind() == reflect.Ptr {
			if v.IsNil() {
				if setNew {
					v.Set(reflect.New(v.Type().Elem()))
				} else {
					return reflect.Value{}
				}
			}
			v = v.Elem()
		}
		v = v.Field(i)
	}
	return v
}

type byName []field

func (x byName) Len() int { return len(x) }

func (x byName) Swap(i, j int) { x[i], x[j] = x[j], x[i] }

func (x byName) Less(i, j int) bool {
	xi, xj := &x[i], &x[j]
	if xi.name != xj.name {
		return xi.name < xj.name
	}
	if xi.tagged != xj.tagged {
		return xi.tagged
	}
	if len(xi.index) != len(xj.index) {
		return len(xi.index) < len(xj.index)
	}

	for k, xik := range xi.index {
		if k >= len(xj.index) {
			return false
		}
		if xik != xj.index[k] {
			return xik < xj.index[k]
		}
	}

	return false
}

// byIndex sorts field by index sequence.
type byIndex []field

func (x byIndex) Len() int { return len(x) }

func (x byIndex) Swap(i, j int) { x[i], x[j] = x[j], x[i] }

func (x byIndex) Less(i, j int) bool {
	for k, xik := range x[i].index {
		if k >= len(x[j].index) {
			return false
		}
		if xik != x[j].index[k] {
			return xik < x[j].index[k]
		}
	}
	return len(x[i].index) < len(x[j].index)
}

func getTagValues(sf reflect.StructField) (name string, ignore, tagged bool) {
	v := sf.Tag.Get("binny")
	if v == "-" {
		return "", true, true
	}
	if len(v) > 0 {
		return v, false, true
	}
	if sf.Anonymous {
		return "", false, false
	}
	return sf.Name, false, false
}

func indirect(v reflect.Value) reflect.Value {
	for kind := v.Kind(); kind == reflect.Ptr || kind == reflect.Interface; kind = v.Kind() {
		v = v.Elem()
	}
	return v
}

var zeroCache = [...]func(v reflect.Value) bool{
	reflect.Bool:          func(v reflect.Value) bool { return !v.Bool() },
	reflect.Struct:        neverZero,
	reflect.Ptr:           isZeroPtr,
	reflect.Interface:     isZeroPtr,
	reflect.Int:           isZeroInt,
	reflect.Int8:          isZeroInt,
	reflect.Int16:         isZeroInt,
	reflect.Int32:         isZeroInt,
	reflect.Int64:         isZeroInt,
	reflect.Uint:          isZeroUint,
	reflect.Uint8:         isZeroUint,
	reflect.Uint16:        isZeroUint,
	reflect.Uint32:        isZeroUint,
	reflect.Uint64:        isZeroUint,
	reflect.Uintptr:       isZeroUint,
	reflect.Float32:       isZeroFloat,
	reflect.Float64:       isZeroFloat,
	reflect.Complex64:     isZeroComplex,
	reflect.Complex128:    isZeroComplex,
	reflect.String:        isZeroLen,
	reflect.Array:         isZeroLen,
	reflect.Slice:         isZeroLen,
	reflect.Map:           isZeroLen,
	reflect.Chan:          alwaysZero,
	reflect.Func:          alwaysZero,
	reflect.UnsafePointer: alwaysZero,
}

func neverZero(reflect.Value) bool       { return false }
func alwaysZero(reflect.Value) bool      { return true }
func isZeroPtr(v reflect.Value) bool     { return v.IsNil() }
func isZeroInt(v reflect.Value) bool     { return v.Int() == 0 }
func isZeroLen(v reflect.Value) bool     { return v.Len() == 0 }
func isZeroUint(v reflect.Value) bool    { return v.Uint() == 0 }
func isZeroFloat(v reflect.Value) bool   { return v.Float() == 0 }
func isZeroComplex(v reflect.Value) bool { return v.Complex() == 0 }

func isZero(v reflect.Value) bool {
	panic(v.Kind().String()) // if this triggers then it's a bug
}

func isNative(k reflect.Kind, supportStruct bool) bool {
	switch k {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128,
		reflect.Bool, reflect.String:
		return true
	case reflect.Struct:
		return supportStruct
	case reflect.Ptr: // too complicated

	}
	return false
}
