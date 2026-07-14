import type { Preview } from "@storybook/react-vite";
import "../src/stories/storybook.css";
import "../src/styles/tokens.css";

const preview: Preview = {
	parameters: {
		layout: "padded",
		backgrounds: { disable: true },
		options: { storySort: { order: ["Foundations", "Components", "Patterns"] } }
	},
	globalTypes: {
		theme: {
			description: "Theme",
			defaultValue: "light",
			toolbar: { icon: "mirror", items: ["light", "dark"], dynamicTitle: true }
		}
	},
	decorators: [
		(Story, context) => {
			document.documentElement.dataset.theme = String(context.globals.theme ?? "light");
			return (
				<div className="storybook-surface">
					<Story />
				</div>
			);
		}
	]
};

export default preview;
