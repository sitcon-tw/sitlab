import type { BoardCard, DirectoryMember, DirectoryTeam } from "./model";

export function boardSearchTerms(query: string) {
	return normalizeSearchText(query).split(/\s+/).filter(Boolean);
}

export function createBoardSearchIndex(cards: BoardCard[], teams: DirectoryTeam[], members: DirectoryMember[]) {
	const teamByKey = new Map(teams.map((team) => [team.key, team]));
	const memberById = new Map(members.map((member) => [member.gitLabUserId, member]));
	return new Map(
		cards.map((card) => {
			const team = teamByKey.get(card.teamKey);
			const assignees = card.assigneeGitLabUserIds.flatMap((id) => {
				const member = memberById.get(id);
				return member ? [member.displayName, member.username, `@${member.username}`] : [];
			});
			const fields = [
				card.title,
				card.description,
				String(card.issueIid),
				`#${card.issueIid}`,
				card.teamKey,
				team?.name ?? "",
				team?.titlePrefix ?? "",
				...assignees,
				...card.labels
			];
			return [card.issueIid, normalizeSearchText(fields.join(" "))] as const;
		})
	);
}

export function matchesBoardSearch(haystack: string | undefined, terms: string[]) {
	return terms.length === 0 || Boolean(haystack && terms.every((term) => haystack.includes(term)));
}

function normalizeSearchText(value: string) {
	return value.normalize("NFKC").toLocaleLowerCase("zh-Hant").trim();
}
