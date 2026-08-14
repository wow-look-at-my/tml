# tml

Terminal Markup Language: an XML-based declarative language for reusable terminal components. XAML-flavoured, built for
the Charm stack — TML solves layout and Lip Gloss renders, so a view drops straight into a Bubble Tea `View()`.

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

## In a Bubble Tea program

The model keeps its state and its `Update`; TML owns layout, theming and structure.

```go
widgets := widget.NewRegistry().Bind("Search", widget.Bubble(&m.input))
view, _ := tml.Load(ui, "app.tml", tml.Options{Widgets: widgets, Dark: true})

func (m *model) View() tea.View {
    out, err := m.view.Render(tml.Props{"title": sema.StringValue("Deployments")}, m.width, m.height)
    ...
}
```

Run the worked example: `go-toolchain && ./build/dashboard`, or `./build/dashboard -frame` for one frame without a
terminal.

## Hot reload

```go
go tml.Watch(ctx, "ui", "app.tml", opts, func(v *tml.View, err error) {
    program.Send(reloaded{view: v, err: err})
})
```

Edits show up without a restart. A reload that fails hands you the error instead of quietly keeping the last good view.

## CLI

```bash
tml check  app.tml                       # parse and check, no rendering
tml tree   app.tml                       # the expanded element tree
tml render app.tml --width 80 --height 24
```

## What it does

Typed properties with defaults, slots with fallback content, imports, `if` and `For`, themes with light/dark tokens and
named styles, and a measure/arrange layout engine with `auto`, fixed and star sizing across `Stack`, `Grid` and `Box`.
`Grid` takes XAML-style attached properties:

```xml
<Grid columns="auto,1*,2*" gap="1">
    <Text Grid.row="0" Grid.column="1" Grid.columnSpan="2">spans two columns</Text>
</Grid>
```

Everything that can be checked without call-site values is checked when the view loads — unknown elements, bad types,
unresolved references, even inside a branch that never runs.

`Dock` and `Overlay` are not implemented yet and are reported as unknown elements rather than silently ignored.

## Build

```bash
go-toolchain
```

## Docs

- `docs/language.md` — the language reference
- `docs/lipgloss-contract.md` — what TML delegates to Lip Gloss, and the traps in it
- `CLAUDE.md` — the working index for this repo
