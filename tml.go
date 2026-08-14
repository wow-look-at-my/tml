// Package tml loads and renders Terminal Markup Language views.
//
// A .tml file declares a reusable component: typed properties, slots for
// injected content, layout panels and styling. TML solves the layout
// constraints and Lip Gloss renders the result, so a view drops straight into a
// Bubble Tea View().
package tml

import (
	"io/fs"

	"github.com/wow-look-at-my/tml/layout"
	"github.com/wow-look-at-my/tml/render"
	"github.com/wow-look-at-my/tml/sema"
	"github.com/wow-look-at-my/tml/style"
	"github.com/wow-look-at-my/tml/syntax"
)

// Props are the arguments passed to the entry component.
type Props = map[string]sema.Value

// Options configure loading.
type Options struct {
	// Dark selects the dark half of every adaptive theme token.
	Dark bool
	// Natives are host-registered element names, on top of the builtins.
	Natives []string
}

// View is a loaded, checked view ready to render.
type View struct {
	program *sema.Program
	engine  *layout.Engine
	dark    bool
}

// Load parses, checks and prepares the view rooted at entry.
//
// Everything that can fail without knowing the caller's arguments fails here:
// malformed XML, unknown elements, bad types, unresolved references. Rendering
// afterwards can still fail, but only on things that genuinely depend on the
// arguments.
func Load(fsys fs.FS, entry string, opts Options) (*View, error) {
	unit, err := syntax.Load(fsys, entry)
	if err != nil {
		return nil, err
	}
	program, err := sema.Analyze(unit, sema.Options{Natives: opts.Natives})
	if err != nil {
		return nil, err
	}
	sheet, err := style.NewSheet(unit.Themes, program.Tokens(sema.ExpandOptions{Dark: opts.Dark}))
	if err != nil {
		return nil, err
	}
	return &View{program: program, engine: layout.New(sheet), dark: opts.Dark}, nil
}

// Expand instantiates the view and returns the expanded element tree, with
// components, slots and control flow resolved away.
func (v *View) Expand(props Props) (*sema.Node, error) {
	return v.program.Expand(props, sema.ExpandOptions{Dark: v.dark})
}

// Layout instantiates the view and lays it out in a viewport.
func (v *View) Layout(props Props, width, height int) (*layout.Box, error) {
	node, err := v.Expand(props)
	if err != nil {
		return nil, err
	}
	return v.engine.Layout(node, width, height)
}

// Render produces the terminal output for a viewport. The result is a styled
// string ready to hand to tea.NewView.
func (v *View) Render(props Props, width, height int) (string, error) {
	box, err := v.Layout(props, width, height)
	if err != nil {
		return "", err
	}
	return render.Render(box), nil
}
