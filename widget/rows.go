// rows.go holds Rows, the cells of a table the host hands over whole.

package widget

import (
	"reflect"
	"slices"
	"sync"

	"github.com/wow-look-at-my/tml/sema"
)

// Rows is every row of a table, as cells. The host builds it once when its
// data changes and passes the same value on every frame.
type Rows struct {
	cells [][]string

	mu sync.Mutex
	// widths is the widest cell of each column, measured by the measure that measureFor names.
	widths     []int
	measureFor uintptr
	measured   bool
	// cellWidths is the width of each row's cell, by column, measured by the measure that cellsFor names.
	cellWidths map[int][]int
	cellsFor   uintptr
	// memoKey and memo are the last thing Memo computed: one layout's worth, which a resize replaces.
	memoKey string
	memo    []int
}

// Memo returns what compute gave the last time it ran for key, and runs it
// when key is new. It keeps one answer, so a table at a new width computes
// again and the width is forgotten.
func (r *Rows) Memo(key string, compute func() []int) []int {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.memo == nil || r.memoKey != key {
		r.memoKey, r.memo = key, compute()
	}
	return r.memo
}

// NewRows takes the cells of each row. A row with fewer cells than the table has columns reads the rest as empty.
func NewRows(cells [][]string) *Rows { return &Rows{cells: cells} }

// Len is how many rows there are.
func (r *Rows) Len() int {
	if r == nil {
		return 0
	}
	return len(r.cells)
}

// Row is the cells of row i.
func (r *Rows) Row(i int) []string { return r.cells[i] }

// Value is the rows as a property value, for a host's Props.
func (r *Rows) Value() sema.Value { return sema.SourceValue(r) }

// Widths is the widest cell of each of the first n columns, over every row.
func (r *Rows) Widths(n int, measure Measurer) []int {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := reflect.ValueOf(measure).Pointer()
	if !r.measured || r.measureFor != key || len(r.widths) < n {
		r.widths = make([]int, n)
		for _, row := range r.cells {
			for col := range min(n, len(row)) {
				r.widths[col] = max(r.widths[col], measure.Width(row[col]))
			}
		}
		r.measured, r.measureFor = true, key
	}
	return slices.Clone(r.widths[:n])
}

// CellWidths is the width of column col in each row, in row order. The rows measure a column once for each measure.
// The slice is shared, so the caller must not change it.
func (r *Rows) CellWidths(col int, measure Measurer) []int {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := reflect.ValueOf(measure).Pointer()
	if r.cellsFor != key || r.cellWidths == nil {
		r.cellWidths, r.cellsFor = map[int][]int{}, key
	}
	if widths, ok := r.cellWidths[col]; ok {
		return widths
	}
	widths := make([]int, len(r.cells))
	for i, row := range r.cells {
		if col < len(row) {
			widths[i] = measure.Width(row[col])
		}
	}
	r.cellWidths[col] = widths
	return widths
}
