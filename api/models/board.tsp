namespace SitconBoard;

alias OperationState = "pending" | "processing" | "synced" | "failed";
alias OperationKind =
	| "create_card"
	| "update_details"
	| "update_team"
	| "update_assignee"
	| "update_start_date"
	| "update_due_date"
	| "update_labels"
	| "move_card";
alias SyncHealth = "synced" | "syncing" | "offline";

model BoardList {
	key: string;
	name: string;
	position: int32;
	closed: boolean;
	color: string;
}

model BoardCard {
	issueIid: int64;
	issueId: int64 | null;
	title: string;
	description: string;
	webUrl: string | null;
	listKey: string;
	position: int32;
	teamKey: string;
	assigneeGitLabUserIds: int64[];
	startDate: plainDate | null;
	dueDate: plainDate | null;
	labels: string[];
	syncState: OperationState;
	syncError: string | null;
	pendingOperationId: uuid | null;
	createdAt: utcDateTime;
	updatedAt: utcDateTime;
}

model BoardSnapshot {
	lists: BoardList[];
	cards: BoardCard[];
	syncedAt: utcDateTime;
}

model SyncStatus {
	state: SyncHealth;
	lastSuccessAt: utcDateTime;
	message: string | null;
}

model BootstrapResponse {
	revision: string;
	me: CurrentUser;

	@secret
	csrfToken: string;

	teams: DirectoryTeam[];
	members: DirectoryMember[];
	board: BoardSnapshot;
	preferences: UserPreferences;
	sync: SyncStatus;
}

model DurableOperation {
	id: uuid;
	kind: OperationKind;
	state: OperationState;
	attempts: int32;
	lastError: string | null;
	createdAt: utcDateTime;
	updatedAt: utcDateTime;
}

model CardMutationResponse {
	card: BoardCard;
	operation: DurableOperation;
}

@example(#{ id: 10, name: "priority::high", color: "#D73A4A", textColor: "#FFFFFF", description: "Needs prompt attention" })
model ProjectLabel {
	id: int64;
	name: string;

	@pattern("^#[0-9A-Fa-f]{6}$")
	color: string;

	@pattern("^#[0-9A-Fa-f]{6}$")
	textColor: string;

	description: string | null;
}

model CardCommentAuthor {
	gitLabUserId: int64;
	username: string;
	displayName: string;
	avatarUrl: string | null;
	profileUrl: string;
}

@example(#{
	id: 9876,
	body: "已確認寄信重試條件，請協助 review。",
	author: #{ gitLabUserId: 114, username: "yorukot", displayName: "Yorukot", avatarUrl: null, profileUrl: "https://gitlab.com/yorukot" },
	system: false,
	createdAt: utcDateTime.fromISO("2026-07-29T08:00:00Z"),
	updatedAt: utcDateTime.fromISO("2026-07-29T08:00:00Z"),
})
model CardComment {
	id: int64;
	body: string;
	author: CardCommentAuthor;
	system: boolean;
	createdAt: utcDateTime;
	updatedAt: utcDateTime;
}

model ProjectLabelsResponse {
	labels: ProjectLabel[];
}

@example(#{ name: "Priority::High", color: "#D73A4A", description: "Needs prompt attention" })
model CreateProjectLabelRequest {
	@minLength(1)
	@maxLength(255)
	name: string;

	@pattern("^#[0-9A-Fa-f]{6}$")
	color: string;

	description: string | null;
}

@example(#{ name: "Priority::Urgent", color: "#B60205", description: null })
model UpdateProjectLabelRequest {
	@minLength(1)
	@maxLength(255)
	name: string;

	@pattern("^#[0-9A-Fa-f]{6}$")
	color: string;

	description: string | null;
}

model CardCommentsResponse {
	comments: CardComment[];
}

model CreateCardRequest {
	operationId: uuid;

	@minLength(1)
	@maxLength(255)
	title: string;

	@minLength(1)
	teamKey: string;

	@minLength(1)
	listKey: string;

	description: string;
	assigneeGitLabUserIds: int64[];
	labels: string[];
	startDate: plainDate | null;
	dueDate: plainDate | null;
}

model UpdateCardDetailsRequest {
	operationId: uuid;

	@minLength(1)
	@maxLength(255)
	title: string;

	description: string;
}

model UpdateCardTeamRequest {
	operationId: uuid;

	@minLength(1)
	teamKey: string;
}

model UpdateCardAssigneeRequest {
	operationId: uuid;
	assigneeGitLabUserIds: int64[];
}

model UpdateCardDueDateRequest {
	operationId: uuid;
	dueDate: plainDate | null;
}

model UpdateCardStartDateRequest {
	operationId: uuid;
	startDate: plainDate | null;
}

model UpdateCardLabelsRequest {
	operationId: uuid;
	labels: string[];
}

model CreateCardCommentRequest {
	@minLength(1)
	body: string;
}

model MoveCardRequest {
	operationId: uuid;

	@minLength(1)
	listKey: string;

	@minValue(0)
	position: int32;
}

model RetryOperationResponse {
	operation: DurableOperation;
}

model RefreshSyncResponse {
	acceptedAt: utcDateTime;
}
