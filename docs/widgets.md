# Widgets

TML's language is layout. Everything else on screen is a widget, and widgets come from a registry. Default registers the library in `widgets/`. A template can therefore use `<Button>` with no setup. A host that binds its own `Button` wins the name, and `Options{Bare: true}` drops the library fully.

The split is that **widgets draw and panels solve constraints**. A widget is handed a canvas of the size layout settled on, and draws on it. That is the whole contract, and every widget below goes through it -- including the ones that wrap children.

## Interaction

Attributes work on any element:

- `id` names it. An id is how focus survives from one frame to the next. It is also how the host tells one control from another. Elements sharing an id is an error where it is written.
- `action` is the string the element reports when it is activated. The host matches on it, so its `Update` never learns where anything ended up.
- `tooltip` is the text the element shows when the pointer rests on it. See [Tooltips](#tooltips).

```go
func (m *model) Update(p *tml.Program, msg tml.Msg) bool {
    if event, ok := msg.(tml.Event); ok {
        switch event.Kind {
        case tml.Activated:
            m.act(event.Action)
        case tml.Changed:
            m.fields[event.ID] = field{value: event.Value, cursor: event.Cursor}
        case tml.Scrolled:
            m.scroll(event.ID, event.Delta)
        }
    }
    return false
}
```

The program gives every input value to the app first. A value the app does not consume goes to the focus ring, and what the ring makes of it comes back as events. Tab and the arrows move the ring, enter and space activate, and the pointer hovers, clicks and scrolls. Replace the bindings with `view.UI().SetKeyMap(...)` when the host wants a key back. A host with a loop of its own calls `view.UI().Update(msg)` itself.

A focused `Textbox` takes keys before the ring does. It edits its value and holds nothing. Enter, tab and the vertical arrows are not the field's, so they still reach the ring. A pasted text arrives the same way, as one edit.

By default, the pointer is reported, motion included, because hover is part of how a control draws. `ProgramOptions{NoMouse: true}` leaves the pointer to the terminal, so the user selects text with it.

A control is reachable ways. The two are not the same set:

- the **focus ring** is the widgets that take focus. Tab steps through them, in document order.
- the **pointer** reaches those plus anything else carrying an id, which is how the wheel finds a `Scrollbox` that is not a tab stop.

A disabled control is in neither. The frame's geometry leaves it out fully. A click where it is drawn does nothing at all.

One widget can draw many things — a list's rows, a table's columns — and the ring still sees one element. A pointer event carries `X` and `Y` in cells from that element's own corner, which is how the host tells them apart. Both are `-1` when the keyboard did it.

```go
if event.ID == "services" && event.Y >= 0 {
    m.selected = event.Y
}
```

A widget that knows its own parts names them, and the name arrives as `event.Part`. A `Table` answers `header:N` over the heading of column N and `row:N` over data row N. A `List` answers `item:N`. A widget gets this by implementing `widget.Parted`. The point it is asked about is measured from the widget's whole box. A table scrolled halfway out of its viewport still names the right row.

`event.Mod` carries the modifier keys held with the pointer, so ctrl+click and shift+click reach the host.

A right click raises `tml.Menu` on the element under the pointer. It moves neither the focus nor the press, and its release is no click. A host opens its context menu there: a `<Menu>` on a `Canvas`, placed at `Canvas.x` and `Canvas.y`. See [Controls](#controls).

The left button on a `Scrollbox`'s scrollbar holds it. A press on the thumb holds it where it was pressed, and a press on the track jumps the thumb under the pointer. While the button is held, moving the pointer reports `tml.Scrolled` with `Delta` set to the rows the content has to move. A host that scrolls on the wheel therefore scrolls on the bar with no new code. Releasing the button ends the drag, and is no click.

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

Every widget also takes the style attributes. Anything a widget does not claim goes to the stylesheet, so `<Badge label="new" bg="#f00"/>` means both things at once.

### Containers

| Element | Attributes | Notes |
| --- | --- | --- |
| `Popup` | as `Border` | A dialog. Defaults to a rounded border and centres itself on a `Canvas`. |

`Scrollbox` is where the pointer-only rule earns itself. It never takes focus, and its clipping is real. A control scrolled out of view is not in the frame's geometry. A click where it used to be finds nothing.

**`contentHeight` is for content too long to lay out.** A layout of the whole conversation, to show fifty rows of it, costs what the conversation costs, on every frame. Measured at 200x50, a transcript of 10 000 rows is 758 ms. One of 1 000 rows is 80 ms. A frame is 16 ms. Hand over the rows on screen instead and say how many there are altogether:

```xml
<Scrollbox id="transcript" offset="{scroll}" contentHeight="{lines}">
```

The same measurement windowed is **7.2 ms and flat**: 7.1 at many rows, 7.2 at 1 000, 7.5 at 10 000. The cost is now the viewport rather than the content. In this mode the children draw from the top, because the host already cut at the offset. `offset` remains the position. Both the scrollbar and the maximum reported through `UI().Target` describe the whole content, which is the only reason to be told its size. The slice is the host's.

It owns the content, so only it can say which rows the offset selects.

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

A `Menu` is a framed column of choices, for a context menu or a drop-down:

```xml
<Canvas>
    ...
    <Menu id="menu" action="menu" title="pane" items="{choices}" selected="{marked}"
        Canvas.x="{menuX}" Canvas.y="{menuY}" if="{menuOpen}"/>
</Canvas>
```

It is as wide as its longest line, the title included, up to `maxWidth`. A longer choice is cut with an ellipsis. A `Canvas` keeps it whole on screen: opened near an edge, it moves back inside. The host hands over the pointer's cell and nothing else. `selected` is the marked choice. The host owns it, as with a `List`.

The mark is the `accent` and `accentText` colours when the element gives them, else reversed video. A click reports `Activated` with `event.Part` set to `item:N`.

### Data

Both take focus, and both leave the selection to the host. `selected` is a row index the model owns. Nothing in the widget can therefore disagree with what the program thinks is picked. A `Table` marks its row in place, as a reversed bar across every column, bold while the table focus. It uses no cursor gutter, because a row of columns has no single place to put one. The bar draws whether or not the table focus.

The row a keystroke acts on is the host's state. It stays picked while a filter box holds the ring. The header is not a row anyone can select, and `selected="-1"` marks nothing.

Which row a click landed on arrives the same way a `List`'s does, in the event's own coordinates. A borderless table draws its header and the rule under it first, so its rows start a couple of lines below. A bordered table adds its top border to that:

```go
if event.ID == "steps" && event.Y >= 0 {
    m.step = clamp(event.Y-2, 0, len(steps)-1)
}
```

That arithmetic holds only while a row is a line. A cell too wide for its column wraps by default and takes the next line with it. That puts every row below it out of step. `wrap="false"` cuts the cell instead, with an ellipsis to say so. A listing whose rows are clicked or scrolled to wants that:

```xml
<Table id="items" columns="DELETED,SIZE,ORIGINAL PATH" rows="{rows}" selected="{selected}" wrap="false"/>
```

A table takes every row, and the host never slices it. For a long listing, build a `widget.Rows` once when the data changes and pass `rows.Value()` in `Props` on every frame. The table then does the rest:

- It sizes each column from the widest cell over every row. The `Rows` measures that once, so a scroll never resizes a column.
- Inside a `Scrollbox` it draws only the rows on screen. A frame costs a screenful whatever the length. The header stays at the top while the rows scroll under it.
- `event.Part` names the row by its index in the whole listing, however far it scrolled.

```go
rows := widget.NewRows(cells) // [][]string, rebuilt when the data changes
props["rows"] = rows.Value()
```

```xml
<Scrollbox id="items" offset="{offset}" height="*">
    <Table id="rows" columns="{columns}" rows="{rows}" selected="{selected}" wrap="false"
        weights="2,1,1,3" fit="0,2"/>
</Scrollbox>
```

`weights` shares spare room among the columns in proportion, one number for each column. A column the list does not reach weighs one, and a weight of zero takes none. `fit` names the columns that keep their whole width longest when the table is too narrow, the most important first. The other columns give way first, widest first, and then the fit columns, the last named first. A plain list of delimited strings still works for a short table.

### Display

| Element | Attributes | Notes |
| --- | --- | --- |
| `Spinner` | `kind`, `color` | It animates itself; see [Animation](#animation). |
| `Badge` | `label` | A padded chip. |

Nothing in the library holds state of its own. Every one of them draws what its attributes say, which is why the host's model stays the only place anything is true. The element that moves is a `Spinner`. It moves because the engine counts its frame. `examples/gallery` is all of them on one screen.

## Animation

A `Spinner` changes without the host saying so. The engine keeps the frame counter, so no template passes one, and a widget that animates says so by implementing `widget.Animator`:

```go
// Animating reports whether the widget is still changing on its own.
Animating() bool
```

A program steps the animation by itself: its clock runs while something on screen moves, and stops when nothing does. A host with a loop of its own does the same by hand:

```go
view.Advance()                                    // move every animation on one frame
out, err := view.Render(props, width, height)     // draw the frame they arrived on
if view.Animating() { /* keep the ticker */ }     // false means nothing on screen is moving
```

`Animating` is about what is on screen, not about what exists. An element carrying `state="hidden"` or `state="collapsed"` is not drawn. It counts as nothing animating, and a view whose only animation is hidden reports false. Hiding the element is therefore what stops the repainting, with no switch anywhere. The states are docs/language.md §Visibility.

## Tooltips

```xml
<Button id="save" action="save" label="Save" tooltip="Writes the file to disk"/>
```

A widget with a `tooltip` shows the text when the pointer rests on it with no button held. The program waits `ProgramOptions.TooltipDelay`, and a pointer that moves starts the wait again. Any key, press, wheel turn or move takes the tooltip away. A negative delay shows none.

A tooltip is not a menu. It does not look like one. A `Menu` is a bordered box with a title. It takes the pointer. A tooltip is a filled strip with no border, in the theme's `tooltip` and `tooltipText` colours. It sits on the row under the pointer, or over the pointer at the bottom of the screen, and it never takes input.

A theme changes the look by declaring those colours.

The control on top decides. A popup over a button hides the button's tooltip, as it hides the button. `tooltip` on an element that is not a widget is an error: the pointer reaches widgets only.

A host with a loop of its own does what the program does:

```go
ui := view.UI()
if hid := ui.Observe(msg); hid { repaint() }   // Update calls Observe itself
dwell, waits := ui.Dwell()
if waits {
    time.AfterFunc(delay, func() { loop <- func() { if ui.ShowTooltip(dwell) { repaint() } } })
}
```

`ui.Tooltip()` reads the tooltip on screen.

## Writing your own

A widget takes every colour it draws with from the theme: `ctx.Color("color")` for an attribute that names one, `ctx.Theme.Named("accent")` for a name it knows. See docs/language.md §Colours.

A widget is a `widget.Native`: measure, then draw on what layout settled on.

```go
type clock struct{ at time.Time }

func (c *clock) Measure(maxW, maxH int) (int, int) { return 5, 1 }
func (c *clock) Draw(canvas cell.Canvas) {
    canvas.Text(0, 0, c.at.Format("15:04"), cell.Style{}.Bold(true))
}
```

The canvas clips, so a widget never checks its own edges. It also carries the style the element inherited, so a widget states only what it changes. The text above is bold in whatever colour the element has. `widget/widgettest` draws a widget with no view around it, for a test that asserts on the cells.

Bind it either per view or per element:

```go
// One instance, shared by every <Clock> in the view.
widget.NewRegistry().Bind("Clock", &clock{at: now})

// Or one per element, built from that element's attributes.
widget.NewRegistry().BindFactory("Clock", myFactory)
```

A factory declares the attributes it reads. Anything else on the element still reaches the stylesheet. The factory gets a `widget.Context`. That holds the evaluated attributes, the view's filesystem, the element's own directory, and whether the theme is dark. Attribute accessors take a fallback and report a failure rather than substitute one. A value nobody can parse is a mistake in the template.

A plausible substitute hides it.

Then opt into whatever else applies:

| Interface | What it adds |
| --- | --- |
| `Focusable` | Takes part in the focus ring. Returning false means disabled, and a disabled widget is unreachable by pointer too. |
| `Stateful` | Told focus, hover and press before each measure. |
| `Composer` | Wraps the children written inside it: declare the `Insets` you keep on each side. `Draw` paints what sits behind the children, and the engine paints the children inside the insets. |
| `Overlay` | Draws over the children once they are painted. `Scrollbox` draws its scrollbar and its fade this way. |
| `Styler` | A style the children inherit. `Border` hands a button's face to its label this way. |
| `Scrolling` | Told where layout put its content, before it draws. |
| `Keyed` | Takes keys itself while it holds the focus, and answers with an edit. `Textbox` does. |
| `Arranger` | Says how those children are measured and placed -- fill, free (overflowing), or offset. |
| `Slotted` | Names the regions content can be written into, so a misspelt `<Button.Contnt>` fails when the view loads. |
| `Templated` | Names another element the engine wraps the children in, the way a WPF control template is a Border around a ContentPresenter. `Bind` hands that element back, so the widget's state can steer how it draws. `Button` is a templated `Border`. |
| `Anchored` | An opinion about where it sits on a `Canvas`. `Popup` says centre. |
| `Contained` | A `Canvas` keeps it whole on screen, and moves it back inside when it is placed across an edge. `Menu` asks for this. |

A widget that scrolls content of its own draws the library's scrollbar with `widget.Scrollbar`. `Draw` paints the track and the thumb in one column, `Thumb` says where the thumb is, and `At` turns a pointer row into a position. That `Draw` is for a drag. A `Scrollbox` draws its own bar from the same code.

The language is not a second-class one.
