// Turns a terminal frame into HTML spans, one span per run of identical
// styling. The preview has to show what the terminal shows: an inspector that
// renders a program's colours differently from the terminal is an inspector
// that lies about the thing it is inspecting.

const BASE = [
	"#000000", "#cd3131", "#0dbc79", "#e5e510",
	"#2472c8", "#bc3fbc", "#11a8cd", "#e5e5e5",
	"#666666", "#f14c4c", "#23d18b", "#f5f543",
	"#3b8eea", "#d670d6", "#29b8db", "#e5e5e5",
];

// xterm256 is the 256-colour cube plus the grey ramp, computed rather than
// tabulated: the formula is the definition, and a table would be 240 lines
// that can drift from it.
function xterm256(n) {
	if (n < 16) return BASE[n];
	if (n < 232) {
		const c = n - 16;
		const level = (v) => (v === 0 ? 0 : 55 + v * 40);
		return rgb(level(Math.floor(c / 36) % 6), level(Math.floor(c / 6) % 6), level(c % 6));
	}
	const grey = 8 + (n - 232) * 10;
	return rgb(grey, grey, grey);
}

function rgb(r, g, b) {
	const hex = (v) => Math.max(0, Math.min(255, v)).toString(16).padStart(2, "0");
	return `#${hex(r)}${hex(g)}${hex(b)}`;
}

function blank() {
	return { fg: null, bg: null, bold: false, dim: false, italic: false, underline: false, reverse: false };
}

// apply reads one SGR sequence into the running style. Unknown parameters are
// skipped rather than guessed at.
function apply(style, params) {
	for (let i = 0; i < params.length; i++) {
		const p = params[i];
		if (p === 0) Object.assign(style, blank());
		else if (p === 1) style.bold = true;
		else if (p === 2) style.dim = true;
		else if (p === 3) style.italic = true;
		else if (p === 4) style.underline = true;
		else if (p === 7) style.reverse = true;
		else if (p === 22) { style.bold = false; style.dim = false; }
		else if (p === 23) style.italic = false;
		else if (p === 24) style.underline = false;
		else if (p === 27) style.reverse = false;
		else if (p >= 30 && p <= 37) style.fg = BASE[p - 30];
		else if (p >= 90 && p <= 97) style.fg = BASE[p - 90 + 8];
		else if (p >= 40 && p <= 47) style.bg = BASE[p - 40];
		else if (p >= 100 && p <= 107) style.bg = BASE[p - 100 + 8];
		else if (p === 39) style.fg = null;
		else if (p === 49) style.bg = null;
		else if (p === 38 || p === 48) {
			const target = p === 38 ? "fg" : "bg";
			if (params[i + 1] === 5) { style[target] = xterm256(params[i + 2] ?? 0); i += 2; }
			else if (params[i + 1] === 2) { style[target] = rgb(params[i + 2], params[i + 3], params[i + 4]); i += 4; }
		}
	}
}

function css(style) {
	const parts = [];
	const fg = style.reverse ? style.bg ?? "#000000" : style.fg;
	const bg = style.reverse ? style.fg ?? "#cccccc" : style.bg;
	if (fg) parts.push(`color:${fg}`);
	if (bg) parts.push(`background:${bg}`);
	if (style.bold) parts.push("font-weight:700");
	if (style.dim) parts.push("opacity:.6");
	if (style.italic) parts.push("font-style:italic");
	if (style.underline) parts.push("text-decoration:underline");
	return parts.join(";");
}

const ESCAPE = /\x1b\[([0-9;:]*)m/g;

// toHTML converts one frame. Non-SGR escapes are dropped: they move a cursor
// the preview does not have, and leaving them in would print as mojibake.
export function toHTML(text) {
	const style = blank();
	let html = "";
	let at = 0;
	ESCAPE.lastIndex = 0;
	for (let m = ESCAPE.exec(text); m; m = ESCAPE.exec(text)) {
		html += span(text.slice(at, m.index), style);
		apply(style, m[1].split(/[;:]/).map((v) => (v === "" ? 0 : Number(v))));
		at = m.index + m[0].length;
	}
	html += span(text.slice(at), style);
	// Anything left is a cursor move or a mode set, neither of which the
	// preview can honour.
	return html.replace(/\x1b\[[0-9;?]*[A-Za-z]/g, "");
}

function span(chunk, style) {
	if (chunk === "") return "";
	const escaped = chunk
		.replaceAll("&", "&amp;")
		.replaceAll("<", "&lt;")
		.replaceAll(">", "&gt;");
	const rule = css(style);
	return rule ? `<span style="${rule}">${escaped}</span>` : escaped;
}
