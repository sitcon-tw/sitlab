import type { components } from "@/shared/api/openapi";

export type Bootstrap = components["schemas"]["BootstrapResponse"];
export type BoardCard = components["schemas"]["BoardCard"];
export type BoardList = components["schemas"]["BoardList"];
export type DirectoryTeam = components["schemas"]["DirectoryTeam"];
export type DirectoryMember = components["schemas"]["DirectoryMember"];
export type CurrentUser = components["schemas"]["CurrentUser"];
export type UserPreferences = components["schemas"]["UserPreferences"];
export type CardMutation = components["schemas"]["CardMutationResponse"];

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

export function teamMembers(bootstrap: Bootstrap, teamKey: string) {
	return activeMembers(bootstrap).filter((member) => member.teamKeys.includes(teamKey));
}

export function preferredAssignee(bootstrap: Bootstrap, teamKey: string) {
	return bootstrap.preferences.defaultTeamKey === teamKey ? bootstrap.me.gitLabUserId : null;
}
