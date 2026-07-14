namespace SitconBoard;

alias OperationState = "pending" | "processing" | "synced" | "failed";
alias OperationKind = "create_card" | "update_team" | "update_assignee" | "update_due_date" | "move_card";
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
	webUrl: string | null;
	listKey: string;
	position: int32;
	teamKey: string;
	assigneeGitLabUserId: int64 | null;
	dueDate: plainDate | null;
	labels: string[];
	syncState: OperationState;
	syncError: string | null;
	pendingOperationId: uuid | null;
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

	assigneeGitLabUserId: int64 | null;
	dueDate: plainDate | null;
}

model UpdateCardTeamRequest {
	operationId: uuid;

	@minLength(1)
	teamKey: string;
}

model UpdateCardAssigneeRequest {
	operationId: uuid;
	assigneeGitLabUserId: int64 | null;
}

model UpdateCardDueDateRequest {
	operationId: uuid;
	dueDate: plainDate | null;
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
