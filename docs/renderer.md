# The renderer

TML draws by itself. Layout produces rects. The painter fills a grid of cells from them. The screen writer turns the cells that changed into what a terminal reads. Nothing in between is a string.

```text
sema  ->  layout (rects)  ->  render.Paint (cells)  ->  term.Screen (escape sequences)
                                  |
                                  +->  cell.Buffer.String()   one frame as text, for a test or a pipe
```

## Cells

`cell.Buffer` is a frame: a grid of `cell.Cell`, each a grapheme cluster, its width and its style. A wide glyph owns its cell and the one after it. A write that lands on half of a wide glyph blanks the other half, so a row never holds half a character.

`cell.Style` is a foreground, a background, a set of attributes and a hyperlink. Every field can be unset. An unset field shows what is beneath it, which is the whole cascade:

```go
upper.Over(base)   // what upper states wins; the rest comes from base
```

A colour is a palette index, an RGB value or a terminal colour name. `cell.ParseColor` refuses anything else, so a typo in a theme fails the load.

## The canvas

A widget draws on a `cell.Canvas`: a region of the buffer with its own origin, a clip, and the style it inherited.

```go
c.Text(x, y, "label", cell.Style{}.Bold(true))
c.Fill(x, y, w, h, cell.Style{Bg: colour})
c.Box(0, 0, w, h, border, cell.Style{Fg: colour})
c.Restyle(x, y, w, h, func(drawn cell.Cell) cell.Style { ... })   // change styles, keep glyphs
c.Sub(x, y, w, h)                                                  // a region as a canvas of its own
c.Place(x, y, w, h, escape, clear)                                 // an inline image over a region
```

A draw outside the clip is dropped. A glyph drawn with no background keeps the background already on the cell, which is how text sits on a fill. `Text` reads SGR sequences in what it is given, so output that is already coloured draws in its colours.

## The painter

`render.Paint` walks the laid-out tree and draws every box on the rect layout gave it. What the screen shows and what layout published cannot disagree, because both are the same rect.

- An element's `fg` and text attributes reach its descendants. Its `bg` fills its padding box, inside the border.
- Siblings paint in document order. On a `Canvas` and a `Grid`, a child covers what beneaths its rect, blank cells included, which is what makes a popup a popup.
- A `Text` wraps at its width, or cuts each line when `overflow` says so. `align` and `valign` place its lines.
- A widget draws on a canvas the size of its content box. A composer's children are clipped to the inside of its insets, which is also what confines scrolled content to its viewport.
- A hidden element paints nothing and keeps its slot. A collapsed one takes no slot.

## Measuring

Width is counted per grapheme cluster. `Options.Measure` replaces the Unicode default for a host that asked its terminal, and the same answer governs layout and paint. An escape sequence takes no cells.

Wrapping breaks at spaces and after hyphens, and inside a word only when the word is wider than a row. Layout counts rows with the same call the painter draws with.

## The screen writer

`term.Screen` keeps the last frame. A full-screen draw writes only the runs of cells that changed, each after one cursor move. An inline draw rewrites only the rows that changed, and returns the cursor to the region's first row. Both sit inside one synchronized update, so a terminal that supports it never shows half a frame.

Colours follow the profile of the terminal. An RGB colour becomes the nearest palette colour, or the nearest base colour, where the terminal shows no more. `NO_COLOR` keeps the attributes and drops the colours.

An image is not a cell. A widget places its escape sequence over a region. The writer sends it when the set of images changes, after it clears the ones.

## Input

`term.Decoder` turns the bytes a terminal sends. It reads the legacy sequences, the modified forms, the kitty form and SGR pointer reports. A sequence that arrives in pieces waits for its rest. A lone ESC becomes the escape key once nothing followed it.

## Text for a test

`View.Render` and `cell.Buffer.String` give a frame as text with SGR. `cell.Buffer.Text` gives it without styles. A test that asks about a style reads the cell:

```go
frame, _ := view.Draw(props, 40, 10)
frame.At(2, 1).Style.Has(cell.Bold)
```
