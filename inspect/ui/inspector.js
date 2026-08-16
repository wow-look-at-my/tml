// The inspector page. It reads the running program's frame over the same RPC
// the CLI uses, draws the terminal, and lets you pick an element apart.
//
// Every number on screen comes from the program's own layout. Nothing here
// re-measures anything: a preview that computed its own geometry would agree
// with itself and disagree with the terminal.

import { toHTML } from "./ansi.js";

const $ = (id) => document.getElementById(id);
const preview = $("preview");
const highlight = $("highlight");
const treeBox = $("tree");
const detail = $("detail");
const status = $("status");

let selected = null;
let elements = [];
let cell = { w: 8, h: 16 };
let drag = null;

async function rpc(body) {
	const res = await fetch("rpc", {
		method: "POST",
		headers: { "content-type": "application/json" },
		body: JSON.stringify(body),
	});
	const answer = await res.json();
	if (answer.error) throw new Error(answer.error);
	return answer;
}

function setStatus(text, state) {
	status.textContent = text;
	status.dataset.state = state;
}

// measureCell reads the real glyph box out of the rendered preview, so a
// pointer position converts to a terminal cell exactly. Guessing the size
// would put every click one column off at some font size.
function measureCell() {
	const probe = document.createElement("span");
	probe.textContent = "X".repeat(10);
	probe.style.position = "absolute";
	probe.style.visibility = "hidden";
	preview.appendChild(probe);
	const box = probe.getBoundingClientRect();
	probe.remove();
	if (box.width > 0) cell = { w: box.width / 10, h: box.height };
}

function drawFrame(frame) {
	preview.innerHTML = toHTML(frame.ansi ?? frame.text ?? "");
	$("frame-meta").textContent = `#${frame.seq} · ${frame.width}x${frame.height}`;
	measureCell();
	placeHighlight();
}

// placeHighlight puts the outline over the selected element, in cell units
// scaled by the measured glyph box.
function placeHighlight() {
	const el = elements.find((e) => e.id === selected);
	if (!el) {
		highlight.hidden = true;
		return;
	}
	const pad = 8; // matches #preview's padding
	highlight.hidden = false;
	highlight.style.left = `${pad + el.rect.x * cell.w}px`;
	highlight.style.top = `${pad + el.rect.y * cell.h}px`;
	highlight.style.width = `${Math.max(1, el.rect.w) * cell.w}px`;
	highlight.style.height = `${Math.max(1, el.rect.h) * cell.h}px`;
}

function cellAt(event) {
	const box = preview.getBoundingClientRect();
	const pad = 8;
	return {
		x: Math.floor((event.clientX - box.left - pad) / cell.w),
		y: Math.floor((event.clientY - box.top - pad) / cell.h),
	};
}

function nodeLabel(node) {
	const bits = [`<${node.element}>`];
	if (node.id) bits.push(`#${node.id}`);
	const geom = `${node.rect.w}x${node.rect.h}@${node.rect.x},${node.rect.y}`;
	return { head: bits.join(" "), geom, text: node.text ?? "" };
}

function renderTree(root) {
	treeBox.replaceChildren(buildList([root]));
}

function buildList(nodes) {
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
			if (node.id) select(node.id);
		});
		li.append(button);
		if (node.children?.length) li.append(buildList(node.children));
		ul.append(li);
	}
	return ul;
}

const FIELDS = [
	["element", (e) => e.element],
	["action", (e) => e.action || "-"],
	["rect", (e) => `x${e.rect.x} y${e.rect.y} ${e.rect.w}x${e.rect.h}`],
	["content", (e) => `${e.content.w}x${e.content.h}`],
	["clip", (e) => `x${e.clip.x} y${e.clip.y} ${e.clip.w}x${e.clip.h}`],
	["scroll", (e) => `${e.scroll.y}/${e.scroll.maxY} (x ${e.scroll.x}/${e.scroll.maxX})`],
	["focus", (e) => String(e.focus)],
];

function showDetail(el) {
	$("selected-id").textContent = el.id;
	detail.replaceChildren();
	for (const [label, read] of FIELDS) {
		const dt = document.createElement("dt");
		dt.textContent = label;
		const dd = document.createElement("dd");
		dd.textContent = read(el);
		detail.append(dt, dd);
	}
	$("element-text").textContent = el.text;
}

async function select(id) {
	selected = id;
	try {
		const { element } = await rpc({ op: "query", id });
		showDetail(element);
		for (const button of treeBox.querySelectorAll(".node")) {
			button.toggleAttribute("aria-current", button.dataset.id === id);
		}
		placeHighlight();
	} catch (err) {
		setStatus(err.message, "lost");
	}
}

async function refresh() {
	const [{ elements: got }, { tree }] = await Promise.all([
		rpc({ op: "elements" }),
		rpc({ op: "tree" }),
	]);
	elements = got;
	renderTree(tree);
	if (selected && elements.some((e) => e.id === selected)) await select(selected);
	else placeHighlight();
}

// Pointer handling: a plain click selects what is under it, and a drag on the
// selected element moves it. Holding shift resizes instead, which is the one
// gesture a terminal grid has room for.
preview.addEventListener("pointerdown", async (event) => {
	const at = cellAt(event);
	const el = elements.find((e) => e.id === selected);
	const inside = el && at.x >= el.rect.x && at.x < el.rect.x + el.rect.w && at.y >= el.rect.y && at.y < el.rect.y + el.rect.h;
	if (inside) {
		drag = { id: selected, from: at, rect: { ...el.rect }, resize: event.shiftKey };
		preview.setPointerCapture(event.pointerId);
		return;
	}
	try {
		const hit = await rpc({ op: "at", x: at.x, y: at.y });
		if (hit.found) await select(hit.hit);
	} catch (err) {
		setStatus(err.message, "lost");
	}
});

preview.addEventListener("pointermove", async (event) => {
	if (!drag) return;
	const at = cellAt(event);
	const dx = at.x - drag.from.x;
	const dy = at.y - drag.from.y;
	if (dx === 0 && dy === 0) return;
	drag.from = at;

	const attrs = drag.resize
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
		setStatus(err.message, "lost");
		drag = null;
	}
});

preview.addEventListener("pointerup", (event) => {
	if (drag) preview.releasePointerCapture(event.pointerId);
	drag = null;
});

$("restyle").addEventListener("submit", async (event) => {
	event.preventDefault();
	if (!selected) return;
	const form = new FormData(event.target);
	const attr = String(form.get("attr") ?? "").trim();
	if (!attr) return;
	try {
		await rpc({ op: "restyle", id: selected, attrs: { [attr]: String(form.get("value") ?? "") } });
		setStatus(`applied ${attr} to ${selected}`, "live");
	} catch (err) {
		setStatus(err.message, "lost");
	}
});

$("reset").addEventListener("click", async () => {
	try {
		await rpc({ op: "reset" });
		setStatus("overrides dropped", "live");
	} catch (err) {
		setStatus(err.message, "lost");
	}
});

$("keys").addEventListener("submit", async (event) => {
	event.preventDefault();
	const key = $("key").value.trim();
	if (!key) return;
	try {
		await rpc({ op: "key", key });
		setStatus(`sent ${key}`, "live");
	} catch (err) {
		setStatus(err.message, "lost");
	}
});

// The stream is the clock. Every frame the program paints re-reads the
// elements, so a drag lands and the next frame shows where it landed.
const stream = new EventSource("events");
stream.addEventListener("frame", async (event) => {
	const frame = JSON.parse(event.data);
	drawFrame(frame);
	setStatus(`live · frame ${frame.seq}`, "live");
	if ($("follow").checked) await refresh();
});
stream.addEventListener("error", () => setStatus("connection lost", "lost"));

(async () => {
	try {
		const { frame } = await rpc({ op: "frame", ansi: true });
		drawFrame(frame);
		await refresh();
		setStatus(`live · frame ${frame.seq}`, "live");
	} catch (err) {
		setStatus(err.message, "lost");
	}
})();
