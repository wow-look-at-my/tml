// area.go holds area.

package widgets

import (
	"github.com/wow-look-at-my/tml/cell"
	"github.com/wow-look-at-my/tml/sema"
	"github.com/wow-look-at-my/tml/widget"
)

var areaAttrs = []string{}

// area is a region the pointer reaches and nothing else: it draws no cell and keeps no cell for itself.
type area struct{}

func newArea(widget.Context) (widget.Native, error) { return area{}, nil }

func (area) Insets() sema.Thickness { return sema.Thickness{} }

// Measure is only reached when the element is empty, and an empty area is no cells.
func (area) Measure(_, _ int) (int, int) { return 0, 0 }

func (area) Draw(cell.Canvas) {}
