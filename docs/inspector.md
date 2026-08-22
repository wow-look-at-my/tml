# Inspecting a running program

A TML program draws through `View.Render`. The inspector records the box tree
that frame was laid out from, and answers questions about it. Nothing is
re-rendered to answer a question, so an answer describes the frame the terminal
is showing rather than a frame built to be asked about.

The unit of every answer is one element, not the screen. `tml query --id
prompt` reports that element's rectangle, its content size, its clip, its
scroll offsets, whether it holds focus, and the text it drew. A whole-screen
dump would say none of that.

## Attaching it

A host makes one, wires input, and serves:

```go
insp := tml.NewInspector(m.view)
m.view.OnFrame(insp.Publish)
insp.OnKey(func(key string) error { program.Send(keyMsg(key)); return nil })
insp.OnClick(func(x, y int) error {
	program.Send(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	return nil
})
insp.OnRepaint(func() error { program.Send(repaintMsg{}); return nil })
if err := insp.ListenSocket(os.Getenv("TML_INSPECT_SOCKET")); err != nil {
	return err
}
```

Each hook is optional and each one is a capability. A host that wires no key
handler is read-only, and the protocol answers `op=key` with that reason rather
than accepting the keystroke and dropping it.

`OnRepaint` is the one that is easy to skip and painful to skip. An override
only reaches the screen on the next paint, and an idle program does not paint,
so without it a restyle looks like it did nothing. `Restyle` and `Reset` drive
the repaint themselves and wait for the frame that answers, so a caller that
restyles and then reads gets the geometry it asked for.

`examples/agent/main.go` wires all four behind its `-inspect` flag.

## One protocol, two transports

`inspect.Server.Handle(Request) Response` is the whole protocol. A unix socket
carries one JSON request per line, which is what `tml query` and the other live
commands speak; HTTP posts the identical objects to `/rpc` and streams frames
on `/events`. There is one place a protocol bug can be.

| op | answers |
| --- | --- |
| `query` | one element by id |
| `elements` | every element that carries an id, in document order |
| `ids` | just the names |
| `tree` | every box, including the ones with no id |
| `at` | the innermost element covering a cell |
| `frame` | the painted frame; with `since`, waits for a newer one |
| `key`, `click` | drive the program |
| `restyle`, `reset` | override attributes per id, and drop the overrides |

A name that is not in the frame is answered with the names that are. An empty
answer would read like an element that drew nothing.

## The CLI

```
$ TML_INSPECT_SOCKET=/tmp/agent.sock tml tree
<Agent>                            100x30  @  0,0
  <Canvas>                           100x30  @  0,0
    ...
      <Grid>                              98x20  @  1,4
        <Border>                            20x20  @  1,4
          <List> #files *focus*               16x4   @  1,4    > cmd/report.go ...

$ tml query --id prompt --field text
ask for a change, or press space to step the script

$ tml at --x 25 --y 4
session

$ tml input --key space
$ tml restyle --id send --set width=20
```

## The browser inspector

`tml serve` opens a local page carrying a live preview of the terminal,
the element tree, and the element you pick. It talks to the program over the
same socket, so it needs no cooperation from the host beyond the hooks above.

Clicking the preview selects the element under the cell. Dragging moves it, by
writing `margin-left` and `margin-top` overrides; shift-dragging resizes it, by
writing `width` and `height`. Both are real overrides that the engine lays out,
so a sibling reflows and the terminal shows the same thing the page does. The
attribute form sets any attribute by name, and one button drops every override.

## Overrides

`layout.Options.Override` is a function from id to attributes. The engine merges
what it returns into the element's attributes before inline styles and before
the stylesheet resolves, so an override wins over both and is laid out rather
than painted over. That is why a drag reflows the document instead of moving a
picture.

## What checks it

`tools/inspector-check/check.mjs` runs `build/agent` under a pty, asks the CLI
the questions above, then drives the browser with a pointer and reads the result
back off the socket. It runs in CI. Removing the `OnRepaint` wiring from the
example is enough to turn it red.
