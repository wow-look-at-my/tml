package syntax

// File is one parsed .tml source file. A file holds exactly one definition:
// either a component or a theme.
type File struct {
	Path      string
	Component *Component
	Theme     *Theme
}

// Component is a reusable element definition.
//
// DataTemplates are the file's <DataTemplate> declarations: the same shape as
// a component, but instantiated per item by an items control instead of being
// written as an element. They are the only nested definitions a file may hold
// — a file defines exactly one <Component>.
type Component struct {
	Name          string
	Imports       []*Import
	Properties    []*Property
	Template      *Node
	DataTemplates []*Component
	// IsData marks a <DataTemplate>: a component instantiated once per item by
	// an items control, with the item supplying its property values, rather than
	// written as an element with attributes.
	IsData bool
	Pos    Pos
}

// Import brings the definition in another file into scope under its own name.
type Import struct {
	Src string
	Pos Pos
}

// Property declares one typed input to a component. Type is kept as source text
// here; sema parses it, so an unknown type is reported with the language's own
// diagnostics rather than a syntax error.
type Property struct {
	Name       string
	Type       string
	Default    string
	HasDefault bool
	Required   bool
	Pos        Pos
}

// Theme is a set of tokens and named styles.
type Theme struct {
	Name   string
	Tokens []*Token
	Styles []*Style
	Pos    Pos
}

// Token is a named theme value. A token either has a single Value or a
// light/dark pair, never both.
type Token struct {
	Name  string
	Value string
	Light string
	Dark  string
	Pos   Pos
}

// Style is a named bundle of style attributes. Extends names another style whose
// unset attributes are inherited.
type Style struct {
	Name    string
	Extends string
	Attrs   []Attr
	Pos     Pos
}

// NodeKind distinguishes the two things that can appear in a template body.
type NodeKind int

const (
	// ElementNode is a markup element: a panel, a native element, a component
	// instance, or a language directive such as Slot or For.
	ElementNode NodeKind = iota
	// TextNode is character data, which may contain {expr} interpolations.
	TextNode
)

// Node is one entry in a template body.
//
// Name holds the element's local name, including any dot: "Stack", "Card", and
// the property-element form "Card.actions" all arrive here verbatim. Sema splits
// the dotted form; syntax does not interpret names.
type Node struct {
	Kind     NodeKind
	Name     string
	Attrs    []Attr
	Children []*Node
	Text     string
	Pos      Pos
}

// Attr is one attribute on an element. Name keeps any dot, so an attached
// property such as "Grid.row" arrives intact.
type Attr struct {
	Name  string
	Value string
	Pos   Pos
}

// Attr returns the value of the named attribute and whether it was present.
func (n *Node) Attr(name string) (string, bool) {
	for _, a := range n.Attrs {
		if a.Name == name {
			return a.Value, true
		}
	}
	return "", false
}

// Elements returns only the element children, skipping text.
func (n *Node) Elements() []*Node {
	var out []*Node
	for _, c := range n.Children {
		if c.Kind == ElementNode {
			out = append(out, c)
		}
	}
	return out
}
