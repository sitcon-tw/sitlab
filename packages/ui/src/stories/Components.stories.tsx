import type { Meta, StoryObj } from "@storybook/react-vite";
import { Archive, Plus } from "lucide-react";
import { useState } from "react";
import {
	Badge,
	Button,
	ConfirmDialog,
	EmptyState,
	IconButton,
	LinearProgress,
	Panel,
	SegmentedButton,
	SelectField,
	Spinner,
	StaticChip,
	TextAreaField,
	TextField
} from "../index";

function ComponentCatalog() {
	const [confirmOpen, setConfirmOpen] = useState(false);
	const [mode, setMode] = useState<"single" | "all">("single");
	return (
		<div className="storybook-stack">
			<Panel title="Buttons and states" description="Commands use explicit tone and stable dimensions.">
				<div className="storybook-row">
					<Button leadingIcon={<Plus size="1rem" aria-hidden="true" />}>Create task</Button>
					<Button variant="outlined">Outlined</Button>
					<Button variant="text">Text</Button>
					<Button variant="tonal">Tonal</Button>
					<Button variant="elevated">Elevated</Button>
					<Button variant="filled" tone="error" onClick={() => setConfirmOpen(true)}>
						Delete
					</Button>
					<Button loading>Saving</Button>
					<IconButton label="Archive" icon={<Archive size="1rem" aria-hidden="true" />} />
					<IconButton label="Add" variant="filled" icon={<Plus size="1rem" aria-hidden="true" />} />
					<IconButton label="Pin" variant="tonal" icon={<Archive size="1rem" aria-hidden="true" />} selected />
					<IconButton label="More" variant="outlined" icon={<Plus size="1rem" aria-hidden="true" />} />
				</div>
				<div className="storybook-row">
					<SegmentedButton
						label="Create mode"
						value={mode}
						onChange={setMode}
						options={[
							{ value: "single", label: "Single team" },
							{ value: "all", label: "All leaders" }
						]}
					/>
					<LinearProgress label="Syncing" />
				</div>
			</Panel>
			<Panel title="Fields">
				<div className="storybook-stack">
					<TextField label="Title" placeholder="Prepare quarterly review" />
					<SelectField
						label="Status"
						defaultValue="todo"
						options={[
							{ value: "todo", label: "To do" },
							{ value: "done", label: "Done" }
						]}
					/>
					<TextAreaField label="Description" optional />
					<TextField label="Invalid field" error="A title is required." />
				</div>
			</Panel>
			<div className="storybook-row">
				<StaticChip label="Neutral" />
				<StaticChip label="In progress" variant="suggestion" />
				<StaticChip label="Done" variant="filter" selected />
				<StaticChip label="Attention" variant="assist" elevated />
				<Badge>3</Badge>
				<Badge tone="primary">12</Badge>
				<Badge dot aria-label="Unread" />
				<Spinner />
			</div>
			<Panel>
				<EmptyState title="No archived tasks" description="Completed work will appear here." icon={<Archive size="2rem" />} />
			</Panel>
			<ConfirmDialog
				open={confirmOpen}
				onOpenChange={setConfirmOpen}
				title="Delete task?"
				description="This action cannot be undone."
				confirmLabel="Delete task"
				destructive
				onConfirm={() => setConfirmOpen(false)}
			/>
		</div>
	);
}

const meta = { title: "Components/Catalog", component: ComponentCatalog } satisfies Meta<typeof ComponentCatalog>;
export default meta;
export const AllStates: StoryObj<typeof meta> = {};
