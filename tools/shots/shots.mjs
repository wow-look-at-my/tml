// The shot list: what to run, how to get it into the state worth looking at,
// and what to call the picture.
//
// A shot is driven two ways. `args` puts the program in a state before the
// terminal is even attached, which is what makes a picture reproducible; `keys`
// are typed into the real terminal afterwards, which is what proves the input
// path works outside a test harness.
export const shots = [
	{
		name: "gallery-controls",
		title: "Controls",
		caption: "Buttons, a field, a checkbox, radios, a progress bar and a sparkline. The keyboard is on Save.",
		command: "build/gallery",
		args: ["-tab", "controls", "-focus", "save"],
	},
	{
		name: "gallery-data",
		title: "Data",
		caption: "A list beside a table and a scrolling log, laid out on a Grid.",
		command: "build/gallery",
		args: ["-tab", "data"],
	},
	{
		name: "gallery-media",
		title: "Media",
		caption: "An image on its half-block fallback, a rule, badges and a spinner.",
		command: "build/gallery",
		args: ["-tab", "media"],
	},
	{
		name: "gallery-popup",
		title: "A popup over the page",
		caption: "A Popup centres itself on the Canvas and the layout behind it keeps rendering.",
		command: "build/gallery",
		args: ["-popup", "-focus", "yes"],
	},
	{
		name: "agent-session",
		title: "The agent example, mid-session",
		caption: "A transcript scrolled to its tail, a diff card, and the context list beside it.",
		command: "build/agent",
		args: ["-steps", "5"],
	},
	{
		name: "agent-permission",
		title: "The agent example, asking",
		caption: "The permission beat stops the script until it is answered.",
		command: "build/agent",
		args: ["-steps", "6"],
	},
	{
		name: "agent-typing",
		title: "Typing at the prompt",
		caption: "Typed into the running terminal rather than set from a flag: the keys go through ttyd, the PTY and the focus ring.",
		command: "build/agent",
		args: ["-steps", "5", "-focus", "prompt"],
		keys: [..."make it faster"],
	},
];
