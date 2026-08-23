# Command-line tests for the tml binary. go-toolchain runs every *.dats
# under dats/ after the build; $GO_TOOLCHAIN_DATS_BUILD_DIR holds a copy of
# that binary. Live commands talk to a throwaway unix socket served by
# inspect_server.mjs, which answers one inspect.Request with canned JSON.

shared:
	copy:
		inspect_server.mjs: inspect_server.mjs
# inspect_server.py, which answers one inspect.Request with canned JSON.
#
# image is the docker-backend fallback (bwrap and seatbelt ignore it): the
# live tests need python3, which debian:stable-slim does not ship.

sandbox:
	image: python:3-slim

shared:
	copy:
		inspect_server.py: inspect_server.py
	files:
		ids.json: |
			{"ids":["composer","status"]}
		query.json: |
			{"element":{"id":"prompt","element":"Textbox","text":"ask for a change","rect":{"x":1,"y":2,"w":88,"h":1},"content":{"w":0,"h":0},"clip":{"x":0,"y":0,"w":0,"h":0},"scroll":{"x":0,"y":0,"maxX":0,"maxY":0}}}
		tree.json: |
			{"tree":{"element":"Stack","id":"app","rect":{"x":0,"y":0,"w":80,"h":24},"content":{"w":0,"h":0},"clip":{"x":0,"y":0,"w":0,"h":0},"children":[{"element":"Text","id":"status","text":"ready","rect":{"x":2,"y":3,"w":10,"h":1},"content":{"w":0,"h":0},"clip":{"x":0,"y":0,"w":0,"h":0}}]}}

setup:
	- test -x "$GO_TOOLCHAIN_DATS_BUILD_DIR/tml"

tests:
	- desc: help lists file and live commands
	  cmd: '"$GO_TOOLCHAIN_DATS_BUILD_DIR/tml" --help'
	  outputs:
		stdout:
			- "  check "
			- "  tree "
			- "  render "
			- "  inspect "
			- "  query "
			- "  elements "
			- "  ids "
			- "  at "
			- "  frame "
			- "  input "
			- "  restyle "
			- "  serve "
			- "  list "
		"!stdout":
			- tml-test

	- desc: check accepts a file
	  cmd: '"$GO_TOOLCHAIN_DATS_BUILD_DIR/tml" check testdata/inspect/app.tml'
	  inputs:
		env:
			TML_INSPECT_DIR: "{outputs.sockets}"
	  outputs:
		stdout:
			- "ok: testdata/inspect/app.tml"

	- desc: tree with a file prints the expanded document
	  cmd: '"$GO_TOOLCHAIN_DATS_BUILD_DIR/tml" tree testdata/inspect/app.tml --prop title=STATUS'
	  inputs:
		env:
			TML_INSPECT_DIR: "{outputs.sockets}"
	  outputs:
		stdout:
			- header
			- STATUS

	- desc: render prints a frame
	  cmd: '"$GO_TOOLCHAIN_DATS_BUILD_DIR/tml" render testdata/inspect/app.tml --width 40 --height 8 --prop title=STATUS --prop rows=one'
	  inputs:
		env:
			TML_INSPECT_DIR: "{outputs.sockets}"
	  outputs:
		stdout:
			- STATUS

	- desc: inspect --id header is JSON for that element
	  cmd: '"$GO_TOOLCHAIN_DATS_BUILD_DIR/tml" inspect testdata/inspect/app.tml --width 40 --height 8 --prop title=STATUS --prop rows=one,two,three --id header'
	  inputs:
		env:
			TML_INSPECT_DIR: "{outputs.sockets}"
	  outputs:
		stdout:
			- '"id": "header"'

	- desc: ids fails when no program is running
	  cmd: 'dir=$(mktemp -d); TML_INSPECT_SOCKET= TML_INSPECT_DIR="$dir" "$GO_TOOLCHAIN_DATS_BUILD_DIR/tml" ids; rc=$?; rmdir "$dir"; exit $rc'
	  exit: 1
	  outputs:
		stderr:
			- "--socket"
			- TML_INSPECT_SOCKET
			- "no TML program is running"

	- desc: query fails when no program is running
	  cmd: 'dir=$(mktemp -d); TML_INSPECT_SOCKET= TML_INSPECT_DIR="$dir" "$GO_TOOLCHAIN_DATS_BUILD_DIR/tml" query --id composer; rc=$?; rmdir "$dir"; exit $rc'
	  exit: 1
	  outputs:
		stderr:
			- "--socket"
			- TML_INSPECT_SOCKET
			- "no TML program is running"

	- desc: ids prints what the socket answers
	  cmd: 'sock={outputs.inspect.sock}; node {shared.inspect_server.mjs} "$sock" {shared.ids.json} & pid=$!; n=0; while [ "$n" -lt 50 ] && [ ! -S "$sock" ]; do n=$((n+1)); sleep 0.05; done; "$GO_TOOLCHAIN_DATS_BUILD_DIR/tml" ids --socket "$sock"; rc=$?; kill "$pid" 2>/dev/null; wait "$pid" 2>/dev/null; exit $rc'
	  cmd: 'sock={outputs.inspect.sock}; python3 {shared.inspect_server.py} "$sock" {shared.ids.json} & pid=$!; n=0; while [ "$n" -lt 50 ] && [ ! -S "$sock" ]; do n=$((n+1)); sleep 0.05; done; "$GO_TOOLCHAIN_DATS_BUILD_DIR/tml" ids --socket "$sock"; rc=$?; kill "$pid" 2>/dev/null; wait "$pid" 2>/dev/null; exit $rc'
	  timeout: 10s
	  outputs:
		stdout:
			- composer
			- status

	- desc: query --field text prints the element's text
	  cmd: 'sock={outputs.inspect.sock}; node {shared.inspect_server.mjs} "$sock" {shared.query.json} & pid=$!; n=0; while [ "$n" -lt 50 ] && [ ! -S "$sock" ]; do n=$((n+1)); sleep 0.05; done; "$GO_TOOLCHAIN_DATS_BUILD_DIR/tml" query --socket "$sock" --id prompt --field text; rc=$?; kill "$pid" 2>/dev/null; wait "$pid" 2>/dev/null; exit $rc'
	  cmd: 'sock={outputs.inspect.sock}; python3 {shared.inspect_server.py} "$sock" {shared.query.json} & pid=$!; n=0; while [ "$n" -lt 50 ] && [ ! -S "$sock" ]; do n=$((n+1)); sleep 0.05; done; "$GO_TOOLCHAIN_DATS_BUILD_DIR/tml" query --socket "$sock" --id prompt --field text; rc=$?; kill "$pid" 2>/dev/null; wait "$pid" 2>/dev/null; exit $rc'
	  timeout: 10s
	  outputs:
		stdout:
			- "ask for a change"

	- desc: tree with no file prints the live frame
	  cmd: 'sock={outputs.inspect.sock}; node {shared.inspect_server.mjs} "$sock" {shared.tree.json} & pid=$!; n=0; while [ "$n" -lt 50 ] && [ ! -S "$sock" ]; do n=$((n+1)); sleep 0.05; done; "$GO_TOOLCHAIN_DATS_BUILD_DIR/tml" tree --socket "$sock"; rc=$?; kill "$pid" 2>/dev/null; wait "$pid" 2>/dev/null; exit $rc'
	  cmd: 'sock={outputs.inspect.sock}; python3 {shared.inspect_server.py} "$sock" {shared.tree.json} & pid=$!; n=0; while [ "$n" -lt 50 ] && [ ! -S "$sock" ]; do n=$((n+1)); sleep 0.05; done; "$GO_TOOLCHAIN_DATS_BUILD_DIR/tml" tree --socket "$sock"; rc=$?; kill "$pid" 2>/dev/null; wait "$pid" 2>/dev/null; exit $rc'
	  timeout: 10s
	  outputs:
		stdout:
			- "<Stack> #app"
			- "<Text> #status"
			- ready
