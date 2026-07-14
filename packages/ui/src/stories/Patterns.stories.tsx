import type { Meta, StoryObj } from "@storybook/react-vite";
import { CheckSquare } from "lucide-react";
import { Badge, Button, DataTable, EmptyState, PageShell, Panel } from "../index";

const rows = [
	{ id: "1", title: "Review access policy", status: "In progress", owner: "Taylor" },
	{ id: "2", title: "Publish release notes", status: "Done", owner: "Morgan" }
];

function TaskPattern() {
	return (
		<PageShell title="Tasks" description="Track work for this workspace." actions={<Button>Create task</Button>}>
			<Panel>
				<DataTable
					label="Workspace tasks"
					rows={rows}
					getRowKey={(row) => row.id}
					columns={[
						{ key: "title", header: "Task", cell: (row) => row.title },
						{ key: "status", header: "Status", cell: (row) => <Badge tone={row.status === "Done" ? "success" : "info"}>{row.status}</Badge> },
						{ key: "owner", header: "Assignee", cell: (row) => row.owner }
					]}
					empty={<EmptyState title="No tasks" description="Create the first task in this workspace." icon={<CheckSquare size="2rem" />} />}
				/>
			</Panel>
		</PageShell>
	);
}

const meta = { title: "Patterns/Task list", component: TaskPattern, parameters: { layout: "fullscreen" } } satisfies Meta<typeof TaskPattern>;
export default meta;
export const DenseWorkspace: StoryObj<typeof meta> = {};
