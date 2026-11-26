# tml

Terminal Markup Language: an XML-based declarative language for reusable terminal components. XAML-flavoured, with a renderer and a runtime of its own — TML lays a view out, paints it on a grid of cells. And runs it on the terminal.

```xml
<?xml version="1.1" encoding="UTF-8"?>
<Component xmlns="urn:tml:v1" name="Card">
    <Property name="title" type="string" required="true"/>
    <Template>
        <Box style="card">
            <Text style="card.title">{title}</Text>
            <Slot/>
        </Box>
    </Template>
</Component>
```

```xml
<Card title="Deploy">
    <Text>Ready to ship.</Text>
    <Card.actions><Button label="OK"/></Card.actions>
</Card>
```

## Widgets

Buttons, fields, checkboxes, lists, tabs, tables, frames, popups, scrolling regions, progress bars, spinners, sparklines and images ship with the language and need no setup. Give a control an `id` and an `action` and it joins the focus ring:

```xml
<Button id="save" action="save" variant="primary" label="Save"/>
```

```go
for _, event := range m.view.UI().Update(msg) {
    if event.Kind == tml.Activated {
        m.act(event.Action)   // "save" -- never a coordinate, never a widget pointer
    }
}
```

Tab and the arrows move, enter activates, and the pointer hovers, clicks and scrolls against the frame the user is actually looking at. `go-toolchain && ./build/gallery` is all of it on one screen.

[![The widget gallery](https://sites.pazer.build/tml/gallery-controls.png)](https://sites.pazer.build/tml/)

[![A mock coding agent built with tml](https://sites.pazer.build/tml/agent-permission.png)](https://sites.pazer.build/tml/)

Both pictures are the examples running in a real terminal, retaken on every push. See `docs/screenshots.md` for the rest of them and how they are made.

Your own widgets plug into the same seam the library uses — see `docs/widgets.md`.

## A program

The host keeps its state. TML owns the terminal, the loop, layout, theming and structure.

```go
view, _ := tml.Load(ui, "app.tml", tml.Options{Dark: true})

type model struct{ query string; cursor int }

// Props is what the view draws from, read before every frame.
func (m *model) Props() tml.Props {
    return tml.Props{"query": sema.StringValue(m.query), "cursor": sema.StringValue(strconv.Itoa(m.cursor))}
}

// Update takes one message. A key it does not consume goes to the focus ring and comes back as an Event.
func (m *model) Update(p *tml.Program, msg tml.Msg) bool {
    switch msg := msg.(type) {
    case tml.Key:
        if msg.String() == "esc" {
            p.Quit()
            return true
        }
    case tml.Event:
        if msg.Kind == tml.Changed && msg.ID == "search" {
            m.query, m.cursor = msg.Value, msg.Cursor
        }
    }
    return false
}

err := tml.Run(view, &model{})
```

The program reads the keyboard and the pointer, steps the view's animations, and writes only the cells that changed. See `docs/program.md`.

Run the worked example: `go-toolchain && ./build/dashboard`, or `./build/dashboard -frame` for one frame without a terminal. `./build/agent` is a bigger one. It is a mock coding agent: a transcript that outgrows its viewport, tool output as cards, and a permission prompt that interrupts. It binds two of its own widgets alongside the library's.

## Inline progress

A `tml.Live` draws a view under the shell prompt and repaints it in place. Lines you print go above it and stay in the scrollback. `Stop` takes it away.

```go
live := tml.NewLive(view, snapshot, tml.LiveOptions{})
live.Start()
live.Printf("done %s", name)   // safe from any goroutine
live.Stop()
```

A `Grid` with `collapse` drops columns in a set order as the terminal narrows. `./build/downloads` is a mock download manager built this way. See `docs/live.md`.

## Hot reload

```go
go tml.Watch(ctx, "ui", "app.tml", opts, func(v *tml.View, err error) {
    program.Send(reloaded{view: v, err: err})
})
```

Edits show up without a start. A reload that fails hands you the error instead of quietly keeping the last good view.

## CLI

```bash
tml check   app.tml                      # parse and check, no rendering
tml tree    app.tml                      # the expanded element tree
tml render  app.tml --width 80 --height 24
tml inspect app.tml --width 80 --height 24 --id composer

# a running program (discovers the socket; --socket or $TML_INSPECT_SOCKET overrides)
tml list
tml ids
tml query --id composer
tml tree                                 # the live frame, including boxes with no id
tml serve --addr :7777                   # the browser inspector
tml capture -o frame.html                # the same page over one frame, as a file
tml capture app.tml --width 80 --height 24 -o frame.html
```

`inspect` is the testable half of `render`. It lays the view out, then prints JSON for the elements that carry an `id`. That JSON gives the rect each one occupies, the content size it was given, its clip, its scroll position, and the lines it drew. With `--id` it reports one element. When nothing carries the id asked for, it fails and names the ids that do exist. A test can then assert where a region sits and what it says, without pinning the whole frame.

A program serves on `$XDG_RUNTIME_DIR/tml/<pid>.sock` with nothing set, and `tml list` names them. `tree` with a file is the expanded document. `tree` with no file is the frame on screen. See `docs/inspector.md`.

`capture` writes that browser inspector over a single frame, as a self-contained HTML file. The file holds the terminal as it was painted, the element tree, and every element's geometry, clip, scroll and drawn text. It fetches nothing when it opens, so a frame travels through a bug report or a comment box. With a file it captures a layout at the given size, and with no file it captures the frame a running program has on screen.

## What it does

Typed properties with defaults, slots with fallback content, imports, `if` and `For`. Themes carry named styles and every colour: a control names a colour by what it is for. The theme holds the value for light and for dark. The widget library takes keyboard and mouse interaction, and any widget shows a tooltip when the pointer rests on it. `Grid` takes XAML-style attached properties:

```xml
<Grid columns="auto,1*,2*" gap="1">
    <Text Grid.row="0" Grid.column="1" Grid.columnSpan="2">spans two columns</Text>
</Grid>
```

The load checks everything that needs no call-site value. That covers unknown elements, bad types and unresolved references, even inside a branch that never runs.

`Dock` is not implemented yet and is reported as an unknown element rather than silently ignored.

## Build

```bash
go-toolchain
```

## Docs

- `docs/language.md` — the language reference
- `docs/widgets.md` — the widget library, interaction, and writing your own
- `docs/images.md` — how a picture reaches a terminal, and what happens when it cannot
- `docs/live.md` — an inline region pinned below the log, repainted in place
- `docs/program.md` — the program loop: input, events, timers and what a host writes
- `docs/renderer.md` — cells, styles, the painter and the screen writer
- `docs/agent.md` — the mock coding agent, and what building it changed about the language
- `docs/screenshots.md` — how the pictures above are taken and published
- `CLAUDE.md` — the working index for this repo
