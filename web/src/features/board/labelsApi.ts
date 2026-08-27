import { api, expectData, getCsrfToken } from "@/shared/api/client";
import type { ProjectLabel } from "./model";

export interface ProjectLabelWrite {
	name: string;
	color: string;
	description: string | null;
}

const demo = import.meta.env.VITE_SITCON_DEMO === "true";

/**
 * Demo mode keeps a mutable catalog so creating, renaming, and deleting a label
 * visibly work without a server. State resets on reload, which is expected.
 */
const demoLabels: ProjectLabel[] = [
	{ id: 1, name: "Team::開發組", color: "#0E8A16", textColor: "#FFFFFF", description: "開發組" },
	{ id: 2, name: "Team::設計組", color: "#B60205", textColor: "#FFFFFF", description: "設計組" },
	{ id: 3, name: "Priority::High", color: "#D73A4A", textColor: "#FFFFFF", description: "優先處理" },
	{ id: 4, name: "Backend", color: "#1D76DB", textColor: "#FFFFFF", description: null }
];
let nextDemoId = 100;

async function demoDelay() {
	await new Promise((resolve) => setTimeout(resolve, 200));
}

export async function listProjectLabels(): Promise<ProjectLabel[]> {
	if (demo) return [...demoLabels];
	return expectData(await api.GET("/labels")).labels;
}

export async function createProjectLabel(input: ProjectLabelWrite): Promise<ProjectLabel> {
	if (demo) {
		await demoDelay();
		if (demoLabels.some((label) => label.name === input.name)) throw new Error("已經有同名的 Label。");
		const created: ProjectLabel = { id: nextDemoId++, name: input.name, color: input.color, textColor: "#FFFFFF", description: input.description };
		demoLabels.push(created);
		return created;
	}
	return expectData(
		await api.POST("/labels", {
			params: { header: { "X-CSRF-Token": await getCsrfToken() } },
			body: input
		})
	);
}

export async function updateProjectLabel(labelId: number, input: ProjectLabelWrite): Promise<ProjectLabel> {
	if (demo) {
		await demoDelay();
		const index = demoLabels.findIndex((label) => label.id === labelId);
		if (index < 0) throw new Error("找不到這個 Label。");
		const updated: ProjectLabel = { ...demoLabels[index]!, name: input.name, color: input.color, description: input.description };
		demoLabels[index] = updated;
		return updated;
	}
	return expectData(
		await api.PUT("/labels/{labelId}", {
			params: { path: { labelId }, header: { "X-CSRF-Token": await getCsrfToken() } },
			body: input
		})
	);
}

export async function deleteProjectLabel(labelId: number): Promise<void> {
	if (demo) {
		await demoDelay();
		const index = demoLabels.findIndex((label) => label.id === labelId);
		if (index >= 0) demoLabels.splice(index, 1);
		return;
	}
	const response = await api.DELETE("/labels/{labelId}", {
		params: { path: { labelId }, header: { "X-CSRF-Token": await getCsrfToken() } }
	});
	if (response.error) throw response.error;
}
