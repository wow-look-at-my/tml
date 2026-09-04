# Widgets

TML's language is layout: `Stack`, `Grid`, `Canvas`, `Box`, `Text`, `Spacer`. Everything else on screen is a widget, and
widgets come from a registry. The library in `widgets/` is registered by default, so a template can use `<Button>` with
no setup; a host that binds its own `Button` wins the name, and `Options{Bare: true}` drops the library entirely.

The split is that **widgets draw and panels solve constraints**. A widget is handed a size and returns a string. That is
the whole contract, and every widget below goes through it -- including the ones that wrap children.

## Interaction

Two attributes work on any element:

- `id` names it. An id is how focus survives from one frame to the next, and it is how the host tells one control from
  another. Two elements sharing an id is an error where it is written.
- `action` is the string the element reports when it is activated. The host matches on it, so its `Update` never learns
  where anything ended up.

```go
for _, event := range m.view.UI().Update(msg) {
    switch event.Kind {
    case tml.Activated:
        m.act(event.Action)
    case tml.Scrolled:
        m.scroll(event.ID, event.Delta)
    }
}
```

`Update` takes any Bubble Tea message and ignores the ones that mean nothing here, so forwarding everything is safe. Tab
and the arrows move the focus ring, enter and space activate, and the pointer hovers, clicks and scrolls. Replace the
bindings with `view.UI().SetKeyMap(...)` when the host wants a key back.

Mouse events only arrive if the program asks for them. In Bubble Tea v2 that is a property of the view:

```go
view := tea.NewView(m.frame())
view.MouseMode = tea.MouseModeAllMotion  // cell motion only reports movement while a button is held
```

A control is reachable two ways, and they are not the same set:

- the **focus ring** is the widgets that take focus. Tab steps through them, in document order.
- the **pointer** reaches those plus anything else carrying an id, which is how the wheel finds a `Scrollbox` that is
  not a tab stop.

A disabled control is in neither: it is left out of the frame's geometry entirely, so a click where it is drawn does
nothing at all.

One widget can draw many things — a list's rows, a table's columns — and the ring still sees one element. A pointer
event carries `X` and `Y` in cells from that element's own corner, which is how the host tells them apart; both are `-1`
when the keyboard did it.

```go
if event.ID == "services" && event.Y >= 0 {
    m.selected = event.Y
}
```

### Reading the frame back

`view.UI().Target(id)` is where an element ended up in the last frame: its rect, and for a scrolling region, how far it
actually scrolled.

That last number matters because the host cannot work it out. How far a `Scrollbox`'s content runs depends on how the
text inside it wrapped at the width the widget was given, so "the bottom" is only knowable once the frame exists. A host
that wants to follow a growing transcript asks to go further than there is -- scrolling clamps to the content, in the
drawing and in the geometry alike -- and reads back where that landed:

```go
m.offset = 1 << 20                                  // the end, wherever it is
...
if session, ok := m.view.UI().Target("session"); ok {
    m.offset = min(session.Scroll.Y+event.Delta, session.Scroll.MaxY)   // one notch off it
}
```

Focus is a rendering input, not something a widget stores. The engine hands each widget its `widget.State` before
measuring, so a control that grows when focused is measured at the size it will actually draw at.

## The library

Every widget also takes the style attributes (`fg`, `bg`, `bold`, `padding`, `border`, …); anything a widget does not
claim goes to the stylesheet, so `<Badge label="new" bg="#f00"/>` means both things at once.

### Containers

| Element | Attributes | Notes |
| --- | --- | --- |
| `Border` | `kind`, `title`, `titleAlign`, `color`, `pad` | A frame around its children. `kind` is a lipgloss border name; the title is spliced into the top edge. |
| `Popup` | as `Border` | A dialog. Defaults to a rounded border and centres itself on a `Canvas`. |
| `Scrollbox` | `offset`, `offsetX`, `scrollbar`, `contentHeight` | A viewport onto taller content. The host owns the offset; the wheel reports `Scrolled` and the host moves it. `scrollbar` is `auto`, `always` or `never`. `contentHeight` says the children are a window over content that long -- see below. |

`Scrollbox` is where the pointer-only rule earns itself: it never takes focus, and clipping is real -- a control
scrolled out of view is not in the frame's geometry, so it cannot be clicked where it used to be.

**`contentHeight` is for content too long to lay out.** Laying out a whole conversation to show fifty rows of it costs
what the conversation costs, every frame: measured at 200x50, a transcript of 10 000 rows is 758 ms and one of 1 000 is
80 ms, against a 16 ms frame. Hand over the rows on screen instead and say how many there are altogether:

```xml
<Scrollbox id="transcript" offset="{scroll}" contentHeight="{lines}">
```

The same measurement windowed is **7.2 ms and flat** -- 7.1 at 100 rows, 7.2 at 1 000, 7.5 at 10 000 -- because the cost
is now the viewport rather than the content. In this mode the children are drawn from the top, since the host already cut
at the offset; `offset` remains the position, and both the scrollbar and the maximum reported through `UI().Target`
describe the whole content, which is the only reason to be told its size. Slicing is the host's: it owns the content, so
only it can say which rows the offset selects.

### Controls

| Element | Attributes | Notes |
| --- | --- | --- |
| `Button` | `label`, `variant`, `disabled` | `variant` is `default`, `primary` or `danger`. Focused, hovered and pressed each look different. |
| `Textbox` | `value`, `placeholder`, `cursor`, `disabled`, `password` | Draws a field; it does not edit one. Editing is state, and state is the host's. |
| `Checkbox` | `label`, `checked`, `disabled` | `[x]` / `[ ]`. |
| `Radio` | `label`, `checked`, `disabled` | `(•)` / `( )`. The host owns which one is on. |

A button's label is sugar for its `Content` slot, so anything can go inside one:

```xml
<Button id="save" action="save" variant="primary">
    <Button.Content>
        <Stack orientation="horizontal" gap="1">
            <Text>Save</Text>
            <Badge label="2"/>
        </Stack>
    </Button.Content>
</Button>
```

### Data

| Element | Attributes | Notes |
| --- | --- | --- |
| `List` | `items`, `selected`, `cursor`, `disabled` | A cursor beside the selected row. |
| `Table` | `columns`, `rows`, `separator`, `border`, `selected`, `disabled`, `wrap` | Rows are `separator`-joined cells; columns size to their widest. |

Both take focus, and both leave the selection to the host: `selected` is a row index the model owns, so nothing in the
widget can disagree with what the program thinks is picked. A `Table` marks its row in place -- a reversed bar across
every column, bold while the table has focus -- rather than in a cursor gutter, since a row of columns has no single
place to put one. The bar is drawn whether or not the table has focus, because the row a keystroke acts on is the host's
state and does not stop being picked while a filter box holds the ring. The header is not a row anyone can select, and
`selected="-1"` marks nothing.

Which row a click landed on arrives the same way a `List`'s does, in the event's own coordinates. A table draws its
header first, so its rows begin a line below:

```go
if event.ID == "steps" && event.Y >= 0 {
    m.step = clamp(event.Y-1, 0, len(steps)-1)
}
```

That arithmetic holds only while a row is a line. A cell too wide for its column wraps by default and takes the next
line with it, which puts every row below it out of step. `wrap="false"` cuts the cell instead, with an ellipsis to say
so, and is what a listing whose rows are clicked or scrolled to wants:

```xml
<Table id="items" columns="DELETED,SIZE,ORIGINAL PATH" rows="{rows}" selected="{selected}" wrap="false"/>
```

### Display

| Element | Attributes | Notes |
| --- | --- | --- |
| `Rule` | `orientation`, `char`, `title`, `color` | A divider, with an optional label breaking it. |
| `ProgressBar` | `value`, `max`, `filled`, `empty`, `color`, `trackColor`, `percent` | Clamps out-of-range values rather than drawing past its track. |
| `Spinner` | `kind`, `frame`, `color` | `kind` is `arrow`, `bar`, `circle`, `dot`, `dots` or `line`. The frame is a tick count from the host and wraps here. |
| `Sparkline` | `values`, `max`, `color` | Plots against `max`, or against the series' own largest value. Keeps the most recent points when the space is short. |
| `Badge` | `label` | A padded chip. |
| `Image` | `src`, `alt`, `protocol` | See docs/images.md. |

Nothing in the library holds state. Every one of them draws what its attributes say, which is why the host's model stays
the only place anything is true. `examples/gallery` is all of them on one screen.

## Writing your own

A widget is a `widget.Native`: measure, then render into what layout settled on.

```go
type clock struct{ at time.Time }

func (c *clock) Measure(maxW, maxH int) (int, int) { return 5, 1 }
func (c *clock) Render(w, h int) string            { return c.at.Format("15:04") }
```

Bind it either per view or per element:

```go
// One instance, shared by every <Clock> in the view.
widget.NewRegistry().Bind("Clock", &clock{at: now})

// Or one per element, built from that element's attributes.
widget.NewRegistry().BindFactory("Clock", myFactory)
```

A factory declares the attributes it reads, so anything else on the element still reaches the stylesheet, and gets a
`widget.Context` holding the evaluated attributes, the view's filesystem, the directory the element was written in, and
whether the theme is dark. Attribute accessors take a fallback and report a failure rather than substituting one: a
value nobody can parse is a mistake in the template, and rendering something plausible instead would hide it.

Then opt into whatever else applies:

| Interface | What it adds |
| --- | --- |
| `Focusable` | Takes part in the focus ring. Returning false means disabled, and a disabled widget is unreachable by pointer too. |
| `Stateful` | Told focus, hover and press before each measure. |
| `Composer` | Wraps the children written inside it: declare the `Inset` you keep, get them back already drawn. |
| `Arranger` | Says how those children are measured and placed -- fill, free (overflowing), or offset. |
| `Slotted` | Names the regions content can be written into, so a misspelt `<Button.Contnt>` fails when the view loads. |
| `Anchored` | An opinion about where it sits on a `Canvas`. `Popup` says centre. |

`Border`, `Popup` and `Scrollbox` are built on exactly these interfaces and nothing else, so a widget written outside
the language is not a second-class one.
