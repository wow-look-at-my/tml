// capture drives each shot in a real terminal -- ttyd serving a PTY, xterm.js
// in Chromium -- and writes a PNG of it, plus an index page for the site the
// pictures are published to.
//
// A real terminal is the point. The examples can already render a frame with
// -frame, and the goldens pin that text, but a frame is not proof that a
// program draws in a terminal, answers a key press, or picks colours a terminal
// can show. see docs/screenshots.md
import { spawn } from "node:child_process";
import { existsSync } from "node:fs";
import { mkdir, writeFile } from "node:fs/promises";
import net from "node:net";
import path from "node:path";
import process from "node:process";

import { chromium } from "playwright-core";

import { shots } from "./shots.mjs";

// The terminal every shot is taken in, unless it asks for its own.
const terminal = { cols: 96, rows: 30 };
const repo = path.resolve(import.meta.dirname, "../..");
const out = path.join(repo, "build/shots");

// The site the pictures are published to. A branch build shows its own picture
// beside the default branch's, which is the whole diff check.
const site = "https://sites.pazer.build/tml-shots";

// The browser to drive. playwright-core ships no browser of its own, so this
// uses whichever one the machine has -- and says which ones it looked for
// rather than failing somewhere further in.
const candidates = [
	process.env.CHROME,
	"/opt/pw-browsers/chromium",
	"/usr/bin/chromium",
	"/usr/bin/chromium-browser",
	"/usr/bin/google-chrome",
].filter(Boolean);

function browserPath() {
	const found = candidates.find((path) => existsSync(path));
	if (!found) {
		throw new Error(`no browser found; looked for ${candidates.join(", ")}. Install one, or point CHROME at it.`);
	}
	return found;
}

async function freePort() {
	const server = net.createServer();
	await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
	const { port } = server.address();
	await new Promise((resolve) => server.close(resolve));
	return port;
}

// serve starts ttyd on the command, in the DOM renderer so the terminal's text
// is in the page: the capture waits on what was actually drawn rather than on a
// timer, and a canvas would leave it with nothing to look at.
async function serve(shot) {
	const port = await freePort();
	const ttyd = spawn("ttyd", [
		"-p", String(port),
		"-W",
		"-t", "rendererType=dom",
		"-t", "fontSize=15",
		"-t", "fontFamily=DejaVu Sans Mono,monospace",
		"-t", "theme={\"background\":\"#12131a\"}",
		path.join(repo, shot.command),
		...(shot.args ?? []),
	], { cwd: repo, stdio: "ignore" });

	await waitFor(() => fetch(`http://127.0.0.1:${port}/`).then((r) => r.ok, () => false), `ttyd on ${port}`);
	return { port, stop: () => ttyd.kill("SIGTERM") };
}

async function waitFor(check, what, timeout = 15000) {
	const deadline = Date.now() + timeout;
	while (Date.now() < deadline) {
		if (await check()) {
			return;
		}
		await new Promise((r) => setTimeout(r, 100));
	}
	throw new Error(`timed out waiting for ${what}`);
}

// size makes the terminal exactly cols x rows. xterm sizes itself to the window,
// so the window is what has to be worked out: measure a cell, measure what the
// page puts around the terminal, then resize to fit. Anything else leaves the
// picture at whatever size the browser happened to open at, and a layout that
// fills its terminal would differ every run.
async function size(page, cols, rows) {
	const metrics = async () => page.evaluate(() => {
		const measure = document.querySelector(".xterm-char-measure-element");
		const screen = document.querySelector(".xterm-screen");
		const rows = document.querySelector(".xterm-rows");
		return {
			cellW: measure.getBoundingClientRect().width / measure.textContent.length,
			cellH: measure.getBoundingClientRect().height,
			screenW: screen.getBoundingClientRect().width,
			viewW: window.innerWidth,
			viewH: window.innerHeight,
			rows: rows.children.length,
		};
	});

	const before = await metrics();
	await page.setViewportSize({
		width: Math.ceil(cols * before.cellW) + (before.viewW - before.screenW) + 1,
		height: Math.ceil(rows * before.cellH) + 16,
	});
	await page.waitForTimeout(700);

	const after = await metrics();
	const got = Math.floor(after.screenW / after.cellW);
	if (got !== cols || after.rows !== rows) {
		throw new Error(`terminal came out ${got}x${after.rows}, wanted ${cols}x${rows}`);
	}
}

// capture takes one picture. It waits for the program to have painted -- a
// terminal that has not drawn yet is a black rectangle, and a fixed sleep is a
// guess that goes stale on a slower machine.
async function capture(browser, shot) {
	if (!shot.expect) {
		throw new Error(`shot ${shot.name} has no expect: there would be nothing to check the picture against`);
	}
	const { port, stop } = await serve(shot);
	const page = await browser.newPage({ viewport: { width: 1200, height: 700 }, deviceScaleFactor: 2 });
	// xterm pads a row with non-breaking spaces, which read as ordinary spaces
	// and match nothing. Everything here compares against what the picture
	// looks like, so the text has to say what it looks like.
	const screen = async () => (await page.locator(".xterm-rows").innerText()).replaceAll("\u00a0", " ");
	try {
		await page.goto(`http://127.0.0.1:${port}/`);
		await page.waitForSelector(".xterm-rows");
		await waitFor(async () => (await screen()).trim().length > 0, `${shot.name} to draw`);
		await size(page, shot.cols ?? terminal.cols, shot.rows ?? terminal.rows);

		for (const key of shot.keys ?? []) {
			await page.keyboard.press(key);
			await page.waitForTimeout(60);
		}
		// The examples animate on a tick, so a spinner is mid-frame whenever the
		// picture is taken. One settled frame after the last key press is as still
		// as this gets.
		await page.waitForTimeout(400);
		try {
			await waitFor(async () => (await screen()).includes(shot.expect), shot.expect, 5000);
		} catch {
			throw new Error(`${shot.name} never showed ${JSON.stringify(shot.expect)}; the terminal held:\n${await screen()}`);
		}

		const file = path.join(out, `${shot.name}.png`);
		await page.locator(".xterm-screen").screenshot({ path: file });
		console.log(`wrote ${path.relative(repo, file)}`);
		return { ...shot, file: `${shot.name}.png` };
	} finally {
		await page.close();
		stop();
	}
}

const escape = (s) => s.replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" })[c]);

// index is the page the site serves. On a branch every picture sits beside the
// default branch's copy of it, so what changed is a scroll rather than an
// archaeology exercise.
function index(taken, branch) {
	const compare = branch !== "" && branch !== "master";
	const figures = taken.map((shot) => `
	<figure>
		<h2>${escape(shot.title)}</h2>
		<p>${escape(shot.caption)}</p>
		<div class="pair">
			<div><img src="${shot.file}" alt="${escape(shot.title)}"><figcaption>${escape(branch || "this build")}</figcaption></div>
			${compare ? `<div><img src="${site}/${shot.file}" alt="${escape(shot.title)} on master"><figcaption>master</figcaption></div>` : ""}
		</div>
	</figure>`).join("\n");

	return `<!doctype html>
<html lang="en">
<meta charset="utf-8">
<title>tml screenshots${branch ? ` -- ${escape(branch)}` : ""}</title>
<style>
	:root { color-scheme: dark; }
	body { margin: 0 auto; padding: 32px 20px 64px; max-width: 1400px; background: #0b0c10; color: #c8d3f5;
	       font: 15px/1.5 ui-sans-serif, system-ui, sans-serif; }
	h1 { font-size: 22px; margin: 0 0 4px; }
	header p { color: #7f8497; margin: 0 0 32px; }
	figure { margin: 0 0 40px; }
	h2 { font-size: 17px; margin: 0 0 2px; }
	figure p { color: #7f8497; margin: 0 0 10px; }
	.pair { display: flex; flex-wrap: wrap; gap: 16px; }
	.pair > div { flex: 1 1 620px; }
	img { width: 100%; border-radius: 6px; display: block; }
	figcaption { color: #7f8497; font-size: 13px; margin-top: 6px; }
	a { color: #82aaff; }
</style>
<header>
	<h1>tml screenshots</h1>
	<p>Every picture is the example running in a real terminal -- ttyd serving a PTY, xterm.js in Chromium --
	captured by <a href="https://github.com/wow-look-at-my/tml/blob/master/tools/shots/capture.mjs">tools/shots</a>.</p>
</header>
${figures}
</html>
`;
}

const branch = process.env.SHOTS_BRANCH ?? "";
await mkdir(out, { recursive: true });
const browser = await chromium.launch({ executablePath: browserPath(), args: ["--no-sandbox"] });
try {
	const taken = [];
	for (const shot of shots) {
		taken.push(await capture(browser, shot));
	}
	await writeFile(path.join(out, "index.html"), index(taken, branch));
	console.log(`wrote ${path.relative(repo, path.join(out, "index.html"))}`);
} finally {
	await browser.close();
}
