// The inspector page. It reads the running program's frame over the same RPC
// the CLI uses, draws the terminal, and lets you pick an element apart.
//
// Every number on screen comes from the program's own layout. Nothing here
// re-measures anything, and nothing here decides what covers a cell: a page
// that worked either out for itself would agree with itself and disagree with
// the terminal.
//
// A capture is the same page over a frozen frame: `tml capture` writes the
// answers into the document, and the reads below come from there instead of
// from a socket. The page is one page either way, so a capture cannot show a
// different thing from the inspector it was captured out of.

import { toHTML } from "./ansi.ts";
import type { Capture, Element, FrameInfo, Node, Request, Response } from "./protocol.ts";

// The page and this script ship together, so a missing id is a broken build
// rather than a state to render around.
function need<T extends HTMLElement>(id: string): T {
	const el = document.getElementById(id);
	if (!el) throw new Error(`the inspector page has no #${id}`);
	return el as T;
}

const preview = need("preview");
const highlight = need("highlight");
const treeBox = need("tree");
const detail = need("detail");
const status = need("status");

interface Point {
	x: number;
	y: number;
}

interface Drag {
	id: string;
	from: Point;
	rect: { x: number; y: number; w: number; h: number };
	resize: boolean;
}

let selected: string | null = null;
let elements: Element[] = [];
let cell = { w: 8, h: 16 };
let pad: Point = { x: 8, y: 8 };
let drag: Drag | null = null;

// capture is the frozen frame this page was written around, or null on the
// live page.
const capture: Capture | null = (() => {
	const held = document.getElementById("capture");
	return held ? (JSON.parse(held.textContent ?? "") as Capture) : null;
})();

async function rpc(body: Request): Promise<Response> {
	if (capture) return answerFromCapture(capture, body);
	const res = await fetch("rpc", {
		method: "POST",
		headers: { "content-type": "application/json" },
		body: JSON.stringify(body),
	});
	const answer = (await res.json()) as Response;
	if (answer.error) throw new Error(answer.error);
	return answer;
}

// answerFromCapture serves the read operations out of the frozen answers. A
// write says what it needs rather than doing nothing: a restyle is a real
// layout the program has to run.
//
// Nothing here works anything out. A cell is resolved by looking up the answer
// the program's own hit test already gave for it, because a rule restated here
// is a rule free to disagree with the one in the engine.
function answerFromCapture(held: Capture, req: Request): Response {
	switch (req.op) {
		case "frame":
			return { hit: "", frame: held.frame };
		case "elements":
			return { hit: "", elements: held.elements };
		case "tree":
			return { hit: "", tree: held.tree };
		case "query": {
			const element = held.elements.find((e) => e.id === req.id);
			if (!element) throw new Error(`no element has id ${JSON.stringify(req.id)} in this capture`);
			return { hit: "", element };
		}
		case "at": {
			const index = held.hits[req.y ?? -1]?.[req.x ?? -1] ?? -1;
			const hit = index < 0 ? "" : held.elements[index].id;
			return { hit, found: hit !== "" };
		}
		default:
			throw new Error(`this is a capture, not a running program: ${req.op} needs one`);
	}
}

function setStatus(text: string, state: string): void {
	status.textContent = text;
	status.dataset.state = state;
}

function saidWhy(err: unknown): string {
	return err instanceof Error ? err.message : String(err);
}

// measureCell reads the real cell box out of the rendered preview, so a cell
// converts to a pixel exactly. Guessing would put every click a column off at
// some font size.
//
// The row pitch is the LINE box, not the glyph box: a span reports the font's
// own height, which is shorter than the line it sits on, and the difference
// compounds down the frame until the outline is a row above the element.
function measureCell(): void {
	const probe = document.createElement("span");
	probe.textContent = "X".repeat(100);
	probe.style.cssText = "position:absolute;visibility:hidden;white-space:pre;font:inherit";
	preview.appendChild(probe);
	const width = probe.getBoundingClientRect().width / 100;
	probe.remove();

	const style = getComputedStyle(preview);
	const height = parseFloat(style.lineHeight);
	if (width > 0 && height > 0) cell = { w: width, h: height };
	pad = { x: parseFloat(style.paddingLeft), y: parseFloat(style.paddingTop) };
}

// frameRows is the viewport the program painted, which is where the label runs
// out of room below the outline.
let frameRows = 0;

function drawFrame(frame: FrameInfo): void {
	preview.innerHTML = toHTML(frame.ansi ?? frame.text ?? "");
	need("frame-meta").textContent = `#${frame.seq} · ${frame.width}x${frame.height}`;
	frameRows = frame.height;
	measureCell();
	placeHighlight();
}

// placeHighlight puts the outline over the selected element, in cell units
// scaled by the measured cell box. It carries the element's name, because an
// outline on its own says where something is and not what it is.
function placeHighlight(): void {
	const el = elements.find((e) => e.id === selected);
	if (!el) {
		highlight.hidden = true;
		return;
	}
	highlight.hidden = false;
	highlight.style.left = `${pad.x + el.rect.x * cell.w}px`;
	highlight.style.top = `${pad.y + el.rect.y * cell.h}px`;
	highlight.style.width = `${Math.max(1, el.rect.w) * cell.w}px`;
	highlight.style.height = `${Math.max(1, el.rect.h) * cell.h}px`;
	highlight.dataset.label = `${el.element} #${el.id}  ${el.rect.w}x${el.rect.h}@${el.rect.x},${el.rect.y}`;
	// The label hangs below the outline, and moves inside on the bottom row so
	// it stays on the picture rather than under it.
	highlight.dataset.place = el.rect.y + el.rect.h >= frameRows - 1 ? "inside" : "below";
}

function cellAt(event: PointerEvent): Point {
	const box = preview.getBoundingClientRect();
	return {
		x: Math.floor((event.clientX - box.left - pad.x) / cell.w),
		y: Math.floor((event.clientY - box.top - pad.y) / cell.h),
	};
}

function nodeLabel(node: Node): { head: string; geom: string; text: string } {
	const bits = [`<${node.element}>`];
	if (node.id) bits.push(`#${node.id}`);
	const geom = `${node.rect.w}x${node.rect.h}@${node.rect.x},${node.rect.y}`;
	return { head: bits.join(" "), geom, text: node.text ?? "" };
}

function renderTree(root: Node): void {
	treeBox.replaceChildren(buildList([root]));
}

function buildList(nodes: Node[]): HTMLUListElement {
	const ul = document.createElement("ul");
	for (const node of nodes) {
		const li = document.createElement("li");
		const button = document.createElement("button");
		button.type = "button";
		button.className = "node";
		button.dataset.focus = String(node.focus);
		if (node.id) button.dataset.id = node.id;
		if (node.id === selected) button.setAttribute("aria-current", "true");

		const { head, geom, text } = nodeLabel(node);
		button.append(head + " ");
		const idSpan = document.createElement("span");
		idSpan.className = "id";
		idSpan.textContent = geom;
		button.append(idSpan);
		if (text) {
			const textSpan = document.createElement("span");
			textSpan.className = "text";
			textSpan.textContent = ` ${text}`;
			button.append(textSpan);
		}

		button.addEventListener("click", () => {
			if (node.id) void select(node.id);
		});
		li.append(button);
		if (node.children?.length) li.append(buildList(node.children));
		ul.append(li);
	}
	return ul;
}

const FIELDS: [string, (e: Element) => string][] = [
	["element", (e) => e.element],
	["action", (e) => e.action || "-"],
	["rect", (e) => `x${e.rect.x} y${e.rect.y} ${e.rect.w}x${e.rect.h}`],
	["content", (e) => `${e.content.w}x${e.content.h}`],
	["clip", (e) => `x${e.clip.x} y${e.clip.y} ${e.clip.w}x${e.clip.h}`],
	["scroll", (e) => `${e.scroll.y}/${e.scroll.maxY} (x ${e.scroll.x}/${e.scroll.maxX})`],
	["focus", (e) => String(e.focus)],
];

function showDetail(el: Element): void {
	need("selected-id").textContent = el.id;
	detail.replaceChildren();
	for (const [label, read] of FIELDS) {
		const dt = document.createElement("dt");
		dt.textContent = label;
		const dd = document.createElement("dd");
		dd.textContent = read(el);
		detail.append(dt, dd);
	}
	need("element-text").textContent = el.text;
}

async function select(id: string): Promise<void> {
	selected = id;
	try {
		const { element } = await rpc({ op: "query", id });
		if (element) showDetail(element);
		for (const button of treeBox.querySelectorAll<HTMLButtonElement>(".node")) {
			button.toggleAttribute("aria-current", button.dataset.id === id);
		}
		placeHighlight();
	} catch (err) {
		setStatus(saidWhy(err), "lost");
	}
}

async function refresh(): Promise<void> {
	const [got, walked] = await Promise.all([rpc({ op: "elements" }), rpc({ op: "tree" })]);
	elements = got.elements ?? [];
	if (walked.tree) renderTree(walked.tree);
	if (selected && elements.some((e) => e.id === selected)) await select(selected);
	else placeHighlight();
}

// Pointer handling: a plain click selects what is under it, and a drag on the
// selected element moves it. Holding shift resizes instead, which is the one
// gesture a terminal grid has room for.
//
// Which element a cell belongs to is the program's answer, never this page's:
// the same op decides whether a press starts a drag and what a click selects.
preview.addEventListener("pointerdown", async (event: PointerEvent) => {
	const at = cellAt(event);
	try {
		const hit = await rpc({ op: "at", x: at.x, y: at.y });
		if (!hit.found) return;
		const el = elements.find((e) => e.id === hit.hit);
		if (hit.hit === selected && el && !capture) {
			drag = { id: hit.hit, from: at, rect: { ...el.rect }, resize: event.shiftKey };
			preview.setPointerCapture(event.pointerId);
			return;
		}
		await select(hit.hit);
	} catch (err) {
		setStatus(saidWhy(err), "lost");
	}
});

preview.addEventListener("pointermove", async (event: PointerEvent) => {
	if (!drag) return;
	const at = cellAt(event);
	const dx = at.x - drag.from.x;
	const dy = at.y - drag.from.y;
	if (dx === 0 && dy === 0) return;
	drag.from = at;

	const attrs: Record<string, string> = drag.resize
		? { width: String(Math.max(1, drag.rect.w + dx)), height: String(Math.max(1, drag.rect.h + dy)) }
		: { "margin-left": String(Math.max(0, drag.rect.x + dx)), "margin-top": String(Math.max(0, drag.rect.y + dy)) };
	if (drag.resize) {
		drag.rect.w = Math.max(1, drag.rect.w + dx);
		drag.rect.h = Math.max(1, drag.rect.h + dy);
	} else {
		drag.rect.x = Math.max(0, drag.rect.x + dx);
		drag.rect.y = Math.max(0, drag.rect.y + dy);
	}
	try {
		await rpc({ op: "restyle", id: drag.id, attrs });
	} catch (err) {
		setStatus(saidWhy(err), "lost");
		drag = null;
	}
});

preview.addEventListener("pointerup", (event: PointerEvent) => {
	if (drag) preview.releasePointerCapture(event.pointerId);
	drag = null;
});

need<HTMLFormElement>("restyle").addEventListener("submit", async (event: SubmitEvent) => {
	event.preventDefault();
	if (!selected) return;
	const form = new FormData(event.target as HTMLFormElement);
	const attr = String(form.get("attr") ?? "").trim();
	if (!attr) return;
	try {
		await rpc({ op: "restyle", id: selected, attrs: { [attr]: String(form.get("value") ?? "") } });
		setStatus(`applied ${attr} to ${selected}`, "live");
	} catch (err) {
		setStatus(saidWhy(err), "lost");
	}
});

need("reset").addEventListener("click", async () => {
	try {
		await rpc({ op: "reset" });
		setStatus("overrides dropped", "live");
	} catch (err) {
		setStatus(saidWhy(err), "lost");
	}
});

need<HTMLFormElement>("keys").addEventListener("submit", async (event: SubmitEvent) => {
	event.preventDefault();
	const key = need<HTMLInputElement>("key").value.trim();
	if (!key) return;
	try {
		await rpc({ op: "key", key });
		setStatus(`sent ${key}`, "live");
	} catch (err) {
		setStatus(saidWhy(err), "lost");
	}
});

// The stream is the clock. Every frame the program paints re-reads the
// elements, so a drag lands and the next frame shows where it landed. A
// capture has no clock: its one frame is already in the document.
if (!capture) {
	const stream = new EventSource("events");
	stream.addEventListener("frame", async (event: MessageEvent<string>) => {
		const frame = JSON.parse(event.data) as FrameInfo;
		drawFrame(frame);
		setStatus(`live · frame ${frame.seq}`, "live");
		if (need<HTMLInputElement>("follow").checked) await refresh();
	});
	stream.addEventListener("error", () => setStatus("connection lost", "lost"));
}

void (async () => {
	if (capture) document.body.dataset.capture = "true";
	try {
		const { frame } = await rpc({ op: "frame", ansi: true });
		if (!frame) throw new Error("the answer to op=frame carried no frame");
		drawFrame(frame);
		await refresh();
		setStatus(capture ? `capture · frame ${frame.seq} · ${frame.at}` : `live · frame ${frame.seq}`, "live");
	} catch (err) {
		setStatus(saidWhy(err), "lost");
	}
})();
