// value.go holds the value code.

package sema

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/wow-look-at-my/tml/cell"
)

// Value is a typed value: a property after defaulting, an evaluated expression, or a literal attribute.
type Value struct {
	typ    Type
	str    string
	num    int
	flag   bool
	length Length
	thick  Thickness
	items  []Value
	fields map[string]Value
}

// Type reports the value's type.
func (v Value) Type() Type { return v.typ }

// String returns the underlying text of a string, color or enum value, and a rendered form of anything else.
func (v Value) String() string {
	switch {
	case v.typ.IsList:
		parts := make([]string, 0, len(v.items))
		for _, item := range v.items {
			parts = append(parts, item.String())
		}
		return strings.Join(parts, ",")
	case v.typ.Kind == KindInt:
		return strconv.Itoa(v.num)
	case v.typ.Kind == KindBool:
		return strconv.FormatBool(v.flag)
	case v.typ.Kind == KindLength:
		return v.length.String()
	case v.typ.Kind == KindThickness:
		return v.thick.String()
	default:
		return v.str
	}
}

// Int returns the value of an int property.
func (v Value) Int() int { return v.num }

// Bool returns the value of a bool property.
func (v Value) Bool() bool { return v.flag }

// Length returns the value of a length property.
func (v Value) Length() Length { return v.length }

// Thickness returns the value of a thickness property.
func (v Value) Thickness() Thickness { return v.thick }

// Items returns the elements of a list value.
func (v Value) Items() []Value { return v.items }

// Truthy reports whether the value counts as true for a conditional. Only bools, strings and lists have a defined
func (v Value) Truthy() (bool, error) {
	switch {
	case v.typ.IsList:
		return len(v.items) > 0, nil
	case v.typ.Kind == KindBool:
		return v.flag, nil
	case v.typ.Kind == KindString:
		return v.str != "", nil
	default:
		return false, fmt.Errorf("a %s has no truth value; use a bool, string or list", v.typ)
	}
}

// StringValue builds an untyped string value, which is what text interpolation and any unconstrained attribute
func StringValue(s string) Value {
	return Value{typ: Type{Kind: KindString}, str: s}
}

// BoolValue builds a bool value.
func BoolValue(b bool) Value {
	return Value{typ: Type{Kind: KindBool}, flag: b}
}

// ListValue builds a list of strings. This is how a host passes a list, and the only way that survives its own
func ListValue(items []string) Value {
	value := Value{typ: Type{Kind: KindString, IsList: true}}
	for _, item := range items {
		value.items = append(value.items, StringValue(item))
	}
	return value
}

// ParseValue reads raw as a literal of type t. A list splits on commas; an empty string is the empty list.
func ParseValue(t Type, raw string) (Value, error) {
	if t.IsList {
		element := t
		element.IsList = false

		value := Value{typ: t}
		if strings.TrimSpace(raw) == "" {
			return value, nil
		}
		for _, field := range strings.Split(raw, ",") {
			item, err := ParseValue(element, strings.TrimSpace(field))
			if err != nil {
				return Value{}, err
			}
			value.items = append(value.items, item)
		}
		return value, nil
	}

	value := Value{typ: t}
	switch t.Kind {
	case KindString:
		value.str = raw
	case KindInt:
		n, err := strconv.Atoi(raw)
		if err != nil {
			return Value{}, fmt.Errorf("expected a whole number, got %q", raw)
		}
		value.num = n
	case KindBool:
		switch raw {
		case "true":
			value.flag = true
		case "false":
			value.flag = false
		default:
			return Value{}, fmt.Errorf("expected true or false, got %q", raw)
		}
	case KindColor:
		ref, err := colorRef(raw)
		if err != nil {
			return Value{}, err
		}
		value.str = ref
	case KindLength:
		length, err := ParseLength(raw)
		if err != nil {
			return Value{}, err
		}
		value.length = length
	case KindThickness:
		thick, err := ParseThickness(raw)
		if err != nil {
			return Value{}, err
		}
		value.thick = thick
	case KindEnum:
		for _, member := range t.Enum {
			if member == raw {
				value.str = raw
				return value, nil
			}
		}
		return Value{}, fmt.Errorf("expected one of %s, got %q", strings.Join(t.Enum, ", "), raw)
	default:
		return Value{}, fmt.Errorf("unknown type %s", t)
	}
	return value, nil
}

// colorRef reads the value of a colour property as a reference to a theme colour. The value is a reference
// already, or the plain name of a theme colour. This is how a host that hands every property over as text names
// one. A hex triplet or a palette index is a colour written in place, and is refused.
func colorRef(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	if _, ok := cell.RefName(raw); ok {
		return raw, nil
	}
	_, isIndex := strconv.Atoi(raw)
	if strings.HasPrefix(raw, "#") || isIndex == nil || strings.ContainsAny(raw, " \t{}") {
		return "", fmt.Errorf(
			"a colour comes from the theme: declare <Color name=\"…\" for=\"…\"> and write {theme.name}, got %q", raw)
	}
	return cell.Ref(raw), nil
}
