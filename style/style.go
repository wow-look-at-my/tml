// Package style resolves TML style attributes into what the painter draws with: a cell style, a box model and an
// alignment. The cascade of named styles lives here.
package style

import (
	"fmt"
	"maps"
	"strings"

	"github.com/wow-look-at-my/tml/sema"
	"github.com/wow-look-at-my/tml/syntax"
)

// Sheet holds the named styles declared across every theme in scope, with each extends chain already flattened.
type Sheet struct {
	styles map[string]map[string]string
}

// NewSheet flattens the named styles from the given themes. Token references in style attributes are resolved here.
func NewSheet(themes []*syntax.Theme, tokens map[string]string) (*Sheet, error) {
	declared := map[string]*syntax.Style{}
	for _, theme := range themes {
		for _, style := range theme.Styles {
			if existing, dup := declared[style.Name]; dup {
				return nil, &syntax.Error{Pos: style.Pos, Message: fmt.Sprintf(
					"style %q is already declared at %s", style.Name, existing.Pos)}
			}
			declared[style.Name] = style
		}
	}

	sheet := &Sheet{styles: make(map[string]map[string]string, len(declared))}
	for name := range declared {
		if _, err := sheet.flatten(declared, name, tokens, nil); err != nil {
			return nil, err
		}
	}
	return sheet, nil
}

func (s *Sheet) flatten(declared map[string]*syntax.Style, name string, tokens map[string]string, stack []string) (map[string]string, error) {
	if done, ok := s.styles[name]; ok {
		return done, nil
	}
	style, ok := declared[name]
	if !ok {
		return nil, fmt.Errorf("unknown style %q", name)
	}
	for _, seen := range stack {
		if seen == name {
			return nil, &syntax.Error{Pos: style.Pos, Message: fmt.Sprintf(
				"style %q extends itself: %s", name, strings.Join(append(stack, name), " -> "))}
		}
	}

	attrs := map[string]string{}
	if style.Extends != "" {
		parent, err := s.flatten(declared, style.Extends, tokens, append(stack, name))
		if err != nil {
			if _, isSyntax := err.(*syntax.Error); isSyntax {
				return nil, err
			}
			return nil, &syntax.Error{Pos: style.Pos, Message: fmt.Sprintf("style %q: %v", name, err)}
		}
		maps.Copy(attrs, parent)
	}
	for _, attr := range style.Attrs {
		value, err := resolveTokens(attr.Value, tokens)
		if err != nil {
			return nil, &syntax.Error{Pos: style.Pos, Message: fmt.Sprintf("style %q attribute %q: %v", name, attr.Name, err)}
		}
		attrs[attr.Name] = value
	}

	s.styles[name] = attrs
	return attrs, nil
}

// resolveTokens evaluates the theme references a style attribute may contain. Styles live in a theme.
func resolveTokens(raw string, tokens map[string]string) (string, error) {
	expr, err := sema.ParseExpr(raw)
	if err != nil {
		return "", err
	}
	if expr.IsLiteral() {
		return raw, nil
	}
	var b strings.Builder
	for _, segment := range expr.Segments {
		if segment.Ref == nil {
			b.WriteString(segment.Literal)
			continue
		}
		if !segment.Ref.Theme {
			return "", fmt.Errorf("a style can only reference theme tokens, got %s", segment.Ref)
		}
		value, ok := tokens[segment.Ref.Name]
		if !ok {
			return "", fmt.Errorf("unknown theme token %q", segment.Ref.Name)
		}
		b.WriteString(value)
	}
	return b.String(), nil
}

// Resolve builds the finished style for an element: the named style when the element names a style.
func (s *Sheet) Resolve(named string, inline map[string]string) (Resolved, error) {
	attrs := map[string]string{}
	if named != "" {
		base, ok := s.styles[named]
		if !ok {
			return Resolved{}, fmt.Errorf("unknown style %q", named)
		}
		maps.Copy(attrs, base)
	}
	maps.Copy(attrs, inline)
	return build(attrs)
}

