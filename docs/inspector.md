# Inspecting a running program

A TML program draws through `View.Render`. The inspector records the box tree
that frame was laid out from, and answers questions about it. Nothing is
re-rendered to answer a question, so an answer describes the frame the terminal
is showing rather than a frame built to be asked about.

The unit of every answer is one element, not the screen. `tml-test query --id
prompt` reports that element's rectangle, its content size, its clip, its
scroll offsets, whether it holds focus, and the text it drew. A whole-screen
dump would say none of that.

## There is nothing to attach

Every `View` that `Load` builds is adopted by the process's one inspector, and
the socket named by `TML_INSPECT_SOCKET` is opened the first time there is a
frame to answer about. A host writes no line of code for either.

```sh
TML_INSPECT_SOCKET=/tmp/app.sock ./myprogram &
tml-test --socket /tmp/app.sock query --id prompt
```

That is deliberate, and it is the whole design. Wiring an inspector used to be
six calls a host made in the right order, and a host that made five of them got
a program that was inspectable in principle and useless in fact — so its tests
went back to reading pane captures, which cannot tell a status line that moved
from one that did not. A capability every program has is worth more than a
capability every program could have.

`tml.NewProgram` is the other half: it builds the Bubble Tea program, so the
protocol has something to type into and the program can be driven as well as
read. It returns the program, because a host that kills it from a worker or
sends to it needs the handle. `tml.Run` is the same plus running it, for a host
with nothing to say in between.

```go
program, err := tml.NewProgram(model, tea.WithContext(ctx))
if err != nil { ... }
```

`tea.NewProgram` builds a program this library cannot reach. One started that
way still answers every question about its frames, and refuses every keystroke
by name — true, unhelpful, and avoided by there being one way to start.

Reloading is nothing either. `Load` bakes in which half of every theme token
resolves and how a width is measured, so a host that learns either late — a
terminal answering OSC 11, or mode 2027 — loads again and renders through a new
`View`. The inspector follows: the new view records, the old one stops, the
overrides carry over, and frame numbers continue upward so a caller waiting for
a newer frame is not answered by the new view's first paint.

### The one thing a host writes

`tml.RepaintMsg` arrives after an override, and a model that caches its frame
must invalidate that cache on it. Bubble Tea redraws after every update, but a
model answering `View` with the string it answered last time paints the old
geometry, so the override lands in the layout and never on the screen.

The protocol catches that rather than letting it pass: `Restyle` and `Reset`
drive the repaint themselves and wait for a frame that is genuinely new, and
fail by name when none arrives.

`tml.InspectError` reports a socket that was asked for and could not be opened.
`tml.NewProgram` returns it, and so does `tml.Run`. A session
that silently is not listening is one every question times out against with
nothing to read.

## One protocol, two transports

`inspect.Server.Handle(Request) Response` is the whole protocol. A unix socket
carries one JSON request per line, which is what `tml-test` speaks; HTTP posts
the identical objects to `/rpc` and streams frames on `/events`. There is one
place a protocol bug can be.

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
$ TML_INSPECT_SOCKET=/tmp/agent.sock tml-test tree
<Agent>                            100x30  @  0,0
  <Canvas>                           100x30  @  0,0
    ...
      <Grid>                              98x20  @  1,4
        <Border>                            20x20  @  1,4
          <List> #files *focus*               16x4   @  1,4    > cmd/report.go ...

$ tml-test query --id prompt --field text
ask for a change, or press space to step the script

$ tml-test at --x 25 --y 4
session

$ tml-test input --key space
$ tml-test restyle --id send --set width=20
```

## The browser inspector

`tml-test serve` opens a local page carrying a live preview of the terminal,
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
