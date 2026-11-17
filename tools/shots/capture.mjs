// capture runs each shot in Ghostty, on a virtual display, and writes a PNG of the window.
import { execFile, spawn } from "node:child_process";
import { existsSync } from "node:fs";
import { mkdir, mkdtemp, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import process from "node:process";
import { promisify } from "node:util";

import { shots } from "./shots.mjs";

const run = promisify(execFile);

// The terminal every shot is taken in, unless it asks for its own.
const terminal = { cols: 96, rows: 30 };
const repo = path.resolve(import.meta.dirname, "../..");
const out = path.join(repo, "build/shots");

// The site the pictures are published to.
const site = "https://sites.pazer.build/tml";

// The X class of a Ghostty window, which is how the capture finds it.
const windowClass = "com.mitchellh.ghostty";

// need fails by name when a tool the capture runs is not on the machine.
async function need(tools) {
	const missing = [];
	for (const tool of tools) {
		await run("which", [tool]).catch(() => missing.push(tool));
	}
	if (missing.length > 0) {
		throw new Error(`not installed: ${missing.join(", ")}. docs/screenshots.md says what the capture needs.`);
	}
}

const goos = { linux: "linux", darwin: "darwin", win32: "windows" }[process.platform] ?? process.platform;
const goarch = { x64: "amd64", arm64: "arm64" }[process.arch] ?? process.arch;

function binary(name) {
	const tries = [path.join(repo, "build", name), path.join(repo, "build", `${name}_${goos}_${goarch}`)];
	const found = tries.find((file) => existsSync(file));
	if (!found) {
		throw new Error(`no ${name} binary: looked for ${tries.join(", ")}. Build them with go-toolchain first.`);
	}
	return found;
}

// display starts a virtual X display. The server picks a free number and writes it to the descriptor it was given.
async function display() {
	const server = spawn("Xvfb", ["-displayfd", "3", "-screen", "0", "2400x1600x24", "-nolisten", "tcp"], {
		stdio: ["ignore", "ignore", "ignore", "pipe"],
	});
	const number = await new Promise((resolve, reject) => {
		server.stdio[3].once("data", (chunk) => resolve(String(chunk).trim()));
		server.once("error", reject);
		server.once("exit", (code) => reject(new Error(`Xvfb exited with ${code} before it named a display`)));
	});
	return { name: `:${number}`, stop: () => server.kill("SIGTERM") };
}

// open starts Ghostty on the shot's command, in a window of exactly cols x rows. Ghostty reads no file of the
// machine's, so the picture depends on these options alone. The program's inspection socket is at a path the capture
// chose, which is how it reads what the program painted.
function open(shot, env, socket) {
	const ghostty = spawn("ghostty", [
		"--config-default-files=false",
		"--gtk-single-instance=false",
		"--window-decoration=none",
		`--window-width=${shot.cols ?? terminal.cols}`,
		`--window-height=${shot.rows ?? terminal.rows}`,
		"--window-padding-x=0",
		"--window-padding-y=0",
		"--font-size=11",
		"--background=#12131a",
		"--confirm-close-surface=false",
		"--resize-overlay=never",
		"--cursor-style-blink=false",
		"-e", binary(shot.command), ...(shot.args ?? []),
	], { cwd: repo, env: { ...env, TML_INSPECT_SOCKET: socket }, stdio: ["ignore", "ignore", "pipe"] });

	let log = "";
	ghostty.stderr.on("data", (chunk) => { log += chunk; });
	const exited = new Promise((resolve) => ghostty.once("exit", resolve));
	return {
		log: () => log.trim().split("\n").slice(-12).join("\n"),
		stop: async () => {
			ghostty.kill("SIGTERM");
			await exited;
		},
	};
}

// frame is what the program has painted, as its own inspector reports it. Null is a program that has not painted.
async function frame(socket) {
	try {
		const { stdout } = await run(binary("tml"), ["frame", "--socket", socket]);
		return JSON.parse(stdout);
	} catch {
		return null;
	}
}

// type sends the shot's keys to the focused window. One character is typed. A longer name is an X keysym, like Return.
async function type(env, keys) {
	for (const key of keys) {
		const args = [...key].length === 1 ? ["type", "--delay", "0", key] : ["key", key];
		await run("xdotool", args, { env });
		await sleep(60);
	}
}

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

async function waitFor(check, what, timeout = 15000) {
	const deadline = Date.now() + timeout;
	while (Date.now() < deadline) {
		if (await check()) {
			return;
		}
		await sleep(100);
	}
	throw new Error(`timed out waiting for ${what}`);
}

// photograph writes the window to file and reports how many colours the picture holds.
async function photograph(env, window, file) {
	await run("import", ["-window", window, file], { env });
	const { stdout } = await run("identify", ["-format", "%k", file], { env });
	return Number(stdout);
}

// capture takes one picture. It waits on what the program painted, read through the program's own inspector. A
// program that died on its arguments answers nothing, and the shot fails rather than photograph an empty terminal.
async function capture(env, work, shot) {
	if (!shot.expect) {
		throw new Error(`shot ${shot.name} has no expect: there would be nothing to check the picture against`);
	}
	const cols = shot.cols ?? terminal.cols;
	const rows = shot.rows ?? terminal.rows;
	const socket = path.join(work, `${shot.name}.sock`);
	const ghostty = open(shot, env, socket);
	try {
		// These layouts fill their terminal, so a window of another size is another picture.
		let painted = null;
		try {
			await waitFor(async () => {
				painted = await frame(socket);
				return painted?.width === cols && painted?.height === rows;
			}, `${shot.name} to draw`);
		} catch {
			const got = painted ? `the terminal came out ${painted.width}x${painted.height}` : "the program painted nothing";
			throw new Error(`${shot.name} wanted ${cols}x${rows} and ${got}. Ghostty said:\n${ghostty.log()}`);
		}

		const { stdout } = await run("xdotool", ["search", "--onlyvisible", "--class", windowClass], { env });
		const window = stdout.trim().split("\n").pop();
		// No window manager runs here, so nothing gives the window the keyboard unless the capture does.
		await run("xdotool", ["mousemove", "2390", "1590"], { env });
		await run("xdotool", ["windowfocus", window], { env });
		await type(env, shot.keys ?? []);

		try {
			await waitFor(async () => (await frame(socket))?.text.includes(shot.expect), shot.expect, 5000);
		} catch {
			const held = (await frame(socket))?.text ?? "";
			throw new Error(`${shot.name} never showed ${JSON.stringify(shot.expect)}; the program had painted:\n${held}`);
		}
		// The program has painted it. Ghostty draws what it read on its next frame.
		await sleep(400);

		const file = path.join(out, `${shot.name}.png`);
		await waitFor(async () => await photograph(env, window, file) > 8, `${shot.name} to appear in the terminal`);
		console.log(`wrote ${path.relative(repo, file)}`);
		return { ...shot, file: `${shot.name}.png` };
	} finally {
		await ghostty.stop();
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
	<p>Every picture is the example running in Ghostty, on a virtual display,
	captured by <a href="https://github.com/wow-look-at-my/tml/blob/master/tools/shots/capture.mjs">tools/shots</a>.</p>
</header>
${figures}
</html>
`;
}

const branch = process.env.SHOTS_BRANCH ?? "";
await need(["ghostty", "Xvfb", "xdotool", "import", "identify"]);
await mkdir(out, { recursive: true });
const work = await mkdtemp(path.join(os.tmpdir(), "tml-shots-"));
const screen = await display();
// Ghostty draws with OpenGL, which Mesa answers in software where there is no GPU. A scale of draws every cell at
// twice its size, which is the sharpness a picture on a dense screen needs.
const env = {
	...process.env,
	DISPLAY: screen.name,
	GDK_BACKEND: "x11",
	GDK_SCALE: "2",
	GTK_A11Y: "none",
	LIBGL_ALWAYS_SOFTWARE: "1",
	XDG_CONFIG_HOME: path.join(work, "config"),
	XDG_RUNTIME_DIR: work,
};
try {
	const taken = [];
	for (const shot of shots) {
		taken.push(await capture(env, work, shot));
	}
	await writeFile(path.join(out, "index.html"), index(taken, branch));
	console.log(`wrote ${path.relative(repo, path.join(out, "index.html"))}`);
} finally {
	screen.stop();
	await rm(work, { recursive: true, force: true });
}
