# Widgets

TML's language is layout: `Stack`, `Grid`, `Canvas`, `Box`, `Text`, `Spacer`. Everything else on screen is a widget, and widgets come from a registry. The library in `widgets/` is registered by default. A template can therefore use `<Button>` with no setup. A host that binds its own `Button` wins the name, and `Options{Bare: true}` drops the library entirely.

The split is that **widgets draw and panels solve constraints**. A widget is handed a size and returns a string. That is the whole contract, and every widget below goes through it -- including the ones that wrap children.

## Interaction

Attributes work on any element:

- `id` names it. An id is how focus survives from one frame to the next. It is also how the host tells one control from another. Elements sharing an id is an error where it is written.
- `action` is the string the element reports when it is activated. The host matches on it, so its `Update` never learns where anything ended up.

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

`Update` takes any Bubble Tea message and ignores the ones that mean nothing here, so forwarding everything is safe. Tab and the arrows move the focus ring, enter and space activate, and the pointer hovers, clicks and scrolls. Replace the bindings with `view.UI().SetKeyMap(...)` when the host wants a key back.

Mouse events only arrive if the program asks for them. In Bubble Tea v2 that is a property of the view:

```go
view := tea.NewView(m.frame())
view.MouseMode = tea.MouseModeAllMotion  // cell motion only reports movement while a button is held
```

A control is reachable ways. The two are not the same set:

- the **focus ring** is the widgets that take focus. Tab steps through them, in document order.
- the **pointer** reaches those plus anything else carrying an id, which is how the wheel finds a `Scrollbox` that is not a tab stop.

A disabled control is in neither. The frame's geometry leaves it out entirely. A click where it is drawn does nothing at all.

One widget can draw many things — a list's rows, a table's columns — and the ring still sees one element. A pointer event carries `X` and `Y` in cells from that element's own corner, which is how the host tells them apart. Both are `-1` when the keyboard did it.

```go
if event.ID == "services" && event.Y >= 0 {
    m.selected = event.Y
}
```

### Reading the frame back

`view.UI().Target(id)` is where an element ended up in the last frame: its rect, and for a scrolling region, how far it actually scrolled.

That last number matters because the host cannot work it out. How far a `Scrollbox`'s content runs depends on how the text inside it wrapped at the width the widget was given. "The bottom" is therefore known only once the frame exists. A host that follows a growing transcript asks to go further than there is, then reads back where that landed. Scrolling clamps to the content, in the drawing and in the geometry alike:

```go
m.offset = 1 << 20                                  // the end, wherever it is
...
if session, ok := m.view.UI().Target("session"); ok {
    m.offset = min(session.Scroll.Y+event.Delta, session.Scroll.MaxY)   // one notch off it
}
```

Focus is a rendering input, not something a widget stores. The engine hands each widget its `widget.State` before it measures. A control that grows on focus is therefore measured at the size it draws at.

## The library

Every widget also takes the style attributes (`fg`, `bg`, `bold`, `padding`, `border`, …). Anything a widget does not claim goes to the stylesheet, so `<Badge label="new" bg="#f00"/>` means both things at once.

### Containers

| Element | Attributes | Notes |
| --- | --- | --- |
| `Border` | `kind`, `title`, `titleAlign`, `color`, `pad` | A frame around its children. `kind` is a lipgloss border name. The title is spliced into the top edge. |
| `Popup` | as `Border` | A dialog. Defaults to a rounded border and centres itself on a `Canvas`. |
| `Scrollbox` | `offset`, `offsetX`, `scrollbar`, `contentHeight`, `fade`, `moreAbove`, `moreBelow` | A viewport onto taller content. The host owns the offset. The wheel reports `Scrolled` and the host moves it. `scrollbar` is `auto`, `always` or `never`. `contentHeight` says the children are a window over content that long -- see below. `fade` blends the rows at an edge that has content beyond it toward the background, one step per row of the band, so the edge reads as a gradient; `moreAbove` and `moreBelow` name those edges for a host that cut the window itself and has no total to offer. |

`Scrollbox` is where the pointer-only rule earns itself. It never takes focus, and its clipping is real. A control scrolled out of view is not in the frame's geometry. A click where it used to be finds nothing.

**`contentHeight` is for content too long to lay out.** A layout of the whole conversation, to show fifty rows of it, costs what the conversation costs, on every frame. Measured at 200x50, a transcript of 10 000 rows is 758 ms. One of 1 000 rows is 80 ms. A frame is 16 ms. Hand over the rows on screen instead and say how many there are altogether:

```xml
<Scrollbox id="transcript" offset="{scroll}" contentHeight="{lines}">
```

The same measurement windowed is **7.2 ms and flat**: 7.1 at many rows, 7.2 at 1 000, 7.5 at 10 000. The cost is now the viewport rather than the content. In this mode the children draw from the top, because the host already cut at the offset. `offset` remains the position. Both the scrollbar and the maximum reported through `UI().Target` describe the whole content, which is the only reason to be told its size. The slice is the host's. It owns the content, so only it can say which rows the offset selects.

**A list of items windows through `tml.Window`, not through a count of the host's own.** A conversation is a list of messages, each a data template, and the host does not know how many rows the engine draws one in. `View.ItemHeight(template, item, width)` asks the engine: it measures one item and publishes nothing. A `Window` keeps the scroll position as an item and a row in it, the bottom row on screen. Its `Slice` names the items on screen and the offset that hides the rows above the window. The host hands those items over whole, with no `contentHeight`:

```go
s := win.Slice(list, rows)   // list reports each item's ItemHeight
props["items"] = records(s.First, s.Last)
props["offset"] = s.Offset
```

```xml
<Scrollbox offset="{offset}">
	<Stack itemsSource="{items}" itemTemplate="Message" width="*"/>
</Scrollbox>
```

The window stops at the first and the last screenful. It steps over an item of height zero, which is how a host hides one without removing it. It follows the end of the list until the user scrolls away, and again once the user scrolls back to the newest row. `Scroll(n, list, rows)` moves it, and `Follow` sends it to the end.

### Controls

| Element | Attributes | Notes |
| --- | --- | --- |
| `Button` | `label`, `variant`, `disabled` | `variant` is `default`, `primary` or `danger`. Focused, hovered and pressed each look different. |
| `Textbox` | `value`, `placeholder`, `cursor`, `disabled`, `password` | Draws a field. It does not edit one. Editing is state, and state is the host's. |
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
| `Table` | `columns`, `rows`, `separator`, `border`, `selected`, `disabled`, `wrap` | Rows are `separator`-joined cells. Columns size to their widest. |

Both take focus, and both leave the selection to the host. `selected` is a row index the model owns. Nothing in the widget can therefore disagree with what the program thinks is picked. A `Table` marks its row in place, as a reversed bar across every column, bold while the table has focus. It uses no cursor gutter, because a row of columns has no single place to put one. The bar draws whether or not the table has focus. The row a keystroke acts on is the host's state. It stays picked while a filter box holds the ring. The header is not a row anyone can select, and `selected="-1"` marks nothing.

Which row a click landed on arrives the same way a `List`'s does, in the event's own coordinates. A table draws its header first, so its rows begin a line below:

```go
if event.ID == "steps" && event.Y >= 0 {
    m.step = clamp(event.Y-1, 0, len(steps)-1)
}
```

That arithmetic holds only while a row is a line. A cell too wide for its column wraps by default and takes the next line with it. That puts every row below it out of step. `wrap="false"` cuts the cell instead, with an ellipsis to say so. A listing whose rows are clicked or scrolled to wants that:

```xml
<Table id="items" columns="DELETED,SIZE,ORIGINAL PATH" rows="{rows}" selected="{selected}" wrap="false"/>
```

### Display

| Element | Attributes | Notes |
| --- | --- | --- |
| `Rule` | `orientation`, `char`, `title`, `color` | A divider, with an optional label breaking it. |
| `ProgressBar` | `value`, `max`, `filled`, `empty`, `color`, `trackColor`, `percent` | Clamps out-of-range values rather than drawing past its track. |
| `Spinner` | `kind`, `color` | `kind` is `arrow`, `bar`, `circle`, `dot`, `dots` or `line`. It animates itself; see [Animation](#animation). |
| `Sparkline` | `values`, `max`, `color` | Plots against `max`, or against the series' own largest value. Keeps the most recent points when the space is short. |
| `Badge` | `label` | A padded chip. |
| `Image` | `src`, `alt`, `protocol` | See docs/images.md. |

Nothing in the library holds state of its own. Every one of them draws what its attributes say, which is why the host's model stays the only place anything is true. The one element that moves is a `Spinner`, and it moves because the engine counts its frame. `examples/gallery` is all of them on one screen.

## Animation

A `Spinner` changes without the host saying so. The engine keeps the frame counter, so no template passes one, and a widget that animates says so by implementing `widget.Animator`:

```go
// Animating reports whether the widget is still changing on its own.
Animating() bool
```

The host steps the animation and asks whether anything moved:

```go
view.Advance()                                    // move every animation on one frame
out, err := view.Render(props, width, height)     // draw the frame they arrived on
if view.Animating() { /* keep the ticker */ }     // false means nothing on screen is moving
```

`Animating` is about what is on screen, not about what exists. An element carrying `state="hidden"` or `state="collapsed"` is not drawn, so it counts as nothing animating, and a view whose only animation is hidden reports false. Hiding the element is therefore what stops the repainting, with no switch anywhere. The states are docs/language.md §Visibility.

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

A factory declares the attributes it reads. Anything else on the element still reaches the stylesheet. The factory gets a `widget.Context`. That holds the evaluated attributes, the view's filesystem, the element's own directory, and whether the theme is dark. Attribute accessors take a fallback and report a failure rather than substitute one. A value nobody can parse is a mistake in the template. A plausible substitute hides it.

Then opt into whatever else applies:

| Interface | What it adds |
| --- | --- |
| `Focusable` | Takes part in the focus ring. Returning false means disabled, and a disabled widget is unreachable by pointer too. |
| `Stateful` | Told focus, hover and press before each measure. |
| `Composer` | Wraps the children written inside it: declare the `Inset` you keep, get them back already drawn. |
| `Arranger` | Says how those children are measured and placed -- fill, free (overflowing), or offset. |
| `Slotted` | Names the regions content can be written into, so a misspelt `<Button.Contnt>` fails when the view loads. |
| `Anchored` | An opinion about where it sits on a `Canvas`. `Popup` says centre. |

`Border`, `Popup` and `Scrollbox` are built on exactly these interfaces and nothing else, so a widget written outside. The language is not a second-class one.
