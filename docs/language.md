# The TML language

A `.tml` file is strict XML 1.1. It opens with `<?xml version="1.1" encoding="UTF-8"?>`, is UTF-8 without a BOM, and has no
DOCTYPE and no entities beyond the predefined five. That comes from xml-validator and is not negotiable per file.

The default namespace is `urn:tml:v1`. It is a URN on purpose: a name, not a URL, and nothing fetches it.

A file holds exactly one definition, rooted at `<Component>` or `<Theme>`.

## Components

```xml
<?xml version="1.1" encoding="UTF-8"?>
<Component xmlns="urn:tml:v1" name="Card">
    <Import src="./Badge.tml"/>

    <Property name="title" type="string" required="true"/>
    <Property name="tags"  type="string[]" default=""/>

    <Template>
        <Box style="card">
            <Text>{title}</Text>
            <Slot/>
        </Box>
    </Template>
</Component>
```

A `<Component>` nested inside another is a file-private helper: visible in that file, never importable.

### Properties

`<Property>` takes `name`, `type`, and either `required="true"` or a `default`. Requiring a property and defaulting it at the
same time is an error, since the default could never apply.

Types: `string`, `int`, `bool`, `color`, `length`, `thickness`, `enum(a|b|c)`, and a `[]` suffix for a list of any of them.
The suffix spelling exists so a type never needs XML escaping, which `list<string>` would.

- `color` is a hex triplet (`#e0af68`, `#fff`) or an ANSI index `0-255`. Names are not accepted; a theme token is how a
  colour gets a name.
- `length` is `auto`, a cell count, or a star share (`*`, `2*`).
- `thickness` is the CSS shorthand: `1`, `1 2`, or `1 2 3 4` clockwise from the top.

A default is evaluated against the theme only, never against the component's other properties, so defaults do not depend on
declaration order.

### Imports

`<Import src="./Badge.tml"/>` resolves relative to the importing file and brings that file's definition into scope under its
own name. **Imports are not transitive**: a component sees its own definition, its file-private helpers, and only what it
imported itself.

An import cycle is harmless and terminates, because importing only makes a name visible. A component that instantiates
itself is a different thing and is rejected.

## Slots

`<Slot>` is both the declaration and the insertion point. Its children are fallback content, used when the call site
supplies nothing.

```xml
<Slot/>                                        <!-- the default slot -->
<Slot name="actions"/>                         <!-- a named slot -->
<Slot name="actions" required="true"/>         <!-- must be filled -->
<Slot name="actions"><Text>none</Text></Slot>  <!-- with fallback -->
```

A call site fills a named slot with XAML property-element syntax. Untagged children go to the default slot.

```xml
<Card title="Deploy">
    <Text>Ready to ship.</Text>          <!-- default slot -->
    <Card.actions>
        <Button label="OK"/>
    </Card.actions>
</Card>
```

**Slot content is a closure over the call site.** It is expanded in the caller's scope and the caller's file, so it sees the
caller's properties and the caller's imports, not the component it is passed into. This is what makes a component
substitutable without knowing where it is used.

Components do not nest lexically for anything else either: an instance sees its own properties and the theme, never the
enclosing component's names.

## Expressions

The grammar is a name, optionally a theme token, optionally negated:

- `{name}` — a property or a `<For>` variable
- `{theme.accent}` — a theme token
- `{not name}` — negation
- `Hello {name}!` — interpolation in text or in any attribute value
- `{{` and `}}` — literal braces, the only escaping there is

There is no arithmetic and there are no calls. Every reference is therefore checked when the view loads, including
references inside a branch that is never taken. An unknown name is an error, never an empty string.

**A lone reference keeps its type.** `padding="{gutter}"` stays a thickness; `title="pad {gutter}"` is a string. This is why
a typed property can be forwarded to a typed attribute without restating the type.

## Control flow

`if` is an attribute on any element and takes a bool, or a string or list where empty counts as false. An `int` or `color`
in an `if` is an error rather than a silent "non-zero".

```xml
<Text if="{subtitle}">{subtitle}</Text>
<Text if="{not subtitle}">no subtitle</Text>

<For each="{tags}" as="tag" index="i">
    <Text>{i}. {tag}</Text>
</For>
```

`<For>` itself renders nothing; its children repeat. The loop variable and index are visible only inside it.

## Layout

Every element takes `width` and `height`: `auto` (default), a cell count, or a star share.

`auto` sizes to content. A star share takes a weighted slice of what is left after the fixed and auto siblings are placed.
A star size propagates upward: a container holding a star child asks for all the space available on that axis, because
otherwise the container would shrink to its content and the star would have nothing to fill.

Panels:

- `Stack` — `orientation="vertical|horizontal"` (vertical is the default) and `gap`
- `Grid` — `columns`, `rows`, `gap`, with placement declared per child
- `Canvas` — free positioning: children sit where they are put, and overlap
- `Box` — a single-child decorator for border, padding, background and alignment
- `Text` — character data, wrapped to the width it is given
- `Spacer` — occupies space and draws nothing

`Dock` is **not implemented yet**. It is deliberately absent from the builtin list, so using it is reported as an unknown
element rather than silently rendering nothing.

### Canvas

```xml
<Canvas>
    <Stack>…the page…</Stack>
    <Popup title="Confirm" if="{confirming}">…</Popup>
    <Badge label="beta" Canvas.anchor="bottomRight" Canvas.x="-2" Canvas.y="-1"/>
</Canvas>
```

A canvas takes all the space it is offered — one that shrank to its content would leave a child anchored to its
bottom-right corner nowhere to sit — and each child is placed by attached properties: `Canvas.x`, `Canvas.y` and
`Canvas.anchor` (`topLeft`, `topRight`, `bottomLeft`, `bottomRight`, `center`). The offsets are measured from the anchor,
so a negative one moves back in from the edge it is pinned to.

A child that says nothing sits where its own default anchor puts it, which is how `<Popup>` lands in the middle without
being told to. Children paint in document order, so a dialog written last is the one on top.

### Grid

```xml
<Grid columns="auto,1*,2*" rows="1,*" gap="1">
    <Text Grid.row="0" Grid.column="1" Grid.columnSpan="2">spans two columns</Text>
</Grid>
```

Tracks are declared on the grid; placement is declared on each child with XAML-style attached properties: `Grid.row`,
`Grid.column`, `Grid.rowSpan`, `Grid.columnSpan`. Track solving runs fixed, then auto, then star, so a star track only ever
divides what the other two left behind, and the last star track absorbs the rounding so the grid fills exactly.

An auto track sizes to the widest child confined to it. A child spanning several tracks is excluded from that measurement,
because it cannot say which of the tracks it covers should grow.

Placing a child past the last declared track widens the grid with auto tracks rather than dropping the child.

An attached property written on a child of anything other than a `Grid` is an error, as is an unknown one such as
`Grid.depth`. Ignoring them would leave a layout that quietly disregards what was written.

Grid children sit at coordinates rather than in a line, so the renderer composites them through lipgloss layers instead of
joining. Each layer gets a distinct, increasing z in document order — see docs/lipgloss-contract.md for why equal z values
would be unsafe.

Layout attributes are not forwarded through a component instance: every attribute on `<Card .../>` is a property. A
component that wants to be sized declares a `length` property and puts it on its own root, which is what `Card.tml` in
`testdata/dashboard` does.

## Themes

```xml
<?xml version="1.1" encoding="UTF-8"?>
<Theme xmlns="urn:tml:v1" name="default">
    <Token name="accent" light="#5f5fd7" dark="#e0af68"/>
    <Token name="gutter" value="1"/>

    <Style name="card"       border="rounded" borderColor="{theme.accent}" padding="0 1"/>
    <Style name="card.title" extends="card"   bold="true"/>
</Theme>
```

A token has either a single `value` or a `light`/`dark` pair, never both and never half a pair. The pair follows
`Options.Dark`.

Style attributes: `fg`, `bg`, `bold`, `italic`, `underline`, `strikethrough`, `faint`, `reverse`, `padding`, `margin`,
`border`, `borderColor`, `align`, `valign`. Borders are named: `none`, `normal`, `rounded`, `thick`, `double`, `hidden`,
`block`, `ascii`.

An element picks a named style with `style="card"` and overrides individual attributes inline.

`extends` is resolved in TML's own attribute model, not by `lipgloss.Style.Inherit`, because Inherit deliberately drops
padding and margin. See docs/lipgloss-contract.md.

## Widgets

Everything that is not a panel is a widget. The library in `widgets/` — buttons, fields, frames, popups, scrolling
regions, images and the rest — is registered by default, and `id` and `action` on any element are what wire it to the
host's `Update`. See docs/widgets.md for the reference and for writing your own.

A host binds its own elements the same way, and TML measures, places and draws them without touching their state:

```go
widgets := widget.NewRegistry().Bind("Search", widget.Bubble(&m.input))
view, err := tml.Load(ui, "app.tml", tml.Options{Widgets: widgets})
```

Pass a pointer to `Bubble`. `SetWidth` has a pointer receiver, and without one the width TML computes would be set on a
copy and thrown away.

Widget names are checked when the view loads, so a template naming a widget that was never bound fails there rather than
rendering a blank.

TML never routes messages: the host keeps its `Update` and its state. What it does report is what the user did —
`view.UI().Update(msg)` hands back the id and action of whatever was activated, and nothing else. `examples/dashboard`
shows the split with a bubbles component; `examples/gallery` shows it with the library.
