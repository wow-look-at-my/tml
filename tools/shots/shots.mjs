// The shot list: what to run, how to get it into the state worth looking at,
// and what to call the picture.
//
// A shot is driven two ways. `args` puts the program in a state before the
// terminal is even attached, which is what makes a picture reproducible; `keys`
// are typed into the real terminal afterwards, which is what proves the input
// path works outside a test harness.
//
// `expect` is text the terminal has to be showing before the picture counts. A
// program that died on its arguments still leaves a terminal to screenshot, and
// a black rectangle published as a screenshot is worse than a failed build.
//
// `rows` is the terminal to take the picture in. It is per shot because these
// layouts fill the terminal they are given: a page shot in more rows than it
// has content for is mostly empty space, and one shot in fewer drops whatever
// was pinned to the bottom.
export const shots = [
	{
		name: "gallery-controls",
		expect: "Notify me",
		title: "Controls",
		caption: "Buttons, a field, a checkbox, radios, a progress bar and a sparkline. The keyboard is on Save.",
		command: "build/gallery",
		args: ["-tab", "controls", "-focus", "save"],
		rows: 22,
	},
	{
		name: "gallery-data",
		title: "Data",
		caption: "A list beside a table and a scrolling log, laid out on a Grid.",
		command: "build/gallery",
		args: ["-tab", "data", "-focus", "tab-data"],
		rows: 26,
		expect: "scheduler",
	},
	{
		name: "gallery-media",
		title: "Media",
		caption: "An image on its half-block fallback, a rule, badges and a spinner.",
		command: "build/gallery",
		args: ["-tab", "media", "-focus", "tab-media"],
		rows: 26,
		expect: "Media",
	},
	{
		name: "gallery-popup",
		expect: "Really quit?",
		title: "A popup over the page",
		caption: "A Popup centres itself on the Canvas and the layout behind it keeps rendering.",
		command: "build/gallery",
		args: ["-popup", "-focus", "yes"],
		rows: 22,
	},
	{
		name: "agent-session",
		expect: "asJSON",
		title: "The agent example, mid-session",
		caption: "A transcript scrolled to its tail, a diff card, and the context list beside it.",
		command: "build/agent",
		args: ["-steps", "5"],
	},
	{
		name: "agent-permission",
		expect: "Permission",
		title: "The agent example, asking",
		caption: "The permission beat stops the script until it is answered.",
		command: "build/agent",
		args: ["-steps", "6"],
	},
	{
		name: "agent-typing",
		expect: "make it faster",
		title: "Typing at the prompt",
		caption: "Typed into the running terminal rather than set from a flag: the keys go through ttyd, the PTY and the focus ring.",
		command: "build/agent",
		args: ["-steps", "5", "-focus", "prompt"],
		keys: [..."make it faster"],
	},
];
