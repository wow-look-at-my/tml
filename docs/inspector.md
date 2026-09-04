# Inspecting a running program

A TML program draws through `View.Render`. The inspector records the box tree
that frame was laid out from, and answers questions about it. Nothing is
re-rendered to answer a question, so an answer describes the frame the terminal
is showing rather than a frame built to be asked about.

The unit of every answer is one element, not the screen. `tml query --id
prompt` reports that element's rectangle, its content size, its clip, its
scroll offsets, whether it holds focus, and the text it drew. A whole-screen
dump would say none of that.

## There is nothing to attach, and nothing to turn on

Every `View` that `Load` builds is adopted by the process's one inspector, and
the socket is opened the first time there is a frame to answer about. Not if a
variable is set, not if a flag is passed: a program that loads a document is a
program the inspector can reach.

```sh
./myprogram &
tml query --id prompt
```

Neither command names a socket. The program serves on
`$XDG_RUNTIME_DIR/tml/<pid>.sock`, and `tml` looks there, dials what it
finds and uses the one that answers. `tml list` is every program running
now. Two or more, and it says so and asks for `--socket`.

- `TML_INSPECT_SOCKET` overrides the path. It is an override, not a switch:
  with it unset the program still serves.
- `TML_INSPECT_DIR` overrides the directory the default path is built in,
  which is what a test sets to keep its programs out of the user's listing.
- The directory is 0700. The socket carries the right to drive the program, so
  its boundary is the same one the user's own shell already has.
- A view that cannot be served is not returned: `Load` fails, naming the path.
  Handing back a working View and a program nothing can reach is the state this
  exists to remove.

That is deliberate, and it is the whole design. Wiring an inspector used to be
six calls a host made in the right order, and a host that made five of them got
a program that was inspectable in principle and useless in fact — so its tests
went back to reading pane captures, which cannot tell a status line that moved
from one that did not. An environment variable is the same failure with fewer
steps: a capability every program has is worth more than one every program
could have.

`tml.NewProgram` is the other half, and the one thing a host does. It builds the
Bubble Tea program, so the protocol has something to type into and the program
can be driven as well as read. It returns the program, because a host that kills
it from a worker or sends to it needs the handle. `tml.Run` is the same plus
running it, for a host with nothing to say in between.

It cannot be automatic. The inspector delivers a keystroke by sending a message
to a running `*tea.Program`, which does not exist when this package
initialises, and Go offers no way to intercept another package's constructor.
So it is enforced instead: a view still painting `DriveGrace` after its first
frame with nothing able to drive it takes the program down, naming this
function. A program the debugger only half works against is not one this
library keeps running — see `drivable.go`, and `testdata/undrivable` for the
program that proves the guard still fires.

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

### Waiting for the screen

A test asserts about a screen that is still changing, so its real question is
"has it happened yet". A sleep answers that by guessing at how fast the machine
is, and the guess is wrong on somebody else's machine.

```
$ tml query --id status --field text --await 'turn [0-9]+'
turn 1

$ tml query --id notice --field text --await-gone 'working' --timeout 30s
```

Both block until the field matches, or stops matching, and both exit non-zero
on a timeout naming what the element last drew — which "expected output to
contain X" never says. `tml frame --since N` is the same idea for the
whole screen: it blocks until a frame newer than N, so a repaint a test is
waiting for is one it can wait on.

`tml frame --max-width` prints the widest line of the frame in DISPLAY
CELLS, which is what catches a region that fits its own rectangle and paints
past its edge. Bytes and runes are both a different number: a box rule is three
bytes a cell, and a wide glyph is one rune in two cells.

## The browser inspector

`tml serve` opens a local page carrying a live preview of the terminal,
the element tree, and the element you pick. It talks to the program over the
same socket, so it needs no cooperation from the host beyond the hooks above.

Clicking the preview selects the element under the cell. Dragging moves it, by
writing `margin-left` and `margin-top` overrides; shift-dragging resizes it, by
writing `width` and `height`. Both are real overrides that the engine lays out,
so a sibling reflows and the terminal shows the same thing the page does. The
attribute form sets any attribute by name, and one button drops every override.

## A capture is that page over a frozen frame

`tml capture` writes the same page as a self-contained HTML file. It asks the
questions the browser asks on load -- the frame, the elements, the tree -- and
writes the answers into the document alongside the page's own stylesheet and
script. Nothing is fetched when it opens, so the frame travels to whoever needs
to see it and outlives the program that drew it.

    tml capture -o frame.html                     # the running program
    tml capture app.tml --width 96 --height 26 -o frame.html

The reads work exactly as they do live: pick an element from the tree or click
it in the preview, and read its rect, content, clip, scroll and drawn text. The
writes are gone, and visibly so -- a keystroke and a restyle both need the
program to lay them out again, so a capture leaves those controls out rather
than offering a button that cannot work.

`inspect.Snapshot` and `inspect.WriteCapture` are the same thing from Go, over
any `Handler`. `inspect.SnapshotFrame` takes a frame the caller already holds.

## The page works nothing out

Every number the page shows comes off the protocol, and that includes which
element a cell belongs to. `op=at` answers one cell; `op=hits` answers every
cell of the frame in one response, as an index into the elements in document
order. A capture carries that map, so a click there is a lookup.

The alternative was a hit test written again in the page — the same rule about
screen rects and clips, in a second language, free to drift from the engine.
Compiling the Go to wasm would have kept one implementation and cost 9.1 MB
(2.8 MB gzipped) for the reads alone, which a capture cannot carry.

The page's scripts are TypeScript in `inspect/ui/src`, compiled by
[ts0](https://github.com/wow-look-at-my/ts0) into `inspect/ui/inspector.js`:

    pnpm install && pnpm build

That output is committed, because `go:embed` reads the module's own tree and
`go get` has to find it there. CI recompiles it and fails on any diff, so the
committed file cannot drift from its source.

## Overrides

`layout.Options.Override` is a function from id to attributes. The engine merges
what it returns into the element's attributes before inline styles and before
the stylesheet resolves, so an override wins over both and is laid out rather
than painted over. That is why a drag reflows the document instead of moving a
picture.

## What checks it

`tools/inspector-check/check.mjs` runs `build/agent` under a pty, asks the CLI
the questions above, then drives the browser with a pointer and reads the result
back off the socket. It then captures the same program and opens the file from
disk, where a request to anything but that file fails the check. It runs in CI.
Starting the agent with `tea.NewProgram` instead of `tml.Run` is enough to turn
it red.
