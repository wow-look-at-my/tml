package sema

import (
	"fmt"
	"sort"
	"strings"
)

// RecordValue builds a value with named fields.
//
// It exists because a list could only ever hold strings, so anything a host
// wanted to draw per item -- a message's role, its model, its body, its
// elapsed -- had to be flattened into one pre-rendered line before it crossed
// the boundary. That put the drawing in the host and left the document with a
// list of opaque rows: a `<For>` over them can style nothing, because there is
// nothing left to style.
//
// A record does not add arithmetic or calls to the language. It adds one thing:
// an item can be asked for a named part of itself.
func RecordValue(fields map[string]Value) Value {
	copied := make(map[string]Value, len(fields))
	for name, value := range fields {
		copied[name] = value
	}
	return Value{typ: Type{Kind: KindRecord}, fields: copied}
}

// RecordListValue builds a list of records, which is what a host passes for a
// repeated structure: the messages in a transcript, the entries in a menu.
func RecordListValue(items []map[string]Value) Value {
	value := Value{typ: Type{Kind: KindRecord, IsList: true}}
	for _, item := range items {
		value.items = append(value.items, RecordValue(item))
	}
	return value
}

// Field reads one named part of a record.
//
// A field nobody declared is an ERROR naming what the record does hold, not an
// empty string. An item that silently renders blank is the failure this whole
// mechanism exists to replace.
func (v Value) Field(name string) (Value, error) {
	if v.typ.Kind != KindRecord || v.typ.IsList {
		return Value{}, fmt.Errorf("%s has no fields; only a record does", v.typ)
	}
	field, ok := v.fields[name]
	if !ok {
		return Value{}, fmt.Errorf("record has no field %q; it holds %s", name, strings.Join(v.FieldNames(), ", "))
	}
	return field, nil
}

// FieldNames lists a record's fields, sorted, for an error message a reader can
// act on.
func (v Value) FieldNames() []string {
	names := make([]string, 0, len(v.fields))
	for name := range v.fields {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
