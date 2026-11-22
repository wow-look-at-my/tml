// Package tml loads and renders Terminal Markup Language views. A .tml file declares a reusable component: typed
package tml

import (
	"io/fs"

	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/layout"
	"github.com/wow-look-at-my/tml/render"
	"github.com/wow-look-at-my/tml/sema"
	"github.com/wow-look-at-my/tml/style"
	"github.com/wow-look-at-my/tml/syntax"
	"github.com/wow-look-at-my/tml/widget"
	"github.com/wow-look-at-my/tml/widgets"
)

// Props are the arguments passed to the entry component.
type Props = map[string]sema.Value

// Options configure loading.
type Options struct {
	// Dark selects the dark half of every adaptive theme token.
	Dark bool
	// Widgets are the host's own elements, layered over the built-in library. A name bound here wins, so a host can
	Widgets *widget.Registry
	// Bare drops the built-in widget library, leaving only the host's own bindings. A view that never uses the library
	Bare bool
	// Measure is how wide a grapheme cluster is, in cells. Nil is the Unicode default.
	Measure widget.Measurer
}

// View is a loaded, checked view ready to render.
type View struct {
	program *sema.Program
	engine  *layout.Engine
	dark    bool
	ui      *UI
	measure widget.Measurer
	theme   cell.Palette
	// frames is the last painted frame, kept only while an inspector is attached. A view without an inspector pays a single atomic
	frames *frameRecord
}

// Theme is the colours of the view's themes for the mode it was loaded in, by name.
func (v *View) Theme() cell.Palette { return v.theme }

// UI is the view's interaction state: which element has focus, which element the pointer is over, and what the last
func (v *View) UI() *UI { return v.ui }

// Load parses, checks and prepares the view rooted at entry. Everything that can fail without knowing the caller's
func Load(fsys fs.FS, entry string, opts Options) (*View, error) {
	unit, err := syntax.Load(fsys, entry)
	if err != nil {
		return nil, err
	}
	// The base theme comes first, so a theme of the view can replace one of its colours.
	unit.Themes = append([]*syntax.Theme{style.Base()}, unit.Themes...)
	registry := opts.Widgets
	if !opts.Bare {
		registry = opts.Widgets.Merge(widgets.Library())
	}
	program, err := sema.Analyze(unit, sema.Options{
		Natives: registry.Names(),
		Slots:   registry.SlotNames(),
	})
	if err != nil {
		return nil, err
	}
	sheet, err := style.NewSheetFor(unit.Themes, program.Tokens(sema.ExpandOptions{Dark: opts.Dark}), opts.Dark)
	if err != nil {
		return nil, err
	}
	view := &View{program: program, dark: opts.Dark, ui: NewUI(), measure: opts.Measure, theme: sheet.Palette()}
	view.engine = layout.New(sheet, layout.Options{
		Widgets:     registry,
		FS:          fsys,
		Dark:        opts.Dark,
		Interaction: view.ui,
		Measure:     opts.Measure,
	})
	// Every view this library builds is inspectable, with nothing asked of the caller, and a view that cannot be is not
	if err := inspection.adopt(view); err != nil {
		return nil, err
	}
	return view, nil
}

// Expand instantiates the view and returns the expanded element tree, with components, slots and control flow resolved
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

// Draw lays the view out in a viewport and paints it. The frame is a grid of cells: a Program puts it on the terminal,
// and anything else reads it.
func (v *View) Draw(props Props, width, height int) (*cell.Buffer, error) {
	box, err := v.Layout(props, width, height)
	if err != nil {
		return nil, err
	}
	return v.paint(box, width, height), nil
}

// Render is Draw as the text a terminal draws: one line per row, styled with SGR.
func (v *View) Render(props Props, width, height int) (string, error) {
	frame, err := v.Draw(props, width, height)
	if err != nil {
		return "", err
	}
	return frame.String(), nil
}

// Fit instantiates the view and lays it out at the given width, as tall as its content, up to maxHeight.
func (v *View) Fit(props Props, width, maxHeight int) (*layout.Box, error) {
	node, err := v.Expand(props)
	if err != nil {
		return nil, err
	}
	return v.engine.Fit(node, width, maxHeight)
}

// ItemHeight is how many rows one item of the named data template takes at width. It counts the rows the engine draws,
// so a host that windows a long list never keeps a second count of its own. See Window.
func (v *View) ItemHeight(template string, item sema.Value, width int) (int, error) {
	node, err := v.program.ExpandItem(template, item, sema.ExpandOptions{Dark: v.dark})
	if err != nil {
		return 0, err
	}
	return v.engine.Height(node, width)
}

// DrawFit paints a frame as tall as its content, up to maxHeight, rather than one that fills a viewport. A region
// drawn inline under a shell prompt wants this. See Live.
func (v *View) DrawFit(props Props, width, maxHeight int) (*cell.Buffer, error) {
	box, err := v.Fit(props, width, maxHeight)
	if err != nil {
		return nil, err
	}
	return v.paint(box, width, box.Rect.H), nil
}

// RenderFit is DrawFit as the text a terminal draws.
func (v *View) RenderFit(props Props, width, maxHeight int) (string, error) {
	frame, err := v.DrawFit(props, width, maxHeight)
	if err != nil {
		return "", err
	}
	if frame.H == 0 {
		return "", nil
	}
	return frame.String(), nil
}

// Advance moves the view's animations on to their next step.
func (v *View) Advance() { v.engine.Advance() }

// Animating reports whether the last layout left something animating on screen.
func (v *View) Animating() bool { return v.engine.Animating() }

func (v *View) paint(box *layout.Box, width, height int) *cell.Buffer {
	frame := render.Frame(box, width, height, v.measure)
	v.ui.drawTooltip(frame, v.theme, v.measure)
	v.record(box, frame, width, height)
	// Readable is half.
	drives.painted(inspection.isDriven())
	return frame
}
