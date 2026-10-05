package tml

// Heights is the list a Window scrolls over: how many items there are, and
// how many rows each takes.
type Heights interface {
	Len() int
	// Height is how many rows item i takes. An item of height zero draws nothing, and the window steps over it.
	Height(i int) int
}

// Window is a scroll position over a list too long to lay out whole: an item
// and a row in it. That is the bottom row on screen.
type Window struct {
	item, row int
	follow    bool
}

// NewWindow returns a window that follows the end of its list.
func NewWindow() *Window { return &Window{follow: true} }

// Following reports whether the window stays on the newest row as items arrive.
func (w *Window) Following() bool { return w.follow }

// Follow moves the window to the newest row and keeps it there.
func (w *Window) Follow() { w.follow = true }

// Slice is the part of a list a window of some height shows.
type Slice struct {
	// First and Last are the items on screen, oldest first. A list with no row to draw gives a negative First.
	First, Last int
	// Offset is how many rows of First are above the window: the offset a Scrollbox holding First to Last takes.
	Offset int
	// Above and Below report rows of the list past each edge of the window.
	Above, Below bool
}

// Scroll moves the window n rows: negative is towards the start of the list.
func (w *Window) Scroll(n int, list Heights, rows int) {
	if n == 0 || !w.resolve(list, rows) {
		return
	}
	w.move(list, n)
	w.follow = false
	w.fill(list, rows)
}

// Slice is what a window rows tall shows of list.
func (w *Window) Slice(list Heights, rows int) Slice {
	if rows <= 0 || !w.resolve(list, rows) {
		return Slice{First: -1, Last: -1}
	}
	first, used := w.item, w.row+1
	for i := w.item - 1; i >= 0 && used < rows; i-- {
		if h := list.Height(i); h > 0 {
			first, used = i, used+h
		}
	}
	s := Slice{First: first, Last: w.item, Offset: max(0, used-rows)}
	s.Above = s.Offset > 0 || prev(list, first) >= 0
	s.Below = w.row < list.Height(w.item)-1 || next(list, w.item) >= 0
	return s
}

// resolve puts the position on a row that exists, and reports whether the list has one.
func (w *Window) resolve(list Heights, rows int) bool {
	last := prev(list, list.Len())
	if last < 0 {
		return false
	}
	if w.follow || w.item > last {
		w.item, w.row = last, list.Height(last)-1
	}
	w.item = max(0, w.item)
	if list.Height(w.item) == 0 {
		if p := prev(list, w.item); p >= 0 {
			w.item, w.row = p, list.Height(p)-1
		} else {
			w.item, w.row = next(list, w.item), 0
		}
	}
	w.row = min(max(0, w.row), list.Height(w.item)-1)
	w.fill(list, rows)
	return true
}

// fill moves the position down until the rows from the start of the list to it fill the window. The window never
// ends on the oldest row above blank ones. A position that reaches the newest row follows the list again.
func (w *Window) fill(list Heights, rows int) {
	used := w.row + 1
	for i := w.item - 1; i >= 0 && used < rows; i-- {
		used += list.Height(i)
	}
	if used < rows {
		w.move(list, rows-used)
	}
	if next(list, w.item) < 0 && w.row == list.Height(w.item)-1 {
		w.follow = true
	}
}

// move steps the position n rows, a whole item at a time where it can, and stops at either end of the list.
func (w *Window) move(list Heights, n int) {
	for n < 0 {
		if w.row >= -n {
			w.row += n
			return
		}
		n += w.row + 1
		p := prev(list, w.item)
		if p < 0 {
			w.row = 0
			return
		}
		w.item, w.row = p, list.Height(p)-1
	}
	for n > 0 {
		h := list.Height(w.item)
		if w.row+n <= h-1 {
			w.row += n
			return
		}
		n -= h - w.row
		nx := next(list, w.item)
		if nx < 0 {
			w.row = h - 1
			return
		}
		w.item, w.row = nx, 0
	}
}

func prev(list Heights, i int) int {
	for i--; i >= 0; i-- {
		if list.Height(i) > 0 {
			return i
		}
	}
	return -1
}

func next(list Heights, i int) int {
	for i++; i < list.Len(); i++ {
		if list.Height(i) > 0 {
			return i
		}
	}
	return -1
}
