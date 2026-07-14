import type { Meta, StoryObj } from "@storybook/react-vite";
import { Archive, Plus } from "lucide-react";
import { useState } from "react";
import { Badge, Button, ConfirmDialog, EmptyState, IconButton, Panel, SelectField, Spinner, TextAreaField, TextField } from "../index";

function ComponentCatalog() {
	const [confirmOpen, setConfirmOpen] = useState(false);
	return (
		<div className="storybook-stack">
			<Panel title="Buttons and states" description="Commands use explicit tone and stable dimensions.">
				<div className="storybook-row">
					<Button leadingIcon={<Plus size="1rem" aria-hidden="true" />}>Create task</Button>
					<Button variant="secondary">Secondary</Button>
					<Button variant="ghost">Ghost</Button>
					<Button variant="danger" onClick={() => setConfirmOpen(true)}>
						Delete
					</Button>
					<Button loading>Saving</Button>
					<IconButton label="Archive" icon={<Archive size="1rem" aria-hidden="true" />} />
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
				<Badge>Neutral</Badge>
				<Badge tone="info">In progress</Badge>
				<Badge tone="success">Done</Badge>
				<Badge tone="warning">Attention</Badge>
				<Badge tone="danger">Blocked</Badge>
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
