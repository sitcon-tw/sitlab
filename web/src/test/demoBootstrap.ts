import type { BoardCard, Bootstrap, DirectoryMember, DirectoryMilestone, DirectoryTeam } from "@/features/board/model";

const teamDefinitions = [
	["administration", "行政組", "行政", [101, 102]],
	["program", "議程組", "議程", [103, 104]],
	["activity", "活動組", "活動", [105, 106]],
	["venue", "場務組", "場務", [107, 108]],
	["design", "設計組", "設計", [109, 110]],
	["documentation", "紀錄組", "紀錄", [111]],
	["production", "製播組", "製播", [112, 113]],
	["development", "開發組", "開發", [114, 115, 116]],
	["editorial", "編輯組", "編輯", [117]],
	["marketing", "行銷組", "行銷", [118, 119]],
	["general-coordinator", "總召組", "總召", [120]]
] as const;

const memberDefinitions = [
	[101, "jolin", "林采欣"],
	[102, "morris", "陳柏宇"],
	[103, "astrid", "許雅筑"],
	[104, "weiting", "黃偉庭"],
	[105, "sharon", "張庭瑄"],
	[106, "hau", "劉家豪"],
	[107, "yucheng", "李育誠"],
	[108, "yen", "郭佳燕"],
	[109, "mia", "周美亞"],
	[110, "leon", "王立恩"],
	[111, "river", "江書妍"],
	[112, "kai", "蔡承凱"],
	[113, "faye", "吳芳瑜"],
	[114, "yorukot", "Yorukot"],
	[115, "ming", "沈明軒"],
	[116, "anita", "謝安庭"],
	[117, "tina", "鄭庭安"],
	[118, "sean", "羅翔宇"],
	[119, "claire", "何嘉玲"],
	[120, "alice", "Alice"],
	[121, "newmember", "新加入成員"]
] as const;

const teams: DirectoryTeam[] = teamDefinitions.map(([key, name, , memberGitLabUserIds], index) => ({
	key,
	name,
	titlePrefix: `[${name}]`,
	gitLabLabel: `Team::${name}`,
	active: true,
	sortOrder: index + 1,
	memberGitLabUserIds: [...memberGitLabUserIds],
	leaderGitLabUserIds: memberGitLabUserIds.length ? [memberGitLabUserIds[0]] : []
}));

const members: DirectoryMember[] = memberDefinitions.map(([gitLabUserId, username, displayName]) => ({
	gitLabUserId,
	username,
	displayName,
	avatarUrl: null,
	profileUrl: `https://gitlab.com/${username}`,
	accessLevel: 30,
	state: "active",
	teamKeys: teams.filter((team) => team.memberGitLabUserIds.includes(gitLabUserId)).map((team) => team.key)
}));

const milestoneDefinitions = [
	["負一籌與 BoF", "2026-08-08", "organizing"],
	["零籌", "2026-08-15", "organizing"],
	["站立會議", "2026-08-22", "standup"],
	["一籌", "2026-08-29", "organizing"],
	["站立會議", "2026-09-05", "standup"],
	["二籌", "2026-09-12", "organizing"],
	["站立會議", "2026-09-19", "standup"],
	["三籌", "2026-09-26", "organizing"],
	["站立會議", "2026-10-03", "standup"],
	["四籌", "2026-10-10", "organizing"],
	["站立會議", "2026-10-17", "standup"],
	["五籌", "2026-10-24", "organizing"],
	["站立會議", "2026-10-31", "standup"],
	["六籌", "2026-11-07", "organizing"],
	["站立會議", "2026-11-14", "standup"],
	["七籌", "2026-11-21", "organizing"],
	["站立會議", "2026-11-28", "standup"],
	["八籌", "2026-12-05", "organizing"],
	["站立會議", "2026-12-12", "standup"],
	["九籌", "2026-12-19", "organizing"],
	["站立會議", "2026-12-26", "standup"],
	["十籌", "2027-01-02", "organizing"],
	["站立會議", "2027-01-09", "standup"],
	["十一籌", "2027-01-16", "organizing"],
	["十二籌", "2027-01-23", "organizing"],
	["十三籌", "2027-01-30", "organizing"],
	["十四籌", "2027-02-06", "organizing"],
	["十五籌", "2027-02-13", "organizing"],
	["十六籌", "2027-02-20", "organizing"],
	["十七籌", "2027-02-27", "organizing"],
	["十八籌", "2027-03-06", "organizing"],
	["年會", "2027-03-13", "conference"]
] as const satisfies ReadonlyArray<readonly [string, string, DirectoryMilestone["kind"]]>;

const milestones: DirectoryMilestone[] = milestoneDefinitions.map(([name, date, kind]) => ({ name, date, kind }));

function card(
	issueIid: number,
	title: string,
	description: string,
	listKey: string,
	position: number,
	teamKey: string,
	assigneeGitLabUserIds: number[],
	startDate: string | null,
	dueDate: string | null,
	syncState: BoardCard["syncState"] = "synced"
): BoardCard {
	const gitLabStatusNames: Record<string, string> = {
		wating: "Waiting",
		inbox: "Inbox",
		todo: "To do",
		doing: "Doing",
		review: "Review",
		closed: "Done"
	};
	const labels = [teams.find((team) => team.key === teamKey)?.gitLabLabel, ...(issueIid === 127 ? ["Priority::High", "Backend"] : [])].filter(
		(label): label is string => Boolean(label)
	);
	return {
		issueIid,
		issueId: 9000 + issueIid,
		title,
		description,
		webUrl: `https://gitlab.com/sitcon-tw/2027/-/issues/${issueIid}`,
		listKey,
		position,
		teamKey,
		assigneeGitLabUserIds,
		startDate,
		dueDate,
		labels,
		gitLabStatusName: gitLabStatusNames[listKey] ?? null,
		syncState,
		syncError: syncState === "failed" ? "GitLab 暫時無法更新，請稍後重試。" : null,
		pendingOperationId: syncState === "failed" ? "10000000-0000-4000-8000-000000000099" : null,
		createdAt: "2026-07-10T03:30:00Z",
		updatedAt: "2026-07-14T08:00:00Z"
	};
}

export const demoBootstrap: Bootstrap = {
	revision: "demo-1",
	me: {
		id: "11111111-1111-4111-8111-111111111111",
		gitLabUserId: 114,
		username: "yorukot",
		displayName: "Yorukot",
		avatarUrl: null,
		profileUrl: "https://gitlab.com/yorukot",
		accessLevel: 40
	},
	csrfToken: "demo-csrf-token",
	teams,
	members,
	milestones,
	board: {
		lists: [
			{ key: "wating", name: "Waiting", position: 0, closed: false, color: "critical" },
			{ key: "inbox", name: "Inbox", position: 1, closed: false, color: "neutral" },
			{ key: "todo", name: "To do", position: 2, closed: false, color: "accent" },
			{ key: "doing", name: "Doing", position: 3, closed: false, color: "info" },
			{ key: "review", name: "Review", position: 4, closed: false, color: "warning" },
			{ key: "closed", name: "Close", position: 5, closed: true, color: "success" }
		],
		cards: [
			card(127, "修正報名系統寄信流程", "釐清失敗重送條件，補上整合測試與觀測紀錄。", "todo", 0, "development", [114], "2026-07-17", "2026-07-21"),
			card(128, "整理志工行前通知", "", "inbox", 0, "administration", [101, 102], null, "2026-07-23"),
			card(129, "確認議程講者資料", "", "wating", 0, "program", [], null, "2026-07-25"),
			card(130, "製作工作人員識別證", "", "doing", 0, "design", [109], "2026-07-16", "2026-07-20"),
			card(131, "盤點會場網路設備", "", "doing", 1, "venue", [107], null, "2026-07-22", "failed"),
			card(132, "校對官網議程文案", "", "review", 0, "editorial", [117], "2026-07-15", "2026-07-19"),
			card(133, "完成主視覺社群素材", "", "closed", 0, "marketing", [118, 119], "2026-07-12", "2026-07-16")
		],
		syncedAt: "2026-07-14T08:00:00Z"
	},
	preferences: {
		defaultTeamKey: "development",
		confirmedAt: "2026-07-14T08:00:00Z",
		directoryTeamKeys: ["development"]
	},
	sync: {
		state: "synced",
		lastSuccessAt: "2026-07-14T08:00:00Z",
		message: null
	}
};
