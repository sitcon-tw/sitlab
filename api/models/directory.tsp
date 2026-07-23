namespace SitconBoard;

alias DirectoryMemberState = "active" | "blocked" | "deactivated";
alias TeamMembershipSource = "gitlab_directory" | "self_selected";

@example(#{
	gitLabUserId: 123456,
	username: "yorukot",
	displayName: "Yorukot",
	avatarUrl: null,
	profileUrl: "https://gitlab.com/yorukot",
	accessLevel: 40,
	state: "active",
	teamKeys: #["development"],
})
model DirectoryMember {
	gitLabUserId: int64;
	username: string;
	displayName: string;

	@format("uri")
	avatarUrl: string | null;

	@format("uri")
	profileUrl: string;

	@minValue(0)
	@maxValue(100)
	accessLevel: int32;

	state: DirectoryMemberState;
	teamKeys: string[];
}

@example(#{
	key: "development",
	name: "開發組",
	titlePrefix: "[開發組]",
	gitLabLabel: "Team::開發組",
	active: true,
	sortOrder: 8,
	memberGitLabUserIds: #[123456],
})
model DirectoryTeam {
	@minLength(1)
	@maxLength(80)
	key: string;

	@minLength(1)
	@maxLength(80)
	name: string;

	@minLength(1)
	@maxLength(120)
	titlePrefix: string;

	@minLength(1)
	@maxLength(255)
	gitLabLabel: string;

	active: boolean;
	sortOrder: int32;
	memberGitLabUserIds: int64[];
}

model DirectorySnapshot {
	teams: DirectoryTeam[];
	members: DirectoryMember[];
	sourceRevision: string;
	syncedAt: utcDateTime;
}

model DirectoryResponse {
	directory: DirectorySnapshot;
}

@example(#{ defaultTeamKey: "development", confirmedAt: utcDateTime.fromISO("2026-07-14T08:00:00Z"), directoryTeamKeys: #["development"] })
model UserPreferences {
	defaultTeamKey: string | null;
	confirmedAt: utcDateTime | null;
	directoryTeamKeys: string[];
}

@example(#{ defaultTeamKey: "development" })
model UpdatePreferencesRequest {
	@minLength(1)
	@maxLength(80)
	defaultTeamKey: string;
}

model PreferencesResponse {
	preferences: UserPreferences;
}
