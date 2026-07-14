import type { Meta, StoryObj } from "@storybook/react-vite";

function Foundations() {
	const roles = ["background", "surface", "surface-subtle", "primary", "success", "warning", "danger", "info"];
	return (
		<div className="storybook-stack">
			<h1>Semantic foundations</h1>
			<p>Components consume roles, so light and dark themes retain the same hierarchy.</p>
			<div className="storybook-swatches">
				{roles.map((role) => (
					<div className="storybook-swatch" key={role} style={{ background: `var(--pt-${role})` }}>
						{role}
					</div>
				))}
			</div>
		</div>
	);
}

const meta = { title: "Foundations/Tokens", component: Foundations } satisfies Meta<typeof Foundations>;
export default meta;
export const SemanticTokens: StoryObj<typeof meta> = {};
