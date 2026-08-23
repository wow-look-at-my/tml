// One-shot unix socket that answers a single inspect JSON-line request.
//
// The CLI dials, writes one Request, and reads one Response. This is that
// other end: bind, accept, discard the request, write argv's canned JSON.
import { createServer } from 'node:net';
import { readFileSync, unlinkSync } from 'node:fs';

const [path, responsePath] = process.argv.slice(2);

let body = readFileSync(responsePath, 'utf8');
if (!body.endsWith('\n')) {
	body += '\n';
}

try {
	unlinkSync(path);
} catch (err) {
	// A leftover socket from a previous run is the only expected case; any
	// other error means the path is unusable and bind will say so.
	if (err.code !== 'ENOENT') {
		throw err;
	}
}

// The request is read to completion before answering so the client sees a
// well-formed exchange rather than a write into a half-read stream.
const server = createServer((conn) => {
	let buf = '';
	conn.on('data', (chunk) => {
		buf += chunk;
		if (buf.includes('\n')) {
			conn.end(body);
		}
	});
	conn.on('end', () => conn.end(body));
});

// Matches the 5s accept timeout the suite's callers rely on: a client that
// never dials must not wedge the test.
const timer = setTimeout(() => {
	server.close();
	process.exit(1);
}, 5000);

server.on('connection', () => clearTimeout(timer));
server.listen(path);
