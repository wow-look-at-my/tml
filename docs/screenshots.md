# Screenshots

`tools/shots` runs each example in a real terminal and takes a picture of it. ttyd serves the program on a PTY. Chromium renders it through xterm.js. The picture shows that terminal. `tools/shots/shots.mjs` lists what to run and what state to get it into. `capture.mjs` is the machinery.

```bash
go-toolchain                                   # build the examples first
npm install --prefix tools/shots
node tools/shots/capture.mjs                   # -> build/shots/*.png and index.html
```

The tool needs `ttyd` and a Chromium on the machine. It looks for the browser in a few usual places, and says which ones it tried. `CHROME=/path/to/chrome` overrides that.

## Why a terminal and not a frame

Both examples already render one frame to stdout with `-frame`, and `testdata/*.golden` pins that text. That is the mechanical check. It stays the one CI fails on. But a frame is a string. It proves nothing about whether the program draws in a terminal, answers a key press from a keyboard, or picks colours a terminal can show. The image fallback is the clearest case.

`<Image>` asks the terminal what it is, then chooses between the kitty protocol, half-blocks and a link. In a golden the result is text either way.

So the screenshots cover what the goldens cannot, and typed keys drive one of them (`keys` in the shot list) to keep the input path honest.

## What a shot declares

- `args` put the program in a state before the terminal is attached. Both examples take their starting state either way round -- `-tab`, `-focus`, `-steps`, `-answer` -- which is what makes a picture reproducible.
- `keys` are typed into the running terminal afterwards.
- `expect` is text the terminal has to be showing before the picture counts. A program that died on its arguments still leaves a terminal to photograph. A black rectangle published as a screenshot is worse than a failed build.
- `rows` is the terminal to shoot in. These layouts fill the terminal they are given. A page shot in more rows than it has content for is mostly empty space. One shot in fewer rows drops whatever was pinned to the bottom.

The terminal is sized exactly. The tool measures a cell and the page's own chrome, then resizes the window to fit. It fails when the terminal does not come out the size it asked for. Without that, every picture is whatever size the browser opened at.

## Publishing

CI captures on every push and publishes `build/shots/` to a buildhost site, per branch:

- `https://sites.pazer.build/tml/` -- master
- `https://sites.pazer.build/tml/@<branch>/` -- any other branch

Until master published once, the bare project URL falls back to whichever branch does have a site. The README's pictures therefore work before the first merge, and switch to master's afterwards.

The project is the repository's name, because that is what buildhost's OIDC trust covers. A token minted for `repo:wow-look-at-my/tml` writes project `tml` and anything namespaced beneath it. Any other name answers `404 project not found` rather than creates the project.

A branch build's index page shows each picture beside master's copy of it. What a change did to the rendering is therefore a scroll, not an archaeology exercise. The README embeds master's, which is where the showcase comes from.

The site is published with `public: true`. tml is a private repo. GitHub's image proxy cannot fetch a picture it needs a credential for. Without `public: true` the README shows nothing. What lands there is pictures of example programs, and the index page that lists them. That directory is the whole privacy boundary. Nothing else may be written into it.

There is deliberately no pixel comparison. Font rasterisation differs between machines. A pixel gate therefore fails on where it ran, not on what changed. The goldens are the mechanical check. These pictures are for the eye.
