// End-to-end check of the inspection layer against a real program.
//
// It starts build/agent under a pty, asks build/tml the questions the live CLI
// exists to answer, then drives the browser inspector with a pointer and reads
// the result back off the socket. Every assertion is about what the program
// reports after the fact, so a pass means the terminal's own layout changed.
//
// see docs/inspector.md
import { spawn, execFileSync } from 'node:child_process';
import { existsSync, mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { chromium } from 'playwright';

const root = process.env.TML_ROOT ?? process.cwd();

// go-toolchain names a local build build/agent and a cross-compiled one
// build/agent_linux_amd64. CI produces the second, so both are looked for
// rather than assuming which build ran.
const goos = { linux: 'linux', darwin: 'darwin', win32: 'windows' }[process.platform] ?? process.platform;
const goarch = { x64: 'amd64', arm64: 'arm64' }[process.arch] ?? process.arch;

function binary(name) {
	const tries = [join(root, 'build', name), join(root, 'build', `${name}_${goos}_${goarch}`)];
	const found = tries.find(existsSync);
	if (!found) {
		throw new Error(`no ${name} binary: looked for ${tries.join(', ')}. Build them with go-toolchain first.`);
	}
	return found;
}

const agent = binary('agent');
const cli = binary('tml');
const work = mkdtempSync(join(tmpdir(), 'tml-inspect-'));
const sock = join(work, 'agent.sock');

const kids = [];
let browser;

function fail(what, got, want) {
	throw new Error(`${what}: got ${JSON.stringify(got)}, want ${JSON.stringify(want)}`);
}

function tml(...args) {
	return execFileSync(cli, args, {
		env: { ...process.env, TML_INSPECT_SOCKET: sock },
		// The caller reads the message off the thrown error. Letting it reach
		// this process's stderr too would print every retry while the program
		// is still starting.
		stdio: ['ignore', 'pipe', 'pipe'],
	}).toString().trim();
}

// settled polls until a reading stops changing, which is what a drag needs: the
// pointer emits an override per cell it crosses, so the first frame wider than
// the start is a frame from the middle of the gesture.
async function settled(read) {
	let last = await read();
	for (;;) {
		await new Promise(r => setTimeout(r, 250));
		const now = await read();
		if (now === last) return now;
		last = now;
	}
}

// A pty is not optional: bubbletea opens /dev/tty and the layout is sized from
// the window, so without one there is no frame to inspect.
function startAgent() {
	// No -inspect flag: the socket is TML_INSPECT_SOCKET, which Load reads. The
	// agent says nothing about being inspectable, which is the point.
	const p = spawn('script', ['-q', '-c', `stty rows 30 cols 100; exec ${agent}`,
		join(work, 'typescript')],
	{ stdio: 'ignore', detached: true, env: { ...process.env, TML_INSPECT_SOCKET: sock } });
	// A program that dies at startup must say so now. Waiting for it to paint
	// would spend the whole timeout and then report the wrong thing.
	p.on('exit', (code) => { p.died = `the program exited with ${code} before it painted`; });
	kids.push(p);
	return p;
}

// waitFor retries fn, because most of what this checks is a program catching
// up. abort is the exception: it names a state no amount of waiting fixes, and
// it is reported the round it happens rather than at the deadline.
async function waitFor(what, fn, abort = () => null, ms = 15000) {
	const until = Date.now() + ms;
	for (;;) {
		const stop = abort();
		if (stop) throw new Error(`${what}: ${stop}`);
		try {
			const got = await fn();
			if (got) return got;
		} catch (err) {
			if (Date.now() > until) throw new Error(`${what}: ${err.message}`);
		}
		if (Date.now() > until) throw new Error(`timed out waiting for ${what}`);
		await new Promise(r => setTimeout(r, 200));
	}
}

function cleanup() {
	for (const p of kids) {
		try { process.kill(-p.pid, 'SIGTERM'); } catch { /* already gone */ }
	}
	rmSync(work, { recursive: true, force: true });
}

try {
	const program = startAgent();
	await waitFor('the program to paint', () => {
		if (program.died) throw new Error(program.died);
		return tml('ids').length > 0;
	});

	// 1. One element by name, with the geometry and the text it drew.
	const prompt = JSON.parse(tml('query', '--id', 'prompt'));
	if (prompt.element !== 'Textbox') fail('prompt element', prompt.element, 'Textbox');
	if (prompt.rect.w !== 88) fail('prompt width', prompt.rect.w, 88);
	if (!prompt.text.includes('ask for a change')) fail('prompt text', prompt.text, 'the placeholder');

	// A name that is not there names the ones that are, rather than answering
	// with an empty element that reads like an element drawing nothing.
	let named = '';
	try { tml('query', '--id', 'nope'); } catch (err) { named = err.stderr.toString(); }
	if (!named.includes('prompt')) fail('unknown id error', named, 'the list of real ids');

	// 2. The tree carries the boxes with no id, which is where a layout
	// mistake usually is.
	const tree = tml('tree');
	for (const want of ['<Grid>', '<Border>', '#session', '#prompt']) {
		if (!tree.includes(want)) fail('tree', tree, want);
	}

	// A cell resolves to the innermost element covering it.
	if (tml('at', '--x', '25', '--y', '4') !== 'session') {
		fail('hit test at 25,4', tml('at', '--x', '25', '--y', '4'), 'session');
	}

	// 3. A key reaches the program and changes what an element draws.
	const quiet = JSON.parse(tml('query', '--id', 'session')).text;
	tml('input', '--key', 'space');
	const stepped = await waitFor('the script to step',
		() => { const t = JSON.parse(tml('query', '--id', 'session')).text; return t !== quiet && t; });
	if (!stepped.includes('--json')) fail('session after a step', stepped, 'the first scripted turn');

	// 4. A restyle is a real override, and the program repaints for it.
	const was = JSON.parse(tml('query', '--id', 'send')).rect;
	tml('restyle', '--id', 'send', '--set', 'width=20');
	const wide = JSON.parse(tml('query', '--id', 'send')).rect;
	if (wide.w !== 20) fail('width after restyle', wide.w, 20);
	// The sibling reflows, which is the whole point of overriding the document
	// rather than painting over the picture.
	const squeezed = JSON.parse(tml('query', '--id', 'prompt')).rect;
	if (squeezed.w >= prompt.rect.w) fail('the sibling did not reflow', squeezed.w, `< ${prompt.rect.w}`);
	tml('restyle', '--clear');
	if (JSON.parse(tml('query', '--id', 'send')).rect.w !== was.w) {
		fail('width after reset', JSON.parse(tml('query', '--id', 'send')).rect.w, was.w);
	}

	// 5. The browser inspector, driven with a pointer.
	const serve = spawn(cli, ['serve', '--addr', '127.0.0.1:0'],
		{ env: { ...process.env, TML_INSPECT_SOCKET: sock }, detached: true });
	kids.push(serve);
	const url = await new Promise((resolve, reject) => {
		let seen = '';
		serve.stdout.on('data', (b) => {
			seen += b.toString();
			const hit = seen.match(/http:\/\/[\d.]+:\d+/);
			if (hit) resolve(hit[0]);
		});
		serve.on('exit', (code) => reject(new Error(`serve exited with ${code}: ${seen}`)));
	});

	// CI installs the browser this playwright expects. A machine that already
	// has one names it in TML_CHROMIUM rather than downloading a second copy.
	browser = await chromium.launch({
		args: ['--no-sandbox'],
		...(process.env.TML_CHROMIUM ? { executablePath: process.env.TML_CHROMIUM } : {}),
	});
	const page = await browser.newPage({ viewport: { width: 1500, height: 950 } });
	page.on('pageerror', (err) => { throw err; });
	// Not networkidle: the frame stream holds a request open for the page's life.
	await page.goto(url, { waitUntil: 'domcontentloaded' });
	await page.waitForFunction(() => document.querySelector('#preview')?.textContent.includes('tml agent'),
		{ timeout: 20000 });

	const rpc = (body) => page.evaluate(async (b) => {
		const res = await fetch('rpc', { method: 'POST', body: JSON.stringify(b) });
		return res.json();
	}, body);

	// The page's own cell-to-pixel map, so the pointer lands where a person's
	// would rather than where this script guesses.
	const cell = (col, row) => page.evaluate(([c, r]) => {
		const pre = document.querySelector('#preview');
		const box = pre.getBoundingClientRect();
		const probe = document.createElement('span');
		probe.textContent = 'X'.repeat(100);
		probe.style.cssText = 'position:absolute;visibility:hidden;white-space:pre;font:inherit';
		pre.appendChild(probe);
		const w = probe.getBoundingClientRect().width / 100;
		probe.remove();
		const style = getComputedStyle(pre);
		const pad = parseFloat(style.paddingLeft);
		return { x: box.left + pad + (c + 0.5) * w, y: box.top + pad + (r + 0.5) * parseFloat(style.lineHeight) };
	}, [col, row]);

	const start = (await rpc({ op: 'query', id: 'send' })).element.rect;
	const grab = await cell(start.x + 1, start.y + 1);

	await page.mouse.click(grab.x, grab.y);
	await page.waitForFunction(() => document.querySelector('#selected-id')?.textContent.trim() === 'send',
		{ timeout: 5000 });

	await page.keyboard.down('Shift');
	await page.mouse.move(grab.x, grab.y);
	await page.mouse.down();
	const to = await cell(start.x + 13, start.y + 4);
	await page.mouse.move(to.x, to.y, { steps: 8 });
	await page.mouse.up();
	await page.keyboard.up('Shift');

	const width = await settled(async () => (await rpc({ op: 'query', id: 'send' })).element.rect.w);
	if (width <= start.w) fail('width after the drag', width, `> ${start.w}`);
	// The socket is a second opinion: the CLI sees the same geometry, which is
	// what proves the drag changed the program and not only the page.
	const overCli = JSON.parse(tml('query', '--id', 'send')).rect.w;
	if (overCli !== width) fail('the CLI and the page disagree', overCli, width);

	await rpc({ op: 'reset' });
	await waitFor('reset to restore the document',
		async () => (await rpc({ op: 'query', id: 'send' })).element.rect.w === start.w);

	console.log(`ok: queried by name, walked the tree, drove a key, restyled, ` +
		`and dragged ${start.w} columns to ${width} from the browser`);
} finally {
	if (browser) await browser.close();
	cleanup();
}
