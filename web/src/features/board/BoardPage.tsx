import { clearCsrfToken, errorMessage } from "@/shared/api/client";
import { Avatar } from "@/shared/Avatar";
import { Dialog, Drawer } from "@project-template/ui";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
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
	SquareTerminal,
	Users,
	X
} from "lucide-react";
import { useId, useRef, useState } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { AssigneePicker } from "./AssigneePicker";
import {
	createCard,
	createComment,
	listComments,
	listProjectLabels,
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
import styles from "./BoardPage.module.css";
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

export interface BoardPageProps {
	bootstrap: Bootstrap;
	updateBootstrap: (update: (current: Bootstrap) => Bootstrap) => void;
	backgroundOffline: boolean;
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
	startDate: string | null;
	dueDate: string | null;
};

export function BoardPage({ bootstrap, updateBootstrap, backgroundOffline }: BoardPageProps) {
	const [membersOpen, setMembersOpen] = useState(false);
	const [draggedIid, setDraggedIid] = useState<number | null>(null);
	const [detailIid, setDetailIid] = useState<number | null>(null);
	const [filterTeamKey, setFilterTeamKey] = useState("");
	const [filterMemberIds, setFilterMemberIds] = useState<number[]>([]);
	const [sortMode, setSortMode] = useState<BoardSortMode>("manual");
	const [undo, setUndo] = useState<{ cardIid: number; assigneeIds: number[]; assigneeNames: string[] } | null>(null);
	const localRetries = useRef(new Map<string, () => void>());
	const nextTemporaryIid = useRef(-1);
	const cards = bootstrap.board.cards;
	const filteredCards = cards.filter(
		(card) =>
			(!filterTeamKey || card.teamKey === filterTeamKey) &&
			(filterMemberIds.length === 0 || card.assigneeGitLabUserIds.some((id) => filterMemberIds.includes(id)))
	);
	const lists = [...bootstrap.board.lists].sort((a, b) => a.position - b.position);
	const orderedCards = lists.flatMap((list) => filteredCards.filter((card) => card.listKey === list.key).sort((a, b) => compareBoardCards(a, b, sortMode)));
	const detailCard = cards.find((card) => card.issueIid === detailIid) ?? null;
	const detailIndex = detailCard ? orderedCards.findIndex((card) => card.issueIid === detailCard.issueIid) : -1;
	const filtersActive = Boolean(filterTeamKey || filterMemberIds.length);

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

	const runCardMutation = (card: BoardCard, patch: CardPatch, request: (operationId: string) => ReturnType<typeof updateTeam>) => {
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
					replaceCard(card.issueIid, result.card);
				})
				.catch((cause: unknown) => {
					localRetries.current.set(operationId, execute);
					patchCard(card.issueIid, {
						...patch,
						syncState: "failed",
						syncError: errorMessage(cause, "變更尚未同步，請重試。"),
						pendingOperationId: operationId
					});
				});
		};
		localRetries.current.set(operationId, execute);
		execute();
	};

	const handleCreate = (input: CreateCardInput) => {
		const operationId = crypto.randomUUID();
		const temporaryIid = nextTemporaryIid.current;
		nextTemporaryIid.current -= 1;
		const teamLabel = bootstrap.teams.find((team) => team.key === input.teamKey)?.gitLabLabel;
		const listLabel = bootstrap.board.lists.find((list) => list.key === input.listKey)?.gitLabLabel;
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
			labels: [teamLabel, listLabel].filter((label): label is string => Boolean(label)),
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
					replaceCard(temporaryIid, result.card);
				})
				.catch((cause: unknown) => {
					localRetries.current.set(operationId, execute);
					patchCard(temporaryIid, {
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
		const labels = canonicalClientLabels(bootstrap, card.labels, teamKey, card.listKey);
		runCardMutation(card, { teamKey, labels, assigneeGitLabUserIds: nextAssigneeIDs }, (operationId) => updateTeam(card, operationId, teamKey));
	};

	const handleAssignee = (card: BoardCard, assigneeGitLabUserIds: number[]) => {
		runCardMutation(card, { assigneeGitLabUserIds }, (operationId) => updateAssignee(card, operationId, assigneeGitLabUserIds));
	};

	const handleDetails = (card: BoardCard, title: string, description: string) => {
		runCardMutation(card, { title, description }, (operationId) => updateDetails(card, operationId, title, description));
	};

	const handleDueDate = (card: BoardCard, dueDate: string | null) => {
		runCardMutation(card, { dueDate }, (operationId) => updateDueDate(card, operationId, dueDate));
	};

	const handleStartDate = (card: BoardCard, startDate: string | null) => {
		runCardMutation(card, { startDate }, (operationId) => updateStartDate(card, operationId, startDate));
	};

	const handleLabels = (card: BoardCard, labels: string[]) => {
		runCardMutation(card, { labels }, (operationId) => updateLabels(card, operationId, labels));
	};

	const handleMove = (card: BoardCard, listKey: string) => {
		if (card.listKey === listKey) return;
		const position = cards.filter((item) => item.listKey === listKey && item.issueIid !== card.issueIid).length;
		const labels = canonicalClientLabels(bootstrap, card.labels, card.teamKey, listKey);
		runCardMutation(card, { listKey, labels, position }, (operationId) => moveCard(card, operationId, listKey, position));
	};

	const handleRetry = (card: BoardCard) => {
		if (!card.pendingOperationId) return;
		const localRetry = localRetries.current.get(card.pendingOperationId);
		patchCard(card.issueIid, { syncState: "pending", syncError: null });
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
					sortMode={sortMode}
					visibleCount={filteredCards.length}
					totalCount={cards.length}
					onTeamChange={setFilterTeamKey}
					onMemberIdsChange={setFilterMemberIds}
					onSortModeChange={setSortMode}
					onClear={() => {
						setFilterTeamKey("");
						setFilterMemberIds([]);
					}}
				/>
				<section className={styles.board} aria-label="SITCON 2027 工作看板">
					{lists.map((list) => {
						const listCards = filteredCards.filter((card) => card.listKey === list.key).sort((a, b) => compareBoardCards(a, b, sortMode));
						return (
							<section
								className={styles.lane}
								data-list={list.key}
								key={list.key}
								onDragOver={(event) => event.preventDefault()}
								onDrop={() => {
									const card = cards.find((item) => item.issueIid === draggedIid);
									if (card) handleMove(card, list.key);
									setDraggedIid(null);
								}}
							>
								<header className={styles.laneHeader}>
									<h2>{list.name}</h2>
									<span>{listCards.length}</span>
								</header>
								<div className={styles.cardList}>
									{listCards.map((card) => (
										<CardItem
											key={card.issueIid}
											card={card}
											bootstrap={bootstrap}
											onDragStart={() => setDraggedIid(card.issueIid)}
											onOpen={() => setDetailIid(card.issueIid)}
											onAssignee={(memberIds) => handleAssignee(card, memberIds)}
											onDueDate={(dueDate) => handleDueDate(card, dueDate)}
											onRetry={() => handleRetry(card)}
										/>
									))}
									{listCards.length === 0 ? <p className={styles.emptyLane}>{filtersActive ? "沒有符合篩選的卡片" : "目前沒有卡片"}</p> : null}
								</div>
							</section>
						);
					})}
				</section>
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
	const [moreOpen, setMoreOpen] = useState(false);
	const [draftListKey, setDraftListKey] = useState(defaultList);
	const [draftDescription, setDraftDescription] = useState("");
	const [assignees, setAssignees] = useState<number[]>(preferredAssignees(bootstrap, defaultTeam));
	const [dueDate, setDueDate] = useState(taipeiDateAfter(7));
	const [clearedAssignees, setClearedAssignees] = useState<number[]>([]);
	const teams = bootstrap.teams.filter((team) => team.active).sort((a, b) => a.sortOrder - b.sortOrder);
	const leaderTargets = teams.map((team) => ({ team, leaders: teamLeaders(bootstrap, team.key) })).filter((target) => target.leaders.length > 0);
	const leaderCount = leaderTargets.reduce((count, target) => count + target.leaders.length, 0);
	const selectedListName = lists.find((list) => list.key === listKey)?.name ?? "Inbox";
	const moreActive = listKey !== defaultList || Boolean(description.trim());

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
		}
		setMoreOpen(next);
	};
	const applyMoreOptions = () => {
		setListKey(draftListKey);
		setDescription(draftDescription);
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
					startDate: null,
					dueDate: dueDate || null
				});
			}
		} else {
			if (!teamKey) return;
			onCreate({ title: normalized, description, teamKey, listKey, assigneeGitLabUserIds: assignees, startDate: null, dueDate: dueDate || null });
		}
		setTitle("");
		setDescription("");
		setDraftDescription("");
		setMoreOpen(false);
	};

	return (
		<form className={styles.quickCreate} onSubmit={submit}>
			<div className={styles.createModes} role="group" aria-label="開卡模式">
				<button type="button" aria-pressed={mode === "single"} onClick={() => setMode("single")}>
					單一組別
				</button>
				<button type="button" aria-pressed={mode === "leaders"} onClick={() => setMode("leaders")}>
					所有組長
				</button>
			</div>
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
						title={`更多建卡選項：${selectedListName}${description.trim() ? "，已填寫 Description" : ""}`}
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

function CardItem({
	card,
	bootstrap,
	onDragStart,
	onOpen,
	onAssignee,
	onDueDate,
	onRetry
}: {
	card: BoardCard;
	bootstrap: Bootstrap;
	onDragStart: () => void;
	onOpen: () => void;
	onAssignee: (memberIds: number[]) => void;
	onDueDate: (dueDate: string | null) => void;
	onRetry: () => void;
}) {
	const team = bootstrap.teams.find((item) => item.key === card.teamKey);
	const title = team && !card.title.startsWith(team.titlePrefix) ? `${team.titlePrefix} ${card.title}` : card.title;
	const lists = [...bootstrap.board.lists].sort((a, b) => a.position - b.position);
	const overdue = Boolean(card.dueDate && card.dueDate < taipeiDateAfter(0) && !lists.find((list) => list.key === card.listKey)?.closed);
	return (
		<article className={styles.card} data-sync={card.syncState === "failed" ? "failed" : undefined} draggable onDragStart={onDragStart}>
			<div className={styles.cardTopline}>
				<GripVertical size="0.9375rem" aria-hidden="true" />
				<span>#{card.issueIid > 0 ? card.issueIid : "new"}</span>
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
	onLabels
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
}) {
	const [title, setTitle] = useState(card.title);
	const [description, setDescription] = useState(card.description);
	const [descriptionMode, setDescriptionMode] = useState<"edit" | "preview">("edit");
	const teams = bootstrap.teams.filter((team) => team.active).sort((a, b) => a.sortOrder - b.sortOrder);
	const lists = [...bootstrap.board.lists].sort((a, b) => a.position - b.position);
	const save = (event: React.FormEvent) => {
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
			<form className={styles.cardDetail} onSubmit={save}>
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
				<label className={styles.detailTitle}>
					<span>標題</span>
					<input value={title} maxLength={255} onChange={(event) => setTitle(event.target.value)} />
				</label>
				<section className={styles.detailDescription}>
					<header className={styles.detailDescriptionHeader}>
						<span>描述</span>
						<div className={styles.descriptionModes} role="group" aria-label="描述顯示模式">
							<button type="button" aria-pressed={descriptionMode === "edit"} onClick={() => setDescriptionMode("edit")}>
								編輯
							</button>
							<button type="button" aria-pressed={descriptionMode === "preview"} onClick={() => setDescriptionMode("preview")}>
								預覽
							</button>
						</div>
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
						<span>組別</span>
						<select aria-label="組別" value={card.teamKey} onChange={(event) => onTeam(event.target.value)}>
							{teams.map((team) => (
								<option key={team.key} value={team.key}>
									{team.name}
								</option>
							))}
						</select>
					</label>
					<label>
						<span>狀態</span>
						<select aria-label="狀態" value={card.listKey} onChange={(event) => onMove(event.target.value)}>
							{lists.map((list) => (
								<option key={list.key} value={list.key}>
									{list.name}
								</option>
							))}
						</select>
					</label>
					<div className={styles.detailAssignees}>
						<span>Assignee</span>
						<AssigneePicker bootstrap={bootstrap} teamKey={card.teamKey} value={card.assigneeGitLabUserIds} onChange={onAssignee} label="變更 Assignee" />
					</div>
					<div className={styles.detailDates}>
						<label>
							<span>Start</span>
							<input type="date" value={card.startDate ?? ""} onChange={(event) => onStartDate(event.target.value || null)} />
						</label>
						<label>
							<span>Due</span>
							<input type="date" value={card.dueDate ?? ""} onChange={(event) => onDueDate(event.target.value || null)} />
						</label>
					</div>
				</div>
				<CardTags card={card} bootstrap={bootstrap} onChange={onLabels} />
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
				</footer>
			</form>
		</Drawer>
	);
}

const legacyStatusLabels = new Set(["Wating", "Waiting", "Inbox", "To Do", "Todo", "Doing", "Review", "Closed"]);

function canonicalClientLabels(bootstrap: Bootstrap, labels: string[], teamKey: string, listKey: string) {
	const teamLabels = new Set(bootstrap.teams.map((team) => team.gitLabLabel));
	const listLabels = new Set(bootstrap.board.lists.flatMap((list) => (list.gitLabLabel ? [list.gitLabLabel] : [])));
	const general = labels.filter((label) => !teamLabels.has(label) && !listLabels.has(label) && !legacyStatusLabels.has(label) && !label.startsWith("Status::"));
	const teamLabel = bootstrap.teams.find((team) => team.key === teamKey && team.active)?.gitLabLabel;
	const list = bootstrap.board.lists.find((item) => item.key === listKey);
	return [...general, ...(teamLabel ? [teamLabel] : []), ...(list?.gitLabLabel && !list.closed ? [list.gitLabLabel] : [])];
}

function CardTags({ card, bootstrap, onChange }: { card: BoardCard; bootstrap: Bootstrap; onChange: (labels: string[]) => void }) {
	const [query, setQuery] = useState("");
	const picker = useRef<HTMLDetailsElement>(null);
	const labelsQuery = useQuery({
		queryKey: ["sitcon", "project-labels"],
		queryFn: listProjectLabels,
		staleTime: 5 * 60_000
	});
	const teamLabels = new Set(bootstrap.teams.filter((team) => team.active).map((team) => team.gitLabLabel));
	const statusLabels = new Set(bootstrap.board.lists.flatMap((list) => (list.gitLabLabel ? [list.gitLabLabel] : [])));
	const labelMetadata = new Map(labelsQuery.data?.map((label) => [label.name, label]));
	const normalizedQuery = query.trim().toLocaleLowerCase("zh-Hant");
	const available = (labelsQuery.data ?? []).filter(
		(label) =>
			!card.labels.includes(label.name) &&
			(!normalizedQuery || `${label.name} ${label.description ?? ""}`.toLocaleLowerCase("zh-Hant").includes(normalizedQuery))
	);
	const selectedTeamCount = card.labels.filter((label) => teamLabels.has(label)).length;

	const scope = (label: string) => {
		if (teamLabels.has(label)) return "team";
		if (statusLabels.has(label) || legacyStatusLabels.has(label)) return "status";
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
		const currentScope = scope(label);
		let next = card.labels.filter((current) => current !== label);
		if (currentScope === "status" && !bootstrap.board.lists.find((list) => list.closed && list.key === card.listKey)) {
			const inbox = bootstrap.board.lists.find((list) => list.key === "inbox")?.gitLabLabel;
			if (inbox && !next.includes(inbox)) next = [...next.filter((current) => scope(current) !== "status"), inbox];
		}
		onChange(next);
	};

	return (
		<section className={styles.detailTags} aria-labelledby="card-tags-heading">
			<header>
				<h3 id="card-tags-heading">Tag</h3>
				<details className={styles.tagPicker} ref={picker}>
					<summary aria-label="新增 Tag" title="新增 Tag">
						<Plus size="0.875rem" aria-hidden="true" /> 新增
					</summary>
					<div className={styles.tagMenu}>
						<input aria-label="搜尋 Tag" value={query} onChange={(event) => setQuery(event.target.value)} />
						<div role="listbox" aria-label="可用 Tag">
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
							{labelsQuery.isSuccess && available.length === 0 ? <p>沒有可用的 Tag</p> : null}
						</div>
					</div>
				</details>
			</header>
			<div className={styles.tagList}>
				{card.labels.map((label) => {
					const locked = scope(label) === "team" && selectedTeamCount <= 1;
					return (
						<span className={styles.tagChip} key={label}>
							<TagSwatch label={labelMetadata.get(label)} />
							<span>{label}</span>
							<button
								type="button"
								aria-label={`移除 Tag ${label}`}
								title={locked ? "Team Tag 必須保留一個" : `移除 ${label}`}
								disabled={locked}
								onClick={() => remove(label)}
							>
								<X size="0.75rem" aria-hidden="true" />
							</button>
						</span>
					);
				})}
				{card.labels.length === 0 ? <span className={styles.emptyTags}>尚無 Tag</span> : null}
			</div>
		</section>
	);
}

function TagSwatch({ label }: { label: ProjectLabel | undefined }) {
	const color = label && /^#[0-9a-f]{6}$/i.test(label.color) ? label.color : undefined;
	return <span className={styles.tagSwatch} style={color ? ({ "--tag-color": color } as React.CSSProperties) : undefined} aria-hidden="true" />;
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
