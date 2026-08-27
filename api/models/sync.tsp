namespace SitconBoard;

alias SyncOperation = "upsert" | "delete";

@doc("A lane's manual card order. Recorded per lane rather than per card because one drag renumbers every card in up to two lanes.")
model CardOrder {
	listKey: string;
	issueIids: int64[];
}

@doc("One recorded change. Payloads are full snapshots, so applying the same action twice is a no-op and the newest action for an entity subsumes every earlier one.")
model SyncAction {
	entity: string;

	@doc("Monotonic checkpoint. Gapless and issued in commit order.")
	syncId: string;

	entityId: string;
	operation: SyncOperation;
	actorGitLabUserId: int64 | null;
	occurredAt: utcDateTime;
}

model CardSyncAction extends SyncAction {
	entity: "card";
	card: BoardCard | null;
}

model CardOrderSyncAction extends SyncAction {
	entity: "cardOrder";
	order: CardOrder;
}

model ListSyncAction extends SyncAction {
	entity: "list";
	list: BoardList | null;
}

model TeamSyncAction extends SyncAction {
	entity: "team";
	team: DirectoryTeam | null;
}

model MemberSyncAction extends SyncAction {
	entity: "member";
	member: DirectoryMember | null;
}

model PreferenceSyncAction extends SyncAction {
	entity: "preference";
	preferences: UserPreferences;
}

model SyncStatusSyncAction extends SyncAction {
	entity: "syncStatus";
	sync: SyncStatus;
}

// Listed explicitly so generated clients get a union that narrows on `entity`
// rather than the open base model.
alias AnySyncAction =
	| CardSyncAction
	| CardOrderSyncAction
	| ListSyncAction
	| TeamSyncAction
	| MemberSyncAction
	| PreferenceSyncAction
	| SyncStatusSyncAction;

model SyncDeltaResponse {
	@doc("The checkpoint reached. Store it and send it back as `since` next time.")
	checkpoint: string;

	actions: AnySyncAction[];

	@doc("More actions remain past this batch; request again from the new checkpoint.")
	hasMore: boolean;
}
