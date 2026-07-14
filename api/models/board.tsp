namespace SitconBoard;

alias OperationState = "pending" | "processing" | "synced" | "failed";
alias OperationKind = "create_card" | "update_details" | "update_team" | "update_assignee" | "update_start_date" | "update_due_date" | "move_card";
alias SyncHealth = "synced" | "syncing" | "offline";

model BoardList {
	key: string;
	name: string;
	gitLabLabel: string;
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

model CreateCardRequest {
	operationId: uuid;

	@minLength(1)
	@maxLength(255)
	title: string;

	@minLength(1)
	teamKey: string;

	description: string;
	assigneeGitLabUserIds: int64[];
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
