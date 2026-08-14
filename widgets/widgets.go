// Package widgets is the language's own widget library: the controls a terminal
// view needs before it needs anything specific to one program.
//
// Every widget here is declarative. It draws from its attributes and holds no
// state of its own, because state lives in the host's Bubble Tea model and
// arrives as properties. What the library does own is presentation: a focused
// button looks focused, a checkbox looks checked, a progress bar fills.
//
// The library is registered by default, so a template can use <Button> without
// the host binding anything. A host that wants its own <Button> binds one and
// wins the name; tml.Options.Bare drops the library entirely.
package widgets

import (
	"github.com/wow-look-at-my/tml/widget"
)

// Library returns a registry holding every built-in widget.
func Library() *widget.Registry {
	return widget.NewRegistry().
		BindFactory("Rule", factory(newRule, ruleAttrs)).
		BindFactory("ProgressBar", factory(newProgressBar, progressAttrs)).
		BindFactory("Spinner", factory(newSpinner, spinnerAttrs)).
		BindFactory("Sparkline", factory(newSparkline, sparklineAttrs)).
		BindFactory("Badge", factory(newBadge, badgeAttrs))
}

// Names lists every element the library binds, for documentation and for a host
// that wants to know what it is getting.
func Names() []string { return Library().Names() }

// builder makes one widget from one element's context.
type builder func(widget.Context) (widget.Native, error)

// factory pairs a builder with the attribute names it reads. The engine hands
// anything not on that list to the stylesheet instead, which is what lets
// <Badge label="new" bg="#f00"/> mean both things at once.
func factory(build builder, attrs []string) widget.Factory {
	return declared{build: build, attrs: attrs}
}

type declared struct {
	build builder
	attrs []string
}

func (d declared) Attributes() []string { return d.attrs }

func (d declared) Build(ctx widget.Context) (widget.Native, error) { return d.build(ctx) }
