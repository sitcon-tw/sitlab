import type { Meta, StoryObj } from "@storybook/react-vite";
import type { CSSProperties } from "react";

const colorPairs = [
	["primary", "on-primary"],
	["primary-container", "on-primary-container"],
	["secondary", "on-secondary"],
	["secondary-container", "on-secondary-container"],
	["tertiary", "on-tertiary"],
	["tertiary-container", "on-tertiary-container"],
	["error", "on-error"],
	["error-container", "on-error-container"],
	["warning", "on-warning"],
	["warning-container", "on-warning-container"],
	["surface", "on-surface"],
	["surface-variant", "on-surface-variant"],
	["inverse-surface", "inverse-on-surface"]
];

const containers = ["surface-container-lowest", "surface-container-low", "surface-container", "surface-container-high", "surface-container-highest"];

const shapes = ["none", "extra-small", "small", "medium", "large", "extra-large", "full"];

const typescale = [
	"display-large",
	"display-medium",
	"display-small",
	"headline-large",
	"headline-medium",
	"headline-small",
	"title-large",
	"title-medium",
	"title-small",
	"body-large",
	"body-medium",
	"body-small",
	"label-large",
	"label-medium",
	"label-small"
];

function Foundations() {
	return (
		<div className="storybook-stack">
			<h1>Material Design 3 foundations</h1>
			<p>
				Tokens come in three layers: <code>--md-ref-palette-*</code> tonal ramps generated from the brand seed, <code>--md-sys-*</code> system tokens that all
				product CSS consumes, and a few product roles MD3 has no equivalent for. Light and dark are separate tonal mappings, not inversions — use the theme
				toolbar to compare.
			</p>

			<h2>Color roles</h2>
			<div className="storybook-swatches">
				{colorPairs.map(([role, on]) => (
					<div
						className="storybook-swatch"
						key={role}
						style={{ background: `var(--md-sys-color-${role})`, color: `var(--md-sys-color-${on})`, borderColor: "var(--md-sys-color-outline-variant)" }}
					>
						{role}
					</div>
				))}
			</div>

			<h2>Surface containers</h2>
			<p>The emphasis ladder. Light reads lowest as the lightest; dark reads it as the darkest.</p>
			<div className="storybook-swatches">
				{containers.map((role) => (
					<div
						className="storybook-swatch"
						key={role}
						style={{ background: `var(--md-sys-color-${role})`, color: "var(--md-sys-color-on-surface)", borderColor: "var(--md-sys-color-outline-variant)" }}
					>
						{role.replace("surface-container", "container")}
					</div>
				))}
			</div>

			<h2>Elevation</h2>
			<div className="storybook-row">
				{[0, 1, 2, 3, 4, 5].map((level) => (
					<div
						key={level}
						style={{
							display: "grid",
							placeItems: "center",
							width: "5rem",
							height: "4rem",
							background: "var(--md-sys-color-surface-container-low)",
							borderRadius: "var(--md-sys-shape-corner-medium)",
							boxShadow: `var(--md-sys-elevation-level${level})`
						}}
					>
						level{level}
					</div>
				))}
			</div>

			<h2>Shape scale</h2>
			<div className="storybook-row">
				{shapes.map((shape) => (
					<div
						key={shape}
						style={{
							display: "grid",
							placeItems: "center",
							width: "6rem",
							height: "3.5rem",
							fontSize: "0.6875rem",
							color: "var(--md-sys-color-on-secondary-container)",
							background: "var(--md-sys-color-secondary-container)",
							borderRadius: `var(--md-sys-shape-corner-${shape})`
						}}
					>
						{shape}
					</div>
				))}
			</div>

			<h2>Type scale</h2>
			<div className="storybook-stack">
				{typescale.map((role) => (
					<p key={role} className={`md-typescale-${role}`}>
						{role}
					</p>
				))}
			</div>

			<h2>State layers</h2>
			<p>Each layer is the surface&apos;s own content color at a fixed opacity, so no variant needs its own token.</p>
			<div className="storybook-row">
				{[
					["hover", "0.08"],
					["focus", "0.10"],
					["pressed", "0.10"],
					["dragged", "0.16"]
				].map(([state, opacity]) => (
					<div
						key={state}
						style={
							{
								position: "relative",
								display: "grid",
								placeItems: "center",
								width: "6rem",
								height: "3rem",
								overflow: "hidden",
								color: "var(--md-sys-color-on-primary)",
								background: "var(--md-sys-color-primary)",
								borderRadius: "var(--md-sys-shape-corner-full)",
								isolation: "isolate"
							} as CSSProperties
						}
					>
						<span
							aria-hidden="true"
							style={{ position: "absolute", inset: 0, zIndex: -1, background: "currentColor", opacity: Number(opacity), borderRadius: "inherit" }}
						/>
						{state} {opacity}
					</div>
				))}
			</div>
		</div>
	);
}

const meta = { title: "Foundations/Tokens", component: Foundations } satisfies Meta<typeof Foundations>;
export default meta;
export const MaterialDesign3Tokens: StoryObj<typeof meta> = {};
