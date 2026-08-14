package syntax

import "fmt"

// Namespace is the TML default namespace. It is a name, not a URL: nothing
// fetches it and it is deliberately not hosted.
const Namespace = "urn:tml:v1"

// Pos locates a construct in a source file.
//
// Line and Col come from the underlying XML parser, which records positions per
// element. Attributes do not carry their own position, so an attribute-level
// diagnostic reports the position of the element that owns it.
type Pos struct {
	File string
	Line int
	Col  int
}

func (p Pos) String() string {
	if p.File == "" {
		return fmt.Sprintf("%d:%d", p.Line, p.Col)
	}
	return fmt.Sprintf("%s:%d:%d", p.File, p.Line, p.Col)
}

// Error is a diagnostic tied to a source position. Every failure TML reports is
// one of these: the language has no warnings and no recoverable states.
type Error struct {
	Pos     Pos
	Message string
}

func (e *Error) Error() string {
	return e.Pos.String() + ": " + e.Message
}

func errorf(pos Pos, format string, args ...any) *Error {
	return &Error{Pos: pos, Message: fmt.Sprintf(format, args...)}
}
