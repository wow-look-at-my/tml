// items.go holds the items code.

package sema

import (
	"fmt"
	"strconv"

	"github.com/wow-look-at-my/tml/syntax"
)

// Both attributes that make an element an items control. They are DIRECTIVES, not widget properties: they never
const (
	attrItemsSource  = "itemsSource"
	attrItemTemplate = "itemTemplate"
)

// isItemsControl reports whether this element repeats a template over a list.
func isItemsControl(node *tnode) bool {
	for _, attr := range node.attrs {
		if attr.name == attrItemsSource || attr.name == attrItemTemplate {
			return true
		}
	}
	return false
}

// expandItems builds a single instance of a data template per item.
func (p *Program) expandItems(node *tnode, scope *evalScope, file string, stack []string) ([]*Node, error) {
	var source *Expr
	var template string
	for _, attr := range node.attrs {
		switch attr.name {
		case attrItemsSource:
			source = attr.expr
		case attrItemTemplate:
			if !attr.expr.IsLiteral() {
				return nil, &syntax.Error{Pos: attr.pos, Message: fmt.Sprintf(
					"%s names a template and cannot be an expression", attrItemTemplate)}
			}
			template = attr.expr.Source
		}
	}
	if source == nil {
		return nil, &syntax.Error{Pos: node.pos, Message: fmt.Sprintf(
			"<%s> has %s but no %s: there is nothing to repeat", node.name, attrItemTemplate, attrItemsSource)}
	}
	if template == "" {
		return nil, &syntax.Error{Pos: node.pos, Message: fmt.Sprintf(
			"<%s> has %s but no %s: there is nothing to draw an item with", node.name, attrItemsSource, attrItemTemplate)}
	}

	compiledTemplate, ok := p.resolveComponent(file, template)
	if !ok {
		return nil, &syntax.Error{Pos: node.pos, Message: fmt.Sprintf(
			"unknown %s %q", attrItemTemplate, template)}
	}
	if !compiledTemplate.def.IsData {
		return nil, &syntax.Error{Pos: node.pos, Message: fmt.Sprintf(
			"%q is a <Component>, not a <DataTemplate>; only a data template takes its values from an item", template)}
	}

	items, err := source.Eval(scope)
	if err != nil {
		return nil, &syntax.Error{Pos: node.pos, Message: fmt.Sprintf("%s: %v", attrItemsSource, err)}
	}
	if !items.Type().IsList {
		return nil, &syntax.Error{Pos: node.pos, Message: fmt.Sprintf(
			"%s needs a list, got %s", attrItemsSource, items.Type())}
	}
	if items.Source() != nil {
		return nil, &syntax.Error{Pos: node.pos, Message: attrItemsSource + " cannot repeat a list the host keeps; give it to a widget that reads one, such as a Table"}
	}

	var out []*Node
	for i, item := range items.Items() {
		args, err := itemArgs(item)
		if err != nil {
			return nil, &syntax.Error{Pos: node.pos, Message: fmt.Sprintf(
				"%s item %d: %v", attrItemsSource, i, err)}
		}
		// The position is offered only to a template that asked for it. Handing it to every template would make an ordinary
		if _, declared := compiledTemplate.props["index"]; declared {
			position, err := ParseValue(Type{Kind: KindInt}, strconv.Itoa(i))
			if err != nil {
				return nil, err
			}
			args["index"] = position
		}
		inner, err := p.bindProps(compiledTemplate, args, scope.tokens, node.pos)
		if err != nil {
			return nil, err
		}
		expanded, err := p.expandNodes(compiledTemplate.body, inner, compiledTemplate.file,
			&slotArgs{scope: scope, file: file}, append(stack, compiledTemplate.def.Name))
		if err != nil {
			return nil, err
		}
		out = append(out, expanded...)
	}
	return out, nil
}

// ExpandItem instantiates one item of the named data template, as an items control repeats it. A host that lays out a
// list in windows measures each item through it, so the rows it counts are the rows the engine draws.
func (p *Program) ExpandItem(template string, item Value, opts ExpandOptions) (*Node, error) {
	var found *compiled
	for _, c := range p.components {
		if !c.def.IsData || c.def.Name != template {
			continue
		}
		if found != nil {
			return nil, fmt.Errorf("data template %q is declared in both %s and %s; name one file's", template, found.file, c.file)
		}
		found = c
	}
	if found == nil {
		return nil, fmt.Errorf("no data template is named %q", template)
	}
	tokens, err := p.resolveTokens(opts)
	if err != nil {
		return nil, err
	}
	args, err := itemArgs(item)
	if err != nil {
		return nil, err
	}
	scope, err := p.bindProps(found, args, tokens, found.def.Pos)
	if err != nil {
		return nil, err
	}
	children, err := p.expandNodes(found.body, scope, found.file, nil, []string{found.def.Name})
	if err != nil {
		return nil, err
	}
	// A vertical stack is what an items control lays its instances out in.
	return &Node{Kind: syntax.ElementNode, Name: "Stack", Attrs: map[string]Value{}, Children: children, Pos: found.def.Pos}, nil
}

// itemArgs is a single item's fields, as the values its template's properties take. A record supplies them by name. A plain
func itemArgs(item Value) (map[string]Value, error) {
	if item.Type().Kind != KindRecord {
		return map[string]Value{"value": item}, nil
	}
	args := make(map[string]Value, len(item.fields))
	for _, name := range item.FieldNames() {
		field, err := item.Field(name)
		if err != nil {
			return nil, err
		}
		args[name] = field
	}
	return args, nil
}
