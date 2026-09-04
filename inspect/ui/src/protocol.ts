// The wire this page speaks, as the Go side defines it: inspect/protocol.go
// and inspect/element.go carry the same names, and inspect/capture.go carries
// Capture. These are declarations of what arrives, never a second model of it.

export interface Rect {
	x: number;
	y: number;
	w: number;
	h: number;
}

export interface Size {
	w: number;
	h: number;
}

export interface Scroll {
	x: number;
	y: number;
	maxX: number;
	maxY: number;
}

export interface Element {
	id: string;
	element: string;
	action?: string;
	focus: boolean;
	hover: boolean;
	held: boolean;
	rect: Rect;
	content: Size;
	clip: Rect;
	scroll: Scroll;
	text: string;
	lines: string[];
	ansi?: string;
}

export interface Node {
	element: string;
	id?: string;
	action?: string;
	focus?: boolean;
	rect: Rect;
	text?: string;
	children?: Node[];
}

export interface FrameInfo {
	seq: number;
	at: string;
	width: number;
	height: number;
	text: string;
	ansi?: string;
}

export interface Request {
	op: string;
	id?: string;
	x?: number;
	y?: number;
	key?: string;
	attrs?: Record<string, string>;
	ansi?: boolean;
	since?: number;
}

export interface Response {
	error?: string;
	ok?: boolean;
	element?: Element;
	elements?: Element[];
	tree?: Node;
	frame?: FrameInfo;
	ids?: string[];
	hits?: number[][];
	hit: string;
	found?: boolean;
}

// Capture is what `tml capture` writes into the document. hits is At's answer
// per cell, indexing elements, so this page never works out for itself what
// covers a cell.
export interface Capture {
	frame: FrameInfo;
	elements: Element[];
	tree: Node;
	hits: number[][];
}
