# Screenshots

`tools/shots` runs each example in a real terminal and takes a picture of it: ttyd serves the program on a PTY, Chromium
renders it through xterm.js, and the picture is of that terminal. `tools/shots/shots.mjs` is the list of what to run and
what state to get it into; `capture.mjs` is the machinery.

```bash
go-toolchain                                   # build the examples first
npm install --prefix tools/shots
node tools/shots/capture.mjs                   # -> build/shots/*.png and index.html
```

The tool needs `ttyd` and a Chromium on the machine. It looks for the browser in a few usual places and says which ones
it tried; `CHROME=/path/to/chrome` overrides that.

## Why a terminal and not a frame

Both examples already render one frame to stdout with `-frame`, and `testdata/*.golden` pins that text. That is the
mechanical check, and it stays the one CI fails on. But a frame is a string: it proves nothing about whether the program
draws in a terminal, answers a key press that came from a keyboard, or picks colours a terminal can actually show. The
image fallback is the clearest case -- `<Image>` chooses between the kitty protocol, half-blocks and a link by asking
the terminal what it is, and in a golden it is text either way.

So the screenshots cover what the goldens cannot, and one of them is driven entirely by typed keys (`keys` in the shot
list) to keep the input path honest.

## What a shot declares

- `args` put the program in a state before the terminal is attached. Both examples take their starting state either way
  round -- `-tab`, `-focus`, `-steps`, `-answer` -- which is what makes a picture reproducible.
- `keys` are typed into the running terminal afterwards.
- `expect` is text the terminal has to be showing before the picture counts. A program that died on its arguments still
  leaves a terminal to photograph, and a black rectangle published as a screenshot is worse than a failed build.
- `rows` is the terminal to shoot in. These layouts fill the terminal they are given, so a page shot in more rows than it
  has content for is mostly empty space, and one shot in fewer drops whatever was pinned to the bottom.

The terminal is sized exactly: the tool measures a cell and the page's own chrome, resizes the window to fit, and fails
if the terminal did not come out the size it asked for. Without that, every picture would be whatever size the browser
happened to open at.

## Publishing

CI captures on every push and publishes `build/shots/` to a buildhost site, per branch:

- `https://sites.pazer.build/tml-shots/` -- master
- `https://sites.pazer.build/tml-shots/@<branch>/` -- any other branch

A branch build's index page shows each picture beside master's copy of it, so what a change did to the rendering is a
scroll rather than an archaeology exercise. The README embeds master's, which is where the showcase comes from.

The site is published with `public: true`. tml is a private repo, and GitHub's image proxy cannot fetch a picture it
needs a credential for, so the README would show nothing without it. What lands there is pictures of example programs
and the index page listing them -- that directory is the whole privacy boundary, so nothing else may be written into it.

There is deliberately no pixel comparison. Font rasterisation differs between machines, so a pixel gate would fail on
where it ran rather than on what changed; the goldens are the mechanical check, and these are for the eye.
