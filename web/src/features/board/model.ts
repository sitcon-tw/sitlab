import type { components, operations } from "@/shared/api/openapi";

export type Bootstrap = components["schemas"]["BootstrapResponse"];
export type BoardCard = components["schemas"]["BoardCard"];
export type BoardList = components["schemas"]["BoardList"];
export type DirectoryTeam = components["schemas"]["DirectoryTeam"];
export type DirectoryMember = components["schemas"]["DirectoryMember"];
export type CurrentUser = components["schemas"]["CurrentUser"];
export type UserPreferences = components["schemas"]["UserPreferences"];
export type CardMutation = components["schemas"]["CardMutationResponse"];
export type ProjectLabel = components["schemas"]["ProjectLabel"];
export type CardComment = components["schemas"]["CardComment"];
export type WorkItemSummary = components["schemas"]["WorkItemSummary"];
export type LinkedWorkItem = components["schemas"]["LinkedWorkItem"];
export type WorkItemLinkType = LinkedWorkItem["linkType"];
export type WorkItemRelationshipKind = operations["searchRelationshipCandidates"]["parameters"]["query"]["kind"];

export type BoardSortMode = "manual" | "due-asc" | "due-desc" | "start-asc" | "start-desc" | "updated-desc" | "updated-asc";

export function compareBoardCards(a: BoardCard, b: BoardCard, sortMode: BoardSortMode) {
	let result = 0;
	switch (sortMode) {
		case "due-asc":
			result = compareOptionalDates(a.dueDate, b.dueDate, 1);
			break;
		case "due-desc":
			result = compareOptionalDates(a.dueDate, b.dueDate, -1);
			break;
		case "start-asc":
			result = compareOptionalDates(a.startDate, b.startDate, 1);
			break;
		case "start-desc":
			result = compareOptionalDates(a.startDate, b.startDate, -1);
			break;
		case "updated-desc":
			result = b.updatedAt.localeCompare(a.updatedAt);
			break;
		case "updated-asc":
			result = a.updatedAt.localeCompare(b.updatedAt);
			break;
	}
	return result || a.position - b.position || a.issueIid - b.issueIid;
}

function compareOptionalDates(a: string | null, b: string | null, direction: 1 | -1) {
	if (a === null && b === null) return 0;
	if (a === null) return 1;
	if (b === null) return -1;
	return a.localeCompare(b) * direction;
}

export function taipeiDateAfter(days: number, now = new Date()) {
	const target = new Date(now.getTime() + days * 24 * 60 * 60 * 1000);
	const parts = new Intl.DateTimeFormat("en-CA", {
		timeZone: "Asia/Taipei",
		year: "numeric",
		month: "2-digit",
		day: "2-digit"
	}).formatToParts(target);
	const value = Object.fromEntries(parts.map((part) => [part.type, part.value]));
	return `${value.year}-${value.month}-${value.day}`;
}

export function memberById(bootstrap: Bootstrap, gitLabUserId: number | null) {
	if (gitLabUserId === null) return undefined;
	return bootstrap.members.find((member) => member.gitLabUserId === gitLabUserId);
}

export function activeMembers(bootstrap: Bootstrap) {
	return bootstrap.members.filter((member) => member.state === "active");
}

export function filterDirectoryMembers(members: DirectoryMember[], query: string) {
	const normalized = query.trim().toLocaleLowerCase("zh-Hant");
	return members.filter((member) => !normalized || `${member.displayName} ${member.username}`.toLocaleLowerCase("zh-Hant").includes(normalized));
}

export function teamMembers(bootstrap: Bootstrap, teamKey: string) {
	return activeMembers(bootstrap).filter((member) => member.teamKeys.includes(teamKey));
}

export function preferredAssignees(bootstrap: Bootstrap, teamKey: string) {
	return bootstrap.preferences.defaultTeamKey === teamKey ? [bootstrap.me.gitLabUserId] : [];
}

export function teamLeaders(bootstrap: Bootstrap, teamKey: string) {
	const team = bootstrap.teams.find((item) => item.key === teamKey && item.active);
	if (!team) return [];
	return bootstrap.members.filter((member) => member.state === "active" && team.leaderGitLabUserIds.includes(member.gitLabUserId));
}
