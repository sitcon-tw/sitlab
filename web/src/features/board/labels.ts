import type { Bootstrap } from "./model";

/**
 * Mirror of server/internal/domain/board/labels.go. Keep both in sync;
 * labels.test.ts mirrors that file's table so drift shows up as a test diff.
 */
export const teamLabelPrefix = "Team::";

export const deprecatedLabels = new Set(["Wating", "Waiting", "Inbox", "To Do", "Todo", "Doing", "Review", "Closed", "組別::總召", "組別::行政", "組別::開發"]);

/** Legacy workflow labels and lifecycle labels the board manages itself. */
export function isDeprecatedLabel(name: string) {
	return deprecatedLabels.has(name) || name.startsWith("Status::");
}

/**
 * Labels owned by the board configuration rather than by users: the whole
 * `Team::` prefix plus every configured team label, and the legacy workflow
 * names. The prefix matters on its own — a user-created `Team::新組` would
 * behave like an ordinary label until someone adds it to board-directory.yml,
 * and would then start reassigning cards.
 */
export function isReservedLabel(bootstrap: Bootstrap, name: string) {
	const trimmed = name.trim();
	if (!trimmed) return true;
	if (trimmed.startsWith(teamLabelPrefix)) return true;
	if (isDeprecatedLabel(trimmed)) return true;
	return bootstrap.teams.some((team) => team.gitLabLabel === trimmed);
}

/** Card labels reduced to general labels plus exactly the active team's label. */
export function canonicalClientLabels(bootstrap: Bootstrap, labels: string[], teamKey: string) {
	const teamLabels = new Set(bootstrap.teams.map((team) => team.gitLabLabel));
	const general = labels.filter((label) => !teamLabels.has(label) && !isDeprecatedLabel(label));
	const teamLabel = bootstrap.teams.find((team) => team.key === teamKey && team.active)?.gitLabLabel;
	return [...general, ...(teamLabel ? [teamLabel] : [])];
}
