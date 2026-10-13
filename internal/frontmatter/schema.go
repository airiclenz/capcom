package frontmatter

// Schema is the field table of one kind of file whose frontmatter Decode fills into a T: every
// key the kind knows, how it is read into a T and how it is written back. The order of the fields
// is the order Encode writes them in and the order of the Kind's Keys. A Schema is built once,
// with NewSchema, and is safe for concurrent use.
type Schema[T any] struct {
	kind   Kind
	fields []SchemaField[T]
	byKey  map[string]SchemaField[T]
}

// SchemaField is one frontmatter key of a Schema[T], built with Text, List or Raw.
type SchemaField[T any] struct {
	key    string
	decode func(value Field, node *T) error
	encode func(node T, out *Encoder)
}

// NewSchema returns the schema of the kind of file whose noun is noun ("an issue", "a note"),
// with fields in the order Encode writes them. Each field's key must be distinct.
func NewSchema[T any](noun string, fields ...SchemaField[T]) Schema[T] {
	keys := make([]string, 0, len(fields))
	byKey := make(map[string]SchemaField[T], len(fields))
	for _, field := range fields {
		keys = append(keys, field.key)
		byKey[field.key] = field
	}
	return Schema[T]{kind: Kind{Noun: noun, Keys: keys}, fields: fields, byKey: byKey}
}

// Kind returns the kind of file the schema reads: its noun and its keys in field order.
func (s Schema[T]) Kind() Kind {
	return s.kind
}

// Decode reads the file content data into node, as Decode does for the schema's Kind, handing
// each key to its field. node keeps what it held for every key the file leaves out, so the caller
// sets any default before the call. path names the file in every error. On a failure node may be
// partly filled and is not to be used.
func (s Schema[T]) Decode(path string, data []byte, node *T) (Document, error) {
	return Decode(path, data, s.kind, func(value Field) error {
		// Decode hands over only the keys of the Kind, and every one of them has a field.
		return s.byKey[value.Key].decode(value, node)
	})
}

// Encode returns the file of node in the canonical form: each field written in schema order, then
// body byte for byte.
func (s Schema[T]) Encode(node T, body string) []byte {
	var out Encoder
	for _, field := range s.fields {
		field.encode(node, &out)
	}
	return out.File(body)
}

// Text is a text key held in the string at returns. An empty value reads as "" and "" writes
// nothing.
func Text[T any](key string, at func(node *T) *string) SchemaField[T] {
	return SchemaField[T]{
		key: key,
		decode: func(value Field, node *T) (err error) {
			*at(node), err = value.Text()
			return err
		},
		encode: func(node T, out *Encoder) { out.Text(key, *at(&node)) },
	}
}

// List is a list key held in the slice at returns; the lines of its entries are dropped. An empty
// value or list reads as nil and no entries write nothing. A list whose entry lines matter is a Raw
// field.
func List[T any](key string, at func(node *T) *[]string) SchemaField[T] {
	return SchemaField[T]{
		key: key,
		decode: func(value Field, node *T) (err error) {
			*at(node), _, err = value.List()
			return err
		},
		encode: func(node T, out *Encoder) { out.List(key, *at(&node)) },
	}
}

// Raw is a key the kind reads and writes itself: decode reads the value into the node, failing
// with an error that cites its line (Field.Errorf), and encode writes it, or nothing.
func Raw[T any](
	key string,
	decode func(value Field, node *T) error,
	encode func(node T, out *Encoder),
) SchemaField[T] {
	return SchemaField[T]{key: key, decode: decode, encode: encode}
}
