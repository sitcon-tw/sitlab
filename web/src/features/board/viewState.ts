import type { BoardSortMode, Bootstrap } from "./model";

export interface BoardViewState {
	teamKey: string;
	memberIds: number[];
	labels: string[];
	sortMode: BoardSortMode;
}

const sortModes = new Set<BoardSortMode>(["manual", "due-asc", "due-desc", "start-asc", "start-desc", "updated-desc", "updated-asc"]);
const ownedParameters = ["team", "member", "label", "sort"] as const;

export function parseBoardViewState(search: string, bootstrap: Bootstrap): BoardViewState {
	const parameters = new URLSearchParams(search);
	const validTeams = new Set(bootstrap.teams.filter((team) => team.active).map((team) => team.key));
	const validMembers = new Set(bootstrap.members.filter((member) => member.state === "active").map((member) => member.gitLabUserId));
	const requestedTeam = parameters.get("team") ?? "";
	const requestedSort = parameters.get("sort") as BoardSortMode | null;

	return {
		teamKey: validTeams.has(requestedTeam) ? requestedTeam : "",
		memberIds: unique(
			parameters
				.getAll("member")
				.map((value) => Number(value))
				.filter((value) => Number.isSafeInteger(value) && validMembers.has(value))
		),
		labels: unique(
			parameters
				.getAll("label")
				.map((value) => value.trim())
				.filter(Boolean)
		),
		sortMode: requestedSort && sortModes.has(requestedSort) ? requestedSort : "manual"
	};
}

export function serializeBoardViewState(currentSearch: string, state: BoardViewState) {
	const parameters = new URLSearchParams(currentSearch);
	for (const parameter of ownedParameters) parameters.delete(parameter);
	if (state.teamKey) parameters.set("team", state.teamKey);
	for (const memberId of unique(state.memberIds)) parameters.append("member", String(memberId));
	for (const label of unique(state.labels.map((value) => value.trim()).filter(Boolean))) parameters.append("label", label);
	if (state.sortMode !== "manual") parameters.set("sort", state.sortMode);
	const query = parameters.toString();
	return query ? `?${query}` : "";
}

function unique<Value>(values: Value[]) {
	return [...new Set(values)];
}
