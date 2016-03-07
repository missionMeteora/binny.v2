# binny.v2
Extremely simple binary Marshaler/Unmarshaler.

Due to the nature of the format, it supports streaming very well as long as both machines support the same endianness.

## Usage

### Encoding:
```
val := SomeStruct{.........}

enc := binny.NewEncoder(w)
if err := enc.Encode(&val); err != nil {
	// handle err
}
enc.Flush() // Flush is needed since we use `bufio.Writer` internally.

// or

data, err := binny.Marshal(val)
```

## Decoding
```
var val SomeStruct

dec := binny.NewDecoder(r)
if err := dec.Decode(&val); err != nil {
	// handle err
}

// or

err := binny.Unmarshal(bytes, &val)
```

## TODO

- Allow generic decoding, like json.
- Optimize Marshal/Unmarshal and use a pool.
- More tests, specifically for decoding.
- Make this read me actually readable by humans.

## Format
| type | size (bytes) |
| ---- | ---- |
| complex128 | 16 |
| int, uint | 1-8 |
| float32 | 4 |
| float64, complex64 | 8 |
| bool, struct{} | 1 |
| varint, varuint | 1-10 |

```
entry = [field-type][value]

switch(field-type) {
	case string, []byte, [...]byte:
		value = [len(v)][bytes-of-v]
	case map:
		value = [len(v)][entry(key0)][entry(v0)]...[entry(keyN)][entry(vN)][EOV]
	case slice:
		value = [len(v)][entry(idx0)]...[entry(idxN)]EOV
	case struct:
		// fields with default value / nil are omited,
		// keep that in mind if you marshal a struct and unmarshal it to a map
		value = [stringEntry(field0Name)][entry(field0Value)]...[stringEntry(fieldNameN)][entry(fieldValueN)][EOV]
	case int*, uint*:
		field-type = [smallest type to fit the value]
		value = [the value in machine-dependent-format, most likely will change to LE at one point]
}
```