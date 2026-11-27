// program_burst.go holds the layout that runs between the inputs of one burst.

package tml

// isInput reports whether a message is a value the focus ring resolves against the last layout.
func isInput(msg Msg) bool {
	switch msg.(type) {
	case Key, Mouse, Paste:
		return true
	}
	return false
}

// settle lays the view out again from the app's props and draws nothing. The
// focus ring then holds the geometry and the widget state that the earlier
// messages of the burst left.
func (p *Program) settle() error {
	p.stale = false
	p.remeasure()
	var err error
	if p.opts.Inline {
		_, err = p.view.Fit(p.app.Props(), p.width, p.limit())
	} else {
		_, err = p.view.Layout(p.app.Props(), p.width, p.height)
	}
	return err
}

// remeasure reads the screen's size. On a change it clears the screen and tells the app the new size.
func (p *Program) remeasure() {
	if !p.measure() {
		return
	}
	p.screen.Invalidate()
	p.app.Update(p, Resize{Width: p.width, Height: p.height})
}

// limit is the rows an inline program may take.
func (p *Program) limit() int {
	if p.opts.MaxHeight > 0 {
		return p.opts.MaxHeight
	}
	return p.height
}
