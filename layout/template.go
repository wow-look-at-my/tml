// template.go holds the template code.

package layout

import (
	"fmt"
	"sort"

	"github.com/wow-look-at-my/tml/sema"
	"github.com/wow-look-at-my/tml/syntax"
	"github.com/wow-look-at-my/tml/widget"
)

// buildTemplate builds the element a templated widget wraps its children in, with those children inside it. The
// children move out of whatever slot they were written into: the chrome has one place for content.
func (p *pass) buildTemplate(node *sema.Node, templated widget.Templated) (*Box, error) {
	element, attrs, fallback := templated.Template()
	chrome := &sema.Node{
		Kind:  syntax.ElementNode,
		Name:  element,
		Attrs: make(map[string]sema.Value, len(attrs)),
		Pos:   node.Pos,
	}
	for name, value := range attrs {
		chrome.Attrs[name] = sema.StringValue(value)
		chrome.Order = append(chrome.Order, name)
	}
	sort.Strings(chrome.Order)

	for _, child := range node.Children {
		moved := *child
		moved.Slot = ""
		chrome.Children = append(chrome.Children, &moved)
	}
	if len(chrome.Children) == 0 && fallback != "" {
		chrome.Children = []*sema.Node{{
			Kind:     syntax.ElementNode,
			Name:     "Text",
			Attrs:    map[string]sema.Value{},
			Pos:      node.Pos,
			Children: []*sema.Node{{Kind: syntax.TextNode, Text: fallback, Pos: node.Pos}},
		}}
	}

	box, err := p.build(chrome)
	if err != nil {
		return nil, err
	}
	if box.Native == nil {
		return nil, &syntax.Error{Pos: node.Pos, Message: fmt.Sprintf(
			"<%s> is drawn as a <%s>, and no widget is bound to that name", node.Name, element)}
	}
	if err := templated.Bind(box.Native); err != nil {
		return nil, &syntax.Error{Pos: node.Pos, Message: fmt.Sprintf("<%s>: %v", node.Name, err)}
	}
	return box, nil
}
