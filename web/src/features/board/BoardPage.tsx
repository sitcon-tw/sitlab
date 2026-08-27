import { clearCsrfToken, errorMessage } from "@/shared/api/client";
import { Avatar } from "@/shared/Avatar";
import { PointerActivationConstraints } from "@dnd-kit/dom";
import { DragDropProvider, DragOverlay, PointerSensor, useDroppable } from "@dnd-kit/react";
import { useSortable } from "@dnd-kit/react/sortable";
import { Dialog, Drawer, SegmentedButton, StaticChip } from "@project-template/ui";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
	ArrowDown,
	ArrowUp,
	Check,
	ChevronDown,
	ChevronLeft,
	ChevronRight,
	CloudOff,
	Ellipsis,
	ExternalLink,
	GripVertical,
	LogOut,
	Plus,
	RefreshCw,
	Save,
	Send,
	Settings,
	SquareTerminal,
	Users,
	X
} from "lucide-react";
import { useEffect, useId, useRef, useState } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { AssigneePicker } from "./AssigneePicker";
import {
	createCard,
	createComment,
	listComments,
	logout,
	moveCard,
	retryOperation,
	savePreferences,
	updateAssignee,
	updateDetails,
	updateDueDate,
	updateLabels,
	updateStartDate,
	updateTeam
} from "./boardApi";
import { BoardFilters } from "./BoardFilters";
import { planCardMove } from "./boardOrder";
import styles from "./BoardPage.module.css";
import { CardLabels } from "./CardLabels";
import { LabelManagerDialog } from "./LabelManagerDialog";
import { canonicalClientLabels, isDeprecatedLabel } from "./labels";
import { MembersDrawer } from "./MembersDrawer";
import {
	compareBoardCards,
	memberById,
	preferredAssignees,
	taipeiDateAfter,
	teamLeaders,
	type BoardCard,
	type BoardSortMode,
	type Bootstrap,
	type CardComment,
	type ProjectLabel
} from "./model";
import { parseQuickAction, quickActionCommands, type QuickAction } from "./quickActions";
import { SaveIndicator } from "./SaveIndicator";
import { TagSwatch } from "./TagSwatch";
import { useBoardDrag } from "./useBoardDrag";
import { useFieldSaveState, type FieldSave, type FieldSaveState, type SaveField } from "./useFieldSaveState";
import { useProjectLabelMap, useProjectLabels } from "./useProjectLabels";
import { parseBoardViewState, serializeBoardViewState } from "./viewState";

export interface BoardPageProps {
	bootstrap: Bootstrap;
	updateBootstrap: (update: (current: Bootstrap) => Bootstrap) => void;
	backgroundOffline: boolean;
	onDraggingChange?: (dragging: boolean) => void;
}

type CardPatch = Partial<
	Pick<BoardCard, "title" | "description" | "teamKey" | "assigneeGitLabUserIds" | "startDate" | "dueDate" | "labels" | "listKey" | "position">
>;
type CreateCardInput = {
	title: string;
	description: string;
	teamKey: string;
	listKey: string;
	assigneeGitLabUserIds: number[];
	labels: string[];
	startDate: string | null;
	dueDate: string | null;
};

const noopDraggingChange = () => undefined;
const boardPointerSensor = PointerSensor.configure({
	activationConstraints(event) {
		return event.pointerType === "touch"
			? [new PointerActivationConstraints.Delay({ value: 180, tolerance: { x: 8, y: 8 } })]
			: [new PointerActivationConstraints.Distance({ value: 6 })];
	}
});

export function BoardPage({ bootstrap, updateBootstrap, backgroundOffline, onDraggingChange = noopDraggingChange }: BoardPageProps) {
	const [initialView] = useState(() => parseBoardViewState(window.location.search, bootstrap));
	const [membersOpen, setMembersOpen] = useState(false);
	const [detailIid, setDetailIid] = useState<number | null>(null);
	const [filterTeamKey, setFilterTeamKey] = useState(initialView.teamKey);
	const [filterMemberIds, setFilterMemberIds] = useState<number[]>(initialView.memberIds);
	const [filterLabels, setFilterLabels] = useState<string[]>(initialView.labels);
	const [sortMode, setSortMode] = useState<BoardSortMode>(initialView.sortMode);
	const [undo, setUndo] = useState<{ cardIid: number; assigneeIds: number[]; assigneeNames: string[] } | null>(null);
	const localRetries = useRef(new Map<string, () => void>());
	const save = useFieldSaveState();
	const labelsQuery = useProjectLabels();
	const labelMetadata = useProjectLabelMap();

	// Label filters are AND-combined, so one that matches nothing hides the whole
	// board. Renaming or deleting a label strands exactly such a filter, and so
	// does an old bookmark. Ignore a filter once the catalog has loaded and no
	// card carries the name either, which is when it provably matches nothing.
	// The user's selection is left alone — this only stops the board going blank.
	const appliedLabels = (() => {
		if (!labelsQuery.isSuccess) return filterLabels;
		const known = new Set(labelsQuery.data.map((label) => label.name));
		for (const card of bootstrap.board.cards) for (const label of card.labels) known.add(label);
		return filterLabels.filter((label) => known.has(label));
	})();
	const nextTemporaryIid = useRef(-1);
	const cards = bootstrap.board.cards;
	const filteredCards = cards.filter(
		(card) =>
			(!filterTeamKey || card.teamKey === filterTeamKey) &&
			(filterMemberIds.length === 0 || card.assigneeGitLabUserIds.some((id) => filterMemberIds.includes(id))) &&
			appliedLabels.every((label) => card.labels.includes(label))
	);
	const lists = [...bootstrap.board.lists].sort((a, b) => a.position - b.position);
	const orderedCards = lists.flatMap((list) => filteredCards.filter((card) => card.listKey === list.key).sort((a, b) => compareBoardCards(a, b, sortMode)));
	const detailCard = cards.find((card) => card.issueIid === detailIid) ?? null;
	const detailIndex = detailCard ? orderedCards.findIndex((card) => card.issueIid === detailCard.issueIid) : -1;
	const filtersActive = Boolean(filterTeamKey || filterMemberIds.length || filterLabels.length);

	useEffect(() => {
		const search = serializeBoardViewState(window.location.search, {
			teamKey: filterTeamKey,
			memberIds: filterMemberIds,
			labels: filterLabels,
			sortMode
		});
		const nextURL = `${window.location.pathname}${search}${window.location.hash}`;
		if (`${window.location.pathname}${window.location.search}${window.location.hash}` !== nextURL)
			window.history.replaceState(window.history.state, "", nextURL);
	}, [filterLabels, filterMemberIds, filterTeamKey, sortMode]);

	const replaceCard = (issueIid: number, card: BoardCard) => {
		updateBootstrap((current) => ({
			...current,
			board: { ...current.board, cards: current.board.cards.map((item) => (item.issueIid === issueIid ? card : item)) }
		}));
	};

	const patchCard = (issueIid: number, patch: Partial<BoardCard>) => {
		updateBootstrap((current) => ({
			...current,
			board: {
				...current.board,
				cards: current.board.cards.map((item) => (item.issueIid === issueIid ? { ...item, ...patch } : item))
			}
		}));
	};
	const replaceCardForOperation = (issueIid: number, operationId: string, card: BoardCard) => {
		updateBootstrap((current) => ({
			...current,
			board: {
				...current.board,
				cards: current.board.cards.map((item) => (item.issueIid === issueIid && item.pendingOperationId === operationId ? card : item))
			}
		}));
	};
	const patchCardForOperation = (issueIid: number, operationId: string, patch: Partial<BoardCard>) => {
		updateBootstrap((current) => ({
			...current,
			board: {
				...current.board,
				cards: current.board.cards.map((item) => (item.issueIid === issueIid && item.pendingOperationId === operationId ? { ...item, ...patch } : item))
			}
		}));
	};

	const runCardMutation = (card: BoardCard, patch: CardPatch, field: SaveField, request: (operationId: string) => ReturnType<typeof updateTeam>) => {
		const operationId = crypto.randomUUID();
		const optimistic: BoardCard = {
			...card,
			...patch,
			syncState: "pending",
			syncError: null,
			pendingOperationId: operationId,
			updatedAt: new Date().toISOString()
		};
		replaceCard(card.issueIid, optimistic);
		const execute = () => {
			request(operationId)
				.then((result) => {
					localRetries.current.delete(operationId);
					replaceCardForOperation(card.issueIid, operationId, result.card);
					save.settle(operationId, "saved");
				})
				.catch((cause: unknown) => {
					localRetries.current.set(operationId, execute);
					const message = errorMessage(cause, "變更尚未同步，請重試。");
					patchCardForOperation(card.issueIid, operationId, {
						...patch,
						syncState: "failed",
						syncError: message,
						pendingOperationId: operationId
					});
					save.settle(operationId, "failed", message);
				});
		};
		localRetries.current.set(operationId, execute);
		save.begin(card.issueIid, field, operationId);
		execute();
	};

	const handleCreate = (input: CreateCardInput) => {
		const operationId = crypto.randomUUID();
		const temporaryIid = nextTemporaryIid.current;
		nextTemporaryIid.current -= 1;
		const optimistic: BoardCard = {
			issueIid: temporaryIid,
			issueId: null,
			title: input.title,
			description: input.description,
			webUrl: null,
			listKey: input.listKey,
			position: 0,
			teamKey: input.teamKey,
			assigneeGitLabUserIds: input.assigneeGitLabUserIds,
			startDate: input.startDate,
			dueDate: input.dueDate,
			labels: canonicalClientLabels(bootstrap, input.labels, input.teamKey),
			syncState: "pending",
			syncError: null,
			pendingOperationId: operationId,
			createdAt: new Date().toISOString(),
			updatedAt: new Date().toISOString()
		};
		updateBootstrap((current) => ({ ...current, board: { ...current.board, cards: [optimistic, ...current.board.cards] } }));

		const execute = () => {
			createCard({ operationId, ...input })
				.then((result) => {
					localRetries.current.delete(operationId);
					replaceCardForOperation(temporaryIid, operationId, result.card);
				})
				.catch((cause: unknown) => {
					localRetries.current.set(operationId, execute);
					patchCardForOperation(temporaryIid, operationId, {
						syncState: "failed",
						syncError: errorMessage(cause, "卡片尚未建立，請重試。"),
						pendingOperationId: operationId
					});
				});
		};
		localRetries.current.set(operationId, execute);
		execute();
	};

	const handleTeam = (card: BoardCard, teamKey: string) => {
		const nextAssigneeIDs = card.assigneeGitLabUserIds.filter((id) => memberById(bootstrap, id)?.teamKeys.includes(teamKey));
		const removed = card.assigneeGitLabUserIds.flatMap((id) => {
			const member = memberById(bootstrap, id);
			return member && !nextAssigneeIDs.includes(id) ? [member] : [];
		});
		if (removed.length) {
			setUndo({
				cardIid: card.issueIid,
				assigneeIds: removed.map((member) => member.gitLabUserId),
				assigneeNames: removed.map((member) => member.displayName)
			});
		}
		const labels = canonicalClientLabels(bootstrap, card.labels, teamKey);
		runCardMutation(card, { teamKey, labels, assigneeGitLabUserIds: nextAssigneeIDs }, "team", (operationId) => updateTeam(card, operationId, teamKey));
	};

	const handleAssignee = (card: BoardCard, assigneeGitLabUserIds: number[]) => {
		runCardMutation(card, { assigneeGitLabUserIds }, "assignee", (operationId) => updateAssignee(card, operationId, assigneeGitLabUserIds));
	};

	const handleDetails = (card: BoardCard, title: string, description: string) => {
		runCardMutation(card, { title, description }, "details", (operationId) => updateDetails(card, operationId, title, description));
	};

	const handleDueDate = (card: BoardCard, dueDate: string | null) => {
		runCardMutation(card, { dueDate }, "dueDate", (operationId) => updateDueDate(card, operationId, dueDate));
	};

	const handleStartDate = (card: BoardCard, startDate: string | null) => {
		runCardMutation(card, { startDate }, "startDate", (operationId) => updateStartDate(card, operationId, startDate));
	};

	const handleLabels = (card: BoardCard, labels: string[]) => {
		runCardMutation(card, { labels }, "labels", (operationId) => updateLabels(card, operationId, labels));
	};

	const handleMove = (card: BoardCard, listKey: string) => {
		if (card.listKey === listKey) return;
		const position = cards.filter((item) => item.listKey === listKey && item.issueIid !== card.issueIid).length;
		handlePosition(card, listKey, position);
	};

	const handlePosition = (card: BoardCard, listKey: string, requestedPosition: number) => {
		const plan = planCardMove(cards, card.issueIid, listKey, requestedPosition);
		if (!plan) return;
		const positions = new Map(plan.patches.map((patch) => [patch.issueIid, patch]));
		const position = plan.position;
		const labels = canonicalClientLabels(bootstrap, card.labels, card.teamKey);
		const operationId = crypto.randomUUID();
		updateBootstrap((current) => ({
			...current,
			board: {
				...current.board,
				cards: current.board.cards.map((item) => {
					const next = positions.get(item.issueIid);
					if (!next) return item;
					if (item.issueIid !== card.issueIid) return { ...item, ...next };
					return {
						...item,
						...next,
						labels,
						syncState: "pending",
						syncError: null,
						pendingOperationId: operationId,
						updatedAt: new Date().toISOString()
					};
				})
			}
		}));

		const execute = () => {
			moveCard(card, operationId, listKey, position)
				.then((result) => {
					localRetries.current.delete(operationId);
					replaceCardForOperation(card.issueIid, operationId, result.card);
					save.settle(operationId, "saved");
				})
				.catch((cause: unknown) => {
					localRetries.current.set(operationId, execute);
					const message = errorMessage(cause, "卡片順序尚未同步，請重試。");
					patchCardForOperation(card.issueIid, operationId, {
						position,
						listKey,
						labels,
						syncState: "failed",
						syncError: message,
						pendingOperationId: operationId
					});
					save.settle(operationId, "failed", message);
				});
		};
		localRetries.current.set(operationId, execute);
		save.begin(card.issueIid, "status", operationId);
		execute();
	};

	const handleReorder = (card: BoardCard, adjacentCard: BoardCard, direction: "up" | "down") => {
		if (card.listKey !== adjacentCard.listKey) return;
		const laneCards = cards
			.filter((item) => item.listKey === card.listKey && item.issueIid !== card.issueIid)
			.sort((a, b) => compareBoardCards(a, b, "manual"));
		const adjacentIndex = laneCards.findIndex((item) => item.issueIid === adjacentCard.issueIid);
		if (adjacentIndex >= 0) handlePosition(card, card.listKey, direction === "up" ? adjacentIndex : adjacentIndex + 1);
	};
	const drag = useBoardDrag({
		cards,
		visibleCards: filteredCards,
		listKeys: lists.map((list) => list.key),
		enabled: sortMode === "manual",
		onMove: (cardIid, listKey, position) => {
			const card = cards.find((item) => item.issueIid === cardIid);
			if (card) handlePosition(card, listKey, position);
		},
		onDraggingChange
	});
	const draggedCard = drag.activeCardIid === null ? null : (cards.find((card) => card.issueIid === drag.activeCardIid) ?? null);
	const cardsForList = (listKey: string) => {
		if (!drag.dragGroups) return filteredCards.filter((card) => card.listKey === listKey).sort((a, b) => compareBoardCards(a, b, sortMode));
		return (drag.dragGroups[listKey] ?? []).flatMap((issueIid) => {
			const card = cards.find((item) => item.issueIid === issueIid);
			return card ? [{ ...card, listKey }] : [];
		});
	};

	const handleRetry = (card: BoardCard) => {
		if (!card.pendingOperationId) return;
		const localRetry = localRetries.current.get(card.pendingOperationId);
		patchCard(card.issueIid, { syncState: "pending", syncError: null });
		save.retry(card.pendingOperationId);
		if (localRetry) {
			localRetry();
			return;
		}
		retryOperation(card.pendingOperationId).catch((cause: unknown) => {
			patchCard(card.issueIid, { syncState: "failed", syncError: errorMessage(cause, "無法重試這項變更。") });
		});
	};

	const restoreAssignee = () => {
		if (!undo) return;
		const card = bootstrap.board.cards.find((item) => item.issueIid === undo.cardIid);
		if (card) handleAssignee(card, [...card.assigneeGitLabUserIds, ...undo.assigneeIds]);
		setUndo(null);
	};

	return (
		<div className={styles.app}>
			<BoardHeader bootstrap={bootstrap} backgroundOffline={backgroundOffline} onMembers={() => setMembersOpen(true)} />
			<DirectoryConflict bootstrap={bootstrap} updateBootstrap={updateBootstrap} />
			<main className={styles.main}>
				<QuickCreate bootstrap={bootstrap} onCreate={handleCreate} />
				{undo ? (
					<div className={styles.undo} role="status">
						<span>已清除不屬於新組別的 Assignee：{undo.assigneeNames.join("、")}</span>
						<button type="button" onClick={restoreAssignee}>
							復原
						</button>
						<button type="button" aria-label="關閉提示" onClick={() => setUndo(null)}>
							×
						</button>
					</div>
				) : null}
				<BoardFilters
					bootstrap={bootstrap}
					teamKey={filterTeamKey}
					memberIds={filterMemberIds}
					labels={filterLabels}
					sortMode={sortMode}
					visibleCount={filteredCards.length}
					totalCount={cards.length}
					onTeamChange={setFilterTeamKey}
					onMemberIdsChange={setFilterMemberIds}
					onLabelsChange={setFilterLabels}
					onSortModeChange={setSortMode}
					onClear={() => {
						setFilterTeamKey("");
						setFilterMemberIds([]);
						setFilterLabels([]);
					}}
				/>
				<DragDropProvider
					sensors={(defaults) => [...defaults.filter((sensor) => sensor !== PointerSensor), boardPointerSensor]}
					onDragStart={drag.onDragStart}
					onDragOver={drag.onDragOver}
					onDragEnd={drag.onDragEnd}
				>
					<section className={styles.board} aria-label="SITCON 2027 工作看板">
						{lists.map((list) => {
							const listCards = cardsForList(list.key);
							return (
								<DroppableLane listKey={list.key} key={list.key}>
									<header className={styles.laneHeader}>
										<h2>{list.name}</h2>
										<span>{listCards.length}</span>
									</header>
									<div className={styles.cardList}>
										{listCards.map((card, index) => (
											<CardItem
												key={card.issueIid}
												card={card}
												bootstrap={bootstrap}
												onOpen={() => setDetailIid(card.issueIid)}
												onAssignee={(memberIds) => handleAssignee(card, memberIds)}
												onDueDate={(dueDate) => handleDueDate(card, dueDate)}
												manualOrder={sortMode === "manual"}
												sortableIndex={index}
												onMoveUp={index > 0 ? () => handleReorder(card, listCards[index - 1]!, "up") : undefined}
												onMoveDown={index < listCards.length - 1 ? () => handleReorder(card, listCards[index + 1]!, "down") : undefined}
												onRetry={() => handleRetry(card)}
												labelMetadata={labelMetadata}
											/>
										))}
										{listCards.length === 0 ? <p className={styles.emptyLane}>{filtersActive ? "沒有符合篩選的卡片" : "目前沒有卡片"}</p> : null}
									</div>
								</DroppableLane>
							);
						})}
					</section>
					<DragOverlay dropAnimation={null}>{draggedCard ? <CardDragPreview card={draggedCard} bootstrap={bootstrap} /> : null}</DragOverlay>
				</DragDropProvider>
			</main>
			{detailCard ? (
				<CardDetail
					key={detailCard.issueIid}
					card={detailCard}
					bootstrap={bootstrap}
					onClose={() => setDetailIid(null)}
					onPrevious={detailIndex > 0 ? () => setDetailIid(orderedCards[detailIndex - 1]!.issueIid) : undefined}
					onNext={detailIndex >= 0 && detailIndex < orderedCards.length - 1 ? () => setDetailIid(orderedCards[detailIndex + 1]!.issueIid) : undefined}
					position={detailIndex + 1}
					total={orderedCards.length}
					onDetails={(title, description) => handleDetails(detailCard, title, description)}
					onTeam={(teamKey) => handleTeam(detailCard, teamKey)}
					onMove={(listKey) => handleMove(detailCard, listKey)}
					onAssignee={(memberIds) => handleAssignee(detailCard, memberIds)}
					onStartDate={(startDate) => handleStartDate(detailCard, startDate)}
					onDueDate={(dueDate) => handleDueDate(detailCard, dueDate)}
					onLabels={(labels) => handleLabels(detailCard, labels)}
					save={save}
				/>
			) : null}
			<MembersDrawer bootstrap={bootstrap} open={membersOpen} onOpenChange={setMembersOpen} />
		</div>
	);
}

function BoardHeader({ bootstrap, backgroundOffline, onMembers }: { bootstrap: Bootstrap; backgroundOffline: boolean; onMembers: () => void }) {
	const offline = backgroundOffline || bootstrap.sync.state === "offline";
	const handleLogout = async () => {
		try {
			await logout();
		} finally {
			clearCsrfToken();
			window.location.assign("/");
		}
	};

	return (
		<header className={styles.topbar}>
			<div className={styles.product}>
				<img src="/sitcon-white.svg" alt="SITCON" />
				<span>2027 · Board</span>
			</div>
			<nav className={styles.headerActions} aria-label="看板工具">
				<button type="button" className={styles.headerButton} aria-label="成員" title="查看籌備團隊" onClick={onMembers}>
					<Users size="1rem" aria-hidden="true" />
					<span>成員</span>
				</button>
				{offline ? (
					<span className={styles.sync} data-state="offline" title={bootstrap.sync.message ?? `最後同步：${formatDateTime(bootstrap.sync.lastSuccessAt)}`}>
						<CloudOff size="0.9375rem" aria-hidden="true" />
						<span>離線</span>
						<span className={styles.syncAge}>· {relativeAge(bootstrap.sync.lastSuccessAt)}</span>
					</span>
				) : null}
				<details className={styles.account}>
					<summary aria-label="開啟帳號選單">
						<Avatar name={bootstrap.me.displayName} src={bootstrap.me.avatarUrl} />
						<ChevronDown size="0.875rem" aria-hidden="true" />
					</summary>
					<div className={styles.accountMenu}>
						<div>
							<strong>{bootstrap.me.displayName}</strong>
							<span>@{bootstrap.me.username}</span>
						</div>
						<a href={bootstrap.me.profileUrl} target="_blank" rel="noreferrer">
							<ExternalLink size="0.875rem" aria-hidden="true" /> GitLab 個人頁
						</a>
						<button type="button" onClick={() => void handleLogout()}>
							<LogOut size="0.875rem" aria-hidden="true" /> 登出
						</button>
					</div>
				</details>
			</nav>
		</header>
	);
}

function QuickCreate({ bootstrap, onCreate }: { bootstrap: Bootstrap; onCreate: (input: CreateCardInput) => void }) {
	const defaultTeam = bootstrap.preferences.defaultTeamKey ?? bootstrap.teams.find((team) => team.active)?.key ?? "";
	const lists = [...bootstrap.board.lists].sort((a, b) => a.position - b.position);
	const defaultList = lists.find((list) => list.key === "inbox")?.key ?? lists[0]?.key ?? "";
	const [mode, setMode] = useState<"single" | "leaders">("single");
	const [title, setTitle] = useState("");
	const [teamKey, setTeamKey] = useState(defaultTeam);
	const [listKey, setListKey] = useState(defaultList);
	const [description, setDescription] = useState("");
	const [labels, setLabels] = useState<string[]>([]);
	const [moreOpen, setMoreOpen] = useState(false);
	const [draftListKey, setDraftListKey] = useState(defaultList);
	const [draftDescription, setDraftDescription] = useState("");
	const [draftLabels, setDraftLabels] = useState<string[]>([]);
	const [assignees, setAssignees] = useState<number[]>(preferredAssignees(bootstrap, defaultTeam));
	const [dueDate, setDueDate] = useState(taipeiDateAfter(7));
	const [clearedAssignees, setClearedAssignees] = useState<number[]>([]);
	const teams = bootstrap.teams.filter((team) => team.active).sort((a, b) => a.sortOrder - b.sortOrder);
	const leaderTargets = teams.map((team) => ({ team, leaders: teamLeaders(bootstrap, team.key) })).filter((target) => target.leaders.length > 0);
	const leaderCount = leaderTargets.reduce((count, target) => count + target.leaders.length, 0);
	const selectedListName = lists.find((list) => list.key === listKey)?.name ?? "Inbox";
	const moreActive = listKey !== defaultList || Boolean(description.trim()) || labels.length > 0;

	const changeTeam = (nextTeam: string) => {
		const compatible = assignees.filter((id) => memberById(bootstrap, id)?.teamKeys.includes(nextTeam));
		setClearedAssignees(assignees.filter((id) => !compatible.includes(id)));
		setAssignees(compatible);
		setTeamKey(nextTeam);
	};
	const changeMoreOpen = (next: boolean) => {
		if (next) {
			setDraftListKey(listKey);
			setDraftDescription(description);
			setDraftLabels(labels);
		}
		setMoreOpen(next);
	};
	const applyMoreOptions = () => {
		setListKey(draftListKey);
		setDescription(draftDescription);
		setLabels(draftLabels);
		setMoreOpen(false);
	};

	const submit = (event: React.FormEvent) => {
		event.preventDefault();
		const normalized = title.trim();
		if (!normalized) return;
		if (mode === "leaders") {
			for (const target of leaderTargets) {
				onCreate({
					title: normalized,
					description,
					teamKey: target.team.key,
					listKey,
					assigneeGitLabUserIds: target.leaders.map((leader) => leader.gitLabUserId),
					labels,
					startDate: null,
					dueDate: dueDate || null
				});
			}
		} else {
			if (!teamKey) return;
			onCreate({ title: normalized, description, teamKey, listKey, assigneeGitLabUserIds: assignees, labels, startDate: null, dueDate: dueDate || null });
		}
		setTitle("");
		setDescription("");
		setLabels([]);
		setDraftDescription("");
		setDraftLabels([]);
		setMoreOpen(false);
	};

	return (
		<form className={styles.quickCreate} onSubmit={submit}>
			<SegmentedButton
				className={styles.createModes}
				label="開卡模式"
				value={mode}
				onChange={setMode}
				options={[
					{ value: "single", label: "單一組別" },
					{ value: "leaders", label: "所有組長" }
				]}
			/>
			{mode === "single" ? (
				<>
					<label className={styles.srOnly} htmlFor="quick-team">
						新卡片組別
					</label>
					<select id="quick-team" className={styles.quickTeam} value={teamKey} onChange={(event) => changeTeam(event.target.value)}>
						{teams.map((team) => (
							<option key={team.key} value={team.key}>
								{team.name}
							</option>
						))}
					</select>
				</>
			) : (
				<span className={styles.bulkTarget}>{leaderTargets.length ? `${leaderTargets.length} 組` : "尚未設定組長"}</span>
			)}
			<label className={styles.srOnly} htmlFor="quick-title">
				卡片標題
			</label>
			<input
				id="quick-title"
				value={title}
				maxLength={255}
				onChange={(event) => setTitle(event.target.value)}
				placeholder="輸入新卡片標題..."
				autoComplete="off"
			/>
			{mode === "single" ? (
				<AssigneePicker bootstrap={bootstrap} teamKey={teamKey} value={assignees} onChange={setAssignees} label="選擇新卡片 Assignee" />
			) : (
				<span className={styles.bulkAssignees}>{leaderCount ? `${leaderCount} 人` : "等待名單"}</span>
			)}
			<label className={styles.dateControl} title="新卡片期限">
				<span className={styles.srOnly}>期限</span>
				<input type="date" value={dueDate} aria-label="新卡片期限" onChange={(event) => setDueDate(event.target.value)} />
			</label>
			<Dialog
				open={moreOpen}
				onOpenChange={changeMoreOpen}
				title="更多建卡選項"
				trigger={
					<button
						type="button"
						className={styles.quickMoreButton}
						data-active={moreActive}
						aria-label="更多建卡選項"
						title={`更多建卡選項：${selectedListName}${description.trim() ? "，已填寫 Description" : ""}${labels.length ? `，${labels.length} 個 Labels` : ""}`}
					>
						<Ellipsis size="1.125rem" aria-hidden="true" />
					</button>
				}
				footer={
					<>
						<button type="button" className={styles.quickMoreCancel} onClick={() => setMoreOpen(false)}>
							取消
						</button>
						<button type="button" className={styles.quickMoreApply} onClick={applyMoreOptions}>
							套用
						</button>
					</>
				}
			>
				<div className={styles.quickMoreFields}>
					<label>
						<span>Status</span>
						<select aria-label="新卡片 Status" value={draftListKey} onChange={(event) => setDraftListKey(event.target.value)}>
							{lists.map((list) => (
								<option key={list.key} value={list.key}>
									{list.name}
								</option>
							))}
						</select>
					</label>
					<label>
						<span>Description</span>
						<textarea
							aria-label="新卡片 Description"
							value={draftDescription}
							onChange={(event) => setDraftDescription(event.target.value)}
							placeholder="輸入卡片描述..."
						/>
					</label>
					<QuickCreateLabels bootstrap={bootstrap} value={draftLabels} onChange={setDraftLabels} />
				</div>
			</Dialog>
			<button
				type="submit"
				className={styles.createButton}
				disabled={!title.trim() || !listKey || (mode === "leaders" && leaderTargets.length === 0)}
				aria-label={mode === "leaders" ? "為所有組長建立卡片" : "建立卡片"}
				title={mode === "leaders" ? "為所有組長建立卡片" : "建立卡片"}
			>
				<Plus size="1.125rem" aria-hidden="true" />
			</button>
			{mode === "single" && clearedAssignees.length ? (
				<p className={styles.quickNotice} role="status">
					已清除不屬於此組別的 Assignee
					<button
						type="button"
						onClick={() => {
							setAssignees([...assignees, ...clearedAssignees]);
							setClearedAssignees([]);
						}}
					>
						復原
					</button>
				</p>
			) : null}
		</form>
	);
}

function QuickCreateLabels({ bootstrap, value, onChange }: { bootstrap: Bootstrap; value: string[]; onChange: (labels: string[]) => void }) {
	const [query, setQuery] = useState("");
	const labelsQuery = useProjectLabels();
	const teamLabels = new Set(bootstrap.teams.map((team) => team.gitLabLabel));
	const normalizedQuery = query.trim().toLocaleLowerCase("zh-Hant");
	const available = (labelsQuery.data ?? []).filter(
		(label) =>
			!teamLabels.has(label.name) &&
			!isDeprecatedLabel(label.name) &&
			(!normalizedQuery || `${label.name} ${label.description ?? ""}`.toLocaleLowerCase("zh-Hant").includes(normalizedQuery))
	);
	const toggle = (label: string) => onChange(value.includes(label) ? value.filter((current) => current !== label) : [...value, label]);

	return (
		<section className={styles.quickLabels} aria-labelledby="quick-labels-heading">
			<header>
				<span id="quick-labels-heading">Labels</span>
				{value.length ? <small>已選擇 {value.length} 個</small> : null}
			</header>
			<input type="search" aria-label="搜尋新卡片 Label" placeholder="搜尋 Label 名稱或說明" value={query} onChange={(event) => setQuery(event.target.value)} />
			<div className={styles.quickLabelOptions}>
				{available.map((label) => (
					<label key={label.name}>
						<input type="checkbox" checked={value.includes(label.name)} onChange={() => toggle(label.name)} />
						<TagSwatch label={label} />
						<span>{label.name}</span>
					</label>
				))}
				{labelsQuery.isLoading ? <p role="status">載入 Labels...</p> : null}
				{labelsQuery.isError ? (
					<button type="button" onClick={() => void labelsQuery.refetch()}>
						<RefreshCw size="0.8125rem" aria-hidden="true" /> 重新載入 Labels
					</button>
				) : null}
				{labelsQuery.isSuccess && available.length === 0 ? <p>沒有符合的一般 Label</p> : null}
			</div>
		</section>
	);
}

function DroppableLane({ listKey, children }: { listKey: string; children: React.ReactNode }) {
	const { isDropTarget, ref } = useDroppable({ id: listKey, type: "lane", accept: "card", collisionPriority: -1 });
	return (
		<section ref={ref} className={styles.lane} data-list={listKey} data-drag-over={isDropTarget || undefined}>
			{children}
		</section>
	);
}

function CardItem({
	card,
	bootstrap,
	onOpen,
	onAssignee,
	onDueDate,
	manualOrder,
	sortableIndex,
	onMoveUp,
	onMoveDown,
	onRetry,
	labelMetadata
}: {
	card: BoardCard;
	bootstrap: Bootstrap;
	onOpen: () => void;
	onAssignee: (memberIds: number[]) => void;
	onDueDate: (dueDate: string | null) => void;
	manualOrder: boolean;
	sortableIndex: number;
	onMoveUp: (() => void) | undefined;
	onMoveDown: (() => void) | undefined;
	onRetry: () => void;
	labelMetadata: Map<string, ProjectLabel>;
}) {
	const { handleRef, isDragSource, ref } = useSortable({
		id: card.issueIid,
		index: sortableIndex,
		group: card.listKey,
		type: "card",
		accept: "card",
		disabled: !manualOrder
	});
	const team = bootstrap.teams.find((item) => item.key === card.teamKey);
	const title = team && !card.title.startsWith(team.titlePrefix) ? `${team.titlePrefix} ${card.title}` : card.title;
	const lists = [...bootstrap.board.lists].sort((a, b) => a.position - b.position);
	const overdue = Boolean(card.dueDate && card.dueDate < taipeiDateAfter(0) && !lists.find((list) => list.key === card.listKey)?.closed);
	const moveUpRef = useRef<HTMLButtonElement>(null);
	const moveDownRef = useRef<HTMLButtonElement>(null);
	const pendingOrderFocus = useRef<"up" | "down" | null>(null);
	const moveAndKeepFocus = (direction: "up" | "down") => {
		const move = direction === "up" ? onMoveUp : onMoveDown;
		if (!move) return;
		pendingOrderFocus.current = direction;
		move();
	};
	useEffect(() => {
		const direction = pendingOrderFocus.current;
		if (!direction) return;
		pendingOrderFocus.current = null;
		const preferred = direction === "up" ? moveUpRef.current : moveDownRef.current;
		const fallback = direction === "up" ? moveDownRef.current : moveUpRef.current;
		if (preferred && !preferred.disabled) preferred.focus();
		else if (fallback && !fallback.disabled) fallback.focus();
	}, [onMoveDown, onMoveUp]);
	return (
		<article ref={ref} className={styles.card} data-sync={card.syncState === "failed" ? "failed" : undefined} data-dragging={isDragSource || undefined}>
			<div className={styles.cardTopline}>
				<button
					ref={handleRef}
					type="button"
					className={styles.dragHandle}
					aria-label={`拖曳 ${title}`}
					title={manualOrder ? "拖曳調整卡片位置" : "切換至手動順序後拖曳"}
					disabled={!manualOrder}
				>
					<GripVertical size="0.9375rem" aria-hidden="true" />
				</button>
				<span>#{card.issueIid > 0 ? card.issueIid : "new"}</span>
				<div className={styles.cardOrderControls} data-visible={manualOrder} role="group" aria-label={`${title} 的手動順序`}>
					<button
						ref={moveUpRef}
						type="button"
						aria-label={`上移 ${title}`}
						title="上移卡片"
						disabled={!manualOrder || !onMoveUp}
						onClick={() => moveAndKeepFocus("up")}
					>
						<ArrowUp size="0.875rem" aria-hidden="true" />
					</button>
					<button
						ref={moveDownRef}
						type="button"
						aria-label={`下移 ${title}`}
						title="下移卡片"
						disabled={!manualOrder || !onMoveDown}
						onClick={() => moveAndKeepFocus("down")}
					>
						<ArrowDown size="0.875rem" aria-hidden="true" />
					</button>
				</div>
				{card.webUrl ? (
					<a href={card.webUrl} target="_blank" rel="noreferrer" aria-label={`在 GitLab 開啟 ${title}`} title="在 GitLab 開啟">
						<ExternalLink size="0.875rem" aria-hidden="true" />
					</a>
				) : null}
			</div>
			<button type="button" className={styles.cardTitle} onClick={onOpen}>
				<h3>{title}</h3>
				{card.description ? <p>{card.description}</p> : null}
			</button>
			<CardLabels card={card} bootstrap={bootstrap} labelMetadata={labelMetadata} title={title} />
			<footer className={styles.cardFooter}>
				<label className={styles.cardDate} data-overdue={overdue}>
					<span className={styles.srOnly}>期限</span>
					<input type="date" aria-label={`${title}的期限`} value={card.dueDate ?? ""} onChange={(event) => onDueDate(event.target.value || null)} />
				</label>
				<AssigneePicker
					bootstrap={bootstrap}
					teamKey={card.teamKey}
					value={card.assigneeGitLabUserIds}
					onChange={onAssignee}
					label={`變更 ${title} 的 Assignee`}
					compact
				/>
			</footer>
			{card.syncState === "failed" ? (
				<div className={styles.syncError} role="alert">
					<span>{card.syncError || "同步失敗"}</span>
					<button type="button" onClick={onRetry}>
						<RefreshCw size="0.8125rem" aria-hidden="true" /> 重試
					</button>
				</div>
			) : null}
		</article>
	);
}

function CardDragPreview({ card, bootstrap }: { card: BoardCard; bootstrap: Bootstrap }) {
	const team = bootstrap.teams.find((item) => item.key === card.teamKey);
	const title = team && !card.title.startsWith(team.titlePrefix) ? `${team.titlePrefix} ${card.title}` : card.title;
	return (
		<article className={`${styles.card} ${styles.dragPreview}`} aria-hidden="true">
			<div className={styles.cardTopline}>
				<GripVertical size="0.9375rem" aria-hidden="true" />
				<span>#{card.issueIid > 0 ? card.issueIid : "new"}</span>
			</div>
			<div className={styles.cardTitle}>
				<h3>{title}</h3>
				{card.description ? <p>{card.description}</p> : null}
			</div>
		</article>
	);
}

function CardDetail({
	card,
	bootstrap,
	onClose,
	onPrevious,
	onNext,
	position,
	total,
	onDetails,
	onTeam,
	onMove,
	onAssignee,
	onStartDate,
	onDueDate,
	onLabels,
	save
}: {
	card: BoardCard;
	bootstrap: Bootstrap;
	onClose: () => void;
	onPrevious: (() => void) | undefined;
	onNext: (() => void) | undefined;
	position: number;
	total: number;
	onDetails: (title: string, description: string) => void;
	onTeam: (teamKey: string) => void;
	onMove: (listKey: string) => void;
	onAssignee: (memberIds: number[]) => void;
	onStartDate: (startDate: string | null) => void;
	onDueDate: (dueDate: string | null) => void;
	onLabels: (labels: string[]) => void;
	save: FieldSaveState;
}) {
	const [title, setTitle] = useState(card.title);
	const [description, setDescription] = useState(card.description);
	const [descriptionMode, setDescriptionMode] = useState<"edit" | "preview">("edit");
	const teams = bootstrap.teams.filter((team) => team.active).sort((a, b) => a.sortOrder - b.sortOrder);
	const lists = [...bootstrap.board.lists].sort((a, b) => a.position - b.position);
	const submitDetails = (event: React.FormEvent) => {
		event.preventDefault();
		const normalized = title.trim();
		if (!normalized) return;
		onDetails(normalized, description);
	};
	const runQuickAction = (action: QuickAction) => {
		switch (action.kind) {
			case "assign":
				onAssignee(action.memberIds);
				break;
			case "due":
				onDueDate(action.value);
				break;
			case "start":
				onStartDate(action.value);
				break;
			case "move":
				onMove(action.listKey);
				break;
		}
	};

	return (
		<Drawer
			open
			onOpenChange={(open) => !open && onClose()}
			title={card.issueIid > 0 ? `#${card.issueIid} 卡片詳細資料` : "新卡片詳細資料"}
			description="細節與排程"
		>
			<form className={styles.cardDetail} onSubmit={submitDetails}>
				<nav className={styles.detailNavigation} aria-label="切換卡片">
					<button type="button" aria-label="上一張卡片" title="上一張卡片" disabled={!onPrevious} onClick={onPrevious}>
						<ChevronLeft size="1rem" aria-hidden="true" />
					</button>
					<span>
						{position} / {total}
					</span>
					<button type="button" aria-label="下一張卡片" title="下一張卡片" disabled={!onNext} onClick={onNext}>
						<ChevronRight size="1rem" aria-hidden="true" />
					</button>
				</nav>
				<p className={styles.srOnly} role="status" aria-live="polite">
					{save.announcement}
				</p>
				<label className={styles.detailTitle}>
					<span>標題</span>
					<input value={title} maxLength={255} onChange={(event) => setTitle(event.target.value)} />
				</label>
				<section className={styles.detailDescription}>
					<header className={styles.detailDescriptionHeader}>
						<span>描述</span>
						<SegmentedButton
							className={styles.descriptionModes}
							label="描述顯示模式"
							value={descriptionMode}
							onChange={setDescriptionMode}
							options={[
								{ value: "edit", label: "編輯" },
								{ value: "preview", label: "預覽" }
							]}
						/>
					</header>
					{descriptionMode === "edit" ? (
						<textarea
							aria-label="描述"
							value={description}
							onChange={(event) => setDescription(event.target.value)}
							placeholder="工作內容、驗收條件、相關連結..."
							rows={8}
						/>
					) : (
						<div className={styles.markdownPreview} aria-label="描述預覽">
							{description.trim() ? (
								<ReactMarkdown
									remarkPlugins={[remarkGfm]}
									components={{
										a: ({ href, children }) => (
											<a href={href} target="_blank" rel="noreferrer">
												{children}
											</a>
										)
									}}
								>
									{description}
								</ReactMarkdown>
							) : (
								<p className={styles.emptyPreview}>尚無描述</p>
							)}
						</div>
					)}
				</section>
				<div className={styles.detailGrid}>
					<label>
						<span>
							組別 <SaveIndicator save={save.get(card.issueIid, "team")} name="組別" />
						</span>
						<select aria-label="組別" value={card.teamKey} onChange={(event) => onTeam(event.target.value)}>
							{teams.map((team) => (
								<option key={team.key} value={team.key}>
									{team.name}
								</option>
							))}
						</select>
					</label>
					<label>
						<span>
							狀態 <SaveIndicator save={save.get(card.issueIid, "status")} name="狀態" />
						</span>
						<select aria-label="狀態" value={card.listKey} onChange={(event) => onMove(event.target.value)}>
							{lists.map((list) => (
								<option key={list.key} value={list.key}>
									{list.name}
								</option>
							))}
						</select>
					</label>
					<div className={styles.detailAssignees}>
						<span>
							Assignee <SaveIndicator save={save.get(card.issueIid, "assignee")} name="Assignee" />
						</span>
						<AssigneePicker bootstrap={bootstrap} teamKey={card.teamKey} value={card.assigneeGitLabUserIds} onChange={onAssignee} label="變更 Assignee" />
					</div>
					<div className={styles.detailDates}>
						<label>
							<span>
								Start <SaveIndicator save={save.get(card.issueIid, "startDate")} name="Start" />
							</span>
							<input type="date" aria-label="Start" value={card.startDate ?? ""} onChange={(event) => onStartDate(event.target.value || null)} />
						</label>
						<label>
							<span>
								Due <SaveIndicator save={save.get(card.issueIid, "dueDate")} name="Due" />
							</span>
							<input type="date" aria-label="Due" value={card.dueDate ?? ""} onChange={(event) => onDueDate(event.target.value || null)} />
						</label>
					</div>
				</div>
				<CardTags card={card} bootstrap={bootstrap} onChange={onLabels} save={save.get(card.issueIid, "labels")} />
				<QuickActionComposer bootstrap={bootstrap} card={card} onAction={runQuickAction} />
				<CardComments card={card} />
				<footer className={styles.detailActions}>
					{card.webUrl ? (
						<a href={card.webUrl} target="_blank" rel="noreferrer">
							<ExternalLink size="0.875rem" aria-hidden="true" /> GitLab Issue
						</a>
					) : (
						<span />
					)}
					<button type="submit" disabled={!title.trim()}>
						<Save size="0.875rem" aria-hidden="true" /> 儲存細節
					</button>
					<SaveIndicator save={save.get(card.issueIid, "details")} name="標題與描述" />
				</footer>
			</form>
		</Drawer>
	);
}

function CardTags({
	card,
	bootstrap,
	onChange,
	save
}: {
	card: BoardCard;
	bootstrap: Bootstrap;
	onChange: (labels: string[]) => void;
	save: FieldSave | undefined;
}) {
	const [query, setQuery] = useState("");
	const [managerOpen, setManagerOpen] = useState(false);
	const [managerSeed, setManagerSeed] = useState("");
	const picker = useRef<HTMLDetailsElement>(null);
	const labelsQuery = useProjectLabels();
	const teamLabels = new Set(bootstrap.teams.filter((team) => team.active).map((team) => team.gitLabLabel));
	const labelMetadata = new Map(labelsQuery.data?.map((label) => [label.name, label]));
	const normalizedQuery = query.trim().toLocaleLowerCase("zh-Hant");
	const available = (labelsQuery.data ?? []).filter(
		(label) =>
			!card.labels.includes(label.name) &&
			!isDeprecatedLabel(label.name) &&
			(!normalizedQuery || `${label.name} ${label.description ?? ""}`.toLocaleLowerCase("zh-Hant").includes(normalizedQuery))
	);
	const selectedTeamCount = card.labels.filter((label) => teamLabels.has(label)).length;

	const scope = (label: string) => {
		if (teamLabels.has(label)) return "team";
		return "general";
	};
	const add = (label: string) => {
		const nextScope = scope(label);
		const next = card.labels.filter((current) => nextScope === "general" || scope(current) !== nextScope);
		onChange([...next, label]);
		setQuery("");
		if (picker.current) picker.current.open = false;
	};
	const remove = (label: string) => {
		onChange(card.labels.filter((current) => current !== label));
	};
	// Dialogs open as siblings of the popover, never nested inside it.
	const openManager = (seed: string) => {
		setManagerSeed(seed);
		setManagerOpen(true);
		if (picker.current) picker.current.open = false;
	};

	return (
		<>
			<section className={styles.detailTags} aria-labelledby="card-labels-heading">
				<header>
					<h3 id="card-labels-heading">Labels</h3>
					<SaveIndicator save={save} name="Labels" />
					<details className={styles.tagPicker} ref={picker}>
						<summary aria-label="新增 Label" title="新增 Label">
							<Plus size="0.875rem" aria-hidden="true" /> 新增
						</summary>
						<div className={styles.tagMenu}>
							<input aria-label="搜尋 Label" value={query} onChange={(event) => setQuery(event.target.value)} />
							<div role="listbox" aria-label="可用 Labels">
								{available.map((label) => (
									<button key={label.name} type="button" role="option" aria-selected="false" onClick={() => add(label.name)}>
										<TagSwatch label={label} />
										<span>{label.name}</span>
									</button>
								))}
								{labelsQuery.isLoading ? <p role="status">載入中...</p> : null}
								{labelsQuery.isError ? (
									<button type="button" onClick={() => void labelsQuery.refetch()}>
										<RefreshCw size="0.8125rem" aria-hidden="true" /> 重新載入
									</button>
								) : null}
								{labelsQuery.isSuccess && available.length === 0 ? <p>沒有可用的 Label</p> : null}
							</div>
							<div className={styles.tagMenuFooter}>
								{normalizedQuery && !(labelsQuery.data ?? []).some((label) => label.name === query.trim()) ? (
									<button type="button" onClick={() => openManager(query.trim())}>
										<Plus size="0.8125rem" aria-hidden="true" /> 建立「{query.trim()}」
									</button>
								) : null}
								<button type="button" onClick={() => openManager("")}>
									<Settings size="0.8125rem" aria-hidden="true" /> 管理 Labels
								</button>
							</div>
						</div>
					</details>
				</header>
				<div className={styles.tagList}>
					{card.labels.map((label) => {
						const locked = scope(label) === "team" && selectedTeamCount <= 1;
						return (
							<StaticChip
								key={label}
								className={styles.tagChip}
								variant="input"
								label={label}
								leading={<TagSwatch label={labelMetadata.get(label)} />}
								trailing={
									<button
										type="button"
										className="md-chip__remove"
										aria-label={`移除 Label ${label}`}
										title={locked ? "Team Tag 必須保留一個" : `移除 ${label}`}
										disabled={locked}
										onClick={() => remove(label)}
									>
										<X size="0.75rem" aria-hidden="true" />
									</button>
								}
							/>
						);
					})}
					{card.labels.length === 0 ? <span className={styles.emptyTags}>尚無 Label</span> : null}
				</div>
			</section>
			<LabelManagerDialog
				open={managerOpen}
				onOpenChange={setManagerOpen}
				bootstrap={bootstrap}
				initialName={managerSeed}
				onCreated={(label) => {
					add(label.name);
					setManagerOpen(false);
				}}
			/>
		</>
	);
}

function CardComments({ card }: { card: BoardCard }) {
	const client = useQueryClient();
	const [body, setBody] = useState("");
	const queryKey = ["sitcon", "card-comments", card.issueIid] as const;
	const commentsQuery = useQuery({
		queryKey,
		queryFn: () => listComments(card),
		enabled: card.issueIid > 0
	});
	const commentMutation = useMutation({
		mutationFn: () => createComment(card, body),
		onSuccess: (comment) => {
			client.setQueryData<CardComment[]>(queryKey, (current) => [...(current ?? []), comment]);
			setBody("");
		}
	});
	const submit = () => {
		if (!body.trim() || commentMutation.isPending) return;
		commentMutation.mutate();
	};

	return (
		<section className={styles.comments} aria-labelledby="card-comments-heading">
			<header>
				<h3 id="card-comments-heading">Comment</h3>
				{commentsQuery.data ? <span>{commentsQuery.data.length}</span> : null}
			</header>
			{commentsQuery.isLoading ? <p className={styles.commentState}>載入中...</p> : null}
			{commentsQuery.isError ? (
				<div className={styles.commentError} role="alert">
					<span>{errorMessage(commentsQuery.error, "無法載入 Comment。")}</span>
					<button type="button" onClick={() => void commentsQuery.refetch()}>
						<RefreshCw size="0.8125rem" aria-hidden="true" /> 重試
					</button>
				</div>
			) : null}
			{commentsQuery.data?.length === 0 ? <p className={styles.commentState}>尚無 Comment</p> : null}
			<div className={styles.commentList}>
				{commentsQuery.data?.map((comment) => (
					<article className={styles.comment} data-system={comment.system || undefined} key={comment.id}>
						<Avatar name={comment.author.displayName} src={comment.author.avatarUrl} />
						<div>
							<header>
								<a href={comment.author.profileUrl} target="_blank" rel="noreferrer">
									{comment.author.displayName}
								</a>
								{comment.system ? <span>系統活動</span> : null}
								<time dateTime={comment.createdAt}>{formatDateTime(comment.createdAt)}</time>
							</header>
							<div className={styles.commentBody}>
								<MarkdownBody value={comment.body} />
							</div>
						</div>
					</article>
				))}
			</div>
			<div className={styles.commentComposer}>
				<textarea
					aria-label="Comment"
					rows={4}
					value={body}
					onChange={(event) => {
						setBody(event.target.value);
						if (commentMutation.isError) commentMutation.reset();
					}}
					onKeyDown={(event) => {
						if (event.key === "Enter" && (event.metaKey || event.ctrlKey)) {
							event.preventDefault();
							submit();
						}
					}}
				/>
				{commentMutation.isError ? <p role="alert">{errorMessage(commentMutation.error, "Comment 送出失敗，請重試。")}</p> : null}
				<button type="button" disabled={!body.trim() || commentMutation.isPending || card.issueIid <= 0} onClick={submit}>
					<Send size="0.875rem" aria-hidden="true" /> {commentMutation.isPending ? "送出中" : "送出 Comment"}
				</button>
			</div>
		</section>
	);
}

function MarkdownBody({ value }: { value: string }) {
	return (
		<ReactMarkdown
			remarkPlugins={[remarkGfm]}
			components={{
				a: ({ href, children }) => (
					<a href={href} target="_blank" rel="noreferrer">
						{children}
					</a>
				)
			}}
		>
			{value}
		</ReactMarkdown>
	);
}

function QuickActionComposer({ bootstrap, card, onAction }: { bootstrap: Bootstrap; card: BoardCard; onAction: (action: QuickAction) => void }) {
	const [value, setValue] = useState("");
	const [error, setError] = useState<string | null>(null);
	const [activeIndex, setActiveIndex] = useState(0);
	const inputId = useId();
	const menuId = useId();
	const commandToken = value.trimStart().split(/\s/, 1)[0]?.toLowerCase() ?? "";
	const suggestions =
		value.trimStart().startsWith("/") && !value.trimStart().includes(" ") ? quickActionCommands.filter((item) => item.command.startsWith(commandToken)) : [];

	const choose = (index: number) => {
		const suggestion = suggestions[index];
		if (!suggestion) return;
		const needsArgument = suggestion.usage !== suggestion.command;
		setValue(suggestion.command + (needsArgument ? " " : ""));
		setError(null);
		setActiveIndex(0);
	};
	const execute = () => {
		const result = parseQuickAction(value, bootstrap, card);
		if ("error" in result) {
			setError(result.error);
			return;
		}
		onAction(result.action);
		setValue("");
		setError(null);
	};

	return (
		<section className={styles.quickActions}>
			<label htmlFor={inputId}>
				<SquareTerminal size="0.875rem" aria-hidden="true" />
				<span>Quick action</span>
			</label>
			<div className={styles.commandInput}>
				<input
					id={inputId}
					role="combobox"
					aria-autocomplete="list"
					aria-expanded={suggestions.length > 0}
					aria-controls={suggestions.length ? menuId : undefined}
					aria-activedescendant={suggestions.length ? `${menuId}-${activeIndex}` : undefined}
					value={value}
					autoComplete="off"
					placeholder="/"
					onChange={(event) => {
						setValue(event.target.value);
						setError(null);
						setActiveIndex(0);
					}}
					onKeyDown={(event) => {
						if (event.key === "ArrowDown" && suggestions.length) {
							event.preventDefault();
							setActiveIndex((index) => (index + 1) % suggestions.length);
						} else if (event.key === "ArrowUp" && suggestions.length) {
							event.preventDefault();
							setActiveIndex((index) => (index - 1 + suggestions.length) % suggestions.length);
						} else if (event.key === "Enter") {
							event.preventDefault();
							if (suggestions.length && value.trim() !== suggestions[activeIndex]?.command) choose(activeIndex);
							else execute();
						} else if (event.key === "Escape" && suggestions.length) {
							event.stopPropagation();
							setValue("");
						}
					}}
				/>
				<button type="button" disabled={!value.trim()} onClick={execute}>
					執行
				</button>
				{suggestions.length ? (
					<div id={menuId} className={styles.commandMenu} role="listbox" aria-label="Quick Actions">
						{suggestions.map((suggestion, index) => (
							<button
								type="button"
								role="option"
								id={`${menuId}-${index}`}
								aria-selected={index === activeIndex}
								key={suggestion.command}
								onMouseDown={(event) => event.preventDefault()}
								onClick={() => choose(index)}
							>
								<code>{suggestion.usage}</code>
								<span>{suggestion.label}</span>
							</button>
						))}
					</div>
				) : null}
			</div>
			{error ? <p role="alert">{error}</p> : null}
		</section>
	);
}

function DirectoryConflict({ bootstrap, updateBootstrap }: Pick<BoardPageProps, "bootstrap" | "updateBootstrap">) {
	const current = bootstrap.preferences.defaultTeamKey;
	const directoryTeam = bootstrap.preferences.directoryTeamKeys.find((key) => key !== current);
	const conflictKey = directoryTeam && current ? `directory-conflict:${bootstrap.me.id}:${directoryTeam}:${current}` : "";
	const [dismissed, setDismissed] = useState(() => Boolean(conflictKey && sessionStorage.getItem(conflictKey)));
	const [saving, setSaving] = useState(false);
	if (!directoryTeam || !current || bootstrap.preferences.directoryTeamKeys.includes(current) || dismissed) return null;
	const directoryName = bootstrap.teams.find((team) => team.key === directoryTeam)?.name ?? directoryTeam;
	const currentName = bootstrap.teams.find((team) => team.key === current)?.name ?? current;

	const keep = () => {
		sessionStorage.setItem(conflictKey, "kept");
		setDismissed(true);
	};
	const switchToDirectory = async () => {
		setSaving(true);
		try {
			const response = await savePreferences(directoryTeam);
			updateBootstrap((state) => ({ ...state, preferences: response.preferences }));
		} finally {
			setSaving(false);
		}
	};

	return (
		<aside className={styles.conflict} aria-label="組別目錄有更新">
			<div>
				<strong>GitLab 目錄將你列為「{directoryName}」</strong>
				<span>目前的預設是「{currentName}」。</span>
			</div>
			<button type="button" disabled={saving} onClick={() => void switchToDirectory()}>
				<Check size="0.875rem" aria-hidden="true" /> {saving ? "更新中..." : `改用${directoryName}`}
			</button>
			<button type="button" onClick={keep}>
				保留{currentName}
			</button>
		</aside>
	);
}

function relativeAge(value: string) {
	const minutes = Math.max(1, Math.round((Date.now() - new Date(value).getTime()) / 60_000));
	if (minutes < 60) return `使用 ${minutes} 分鐘前資料`;
	const hours = Math.round(minutes / 60);
	if (hours < 24) return `使用 ${hours} 小時前資料`;
	return `使用 ${Math.round(hours / 24)} 天前資料`;
}

function formatDateTime(value: string) {
	return new Intl.DateTimeFormat("zh-TW", { dateStyle: "medium", timeStyle: "short", timeZone: "Asia/Taipei" }).format(new Date(value));
}
