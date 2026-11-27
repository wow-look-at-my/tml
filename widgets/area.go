// area.go holds area.

package widgets

import (
	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/sema"
	"github.com/wow-look-at-my/tml/widget"
)

var areaAttrs = []string{}

// target is the Area widget: a region the pointer reaches and nothing else. It draws no cell and keeps no cell for itself.
type target struct{}

func newArea(widget.Context) (widget.Native, error) { return target{}, nil }

func (target) Insets() sema.Thickness { return sema.Thickness{} }

// Measure is only reached when the element is empty, and an empty area is no cells.
func (target) Measure(_, _ int) (int, int) { return 0, 0 }

func (target) Draw(cell.Canvas) {}
