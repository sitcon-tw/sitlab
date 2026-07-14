import { clearCsrfToken, errorMessage } from "@/shared/api/client";
import { Avatar } from "@/shared/Avatar";
import { Dialog } from "@project-template/ui";
import { Check, ChevronDown, CloudOff, ExternalLink, GripVertical, LogOut, Plus, RefreshCw, Save, Users } from "lucide-react";
import { useRef, useState } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { AssigneePicker } from "./AssigneePicker";
import {
	createCard,
	logout,
	moveCard,
	retryOperation,
	savePreferences,
	updateAssignee,
	updateDetails,
	updateDueDate,
	updateStartDate,
	updateTeam
} from "./boardApi";
import styles from "./BoardPage.module.css";
import { MembersDrawer } from "./MembersDrawer";
import { memberById, preferredAssignees, taipeiDateAfter, type BoardCard, type Bootstrap } from "./model";

export interface BoardPageProps {
	bootstrap: Bootstrap;
	updateBootstrap: (update: (current: Bootstrap) => Bootstrap) => void;
	backgroundOffline: boolean;
}

type CardPatch = Partial<Pick<BoardCard, "title" | "description" | "teamKey" | "assigneeGitLabUserIds" | "startDate" | "dueDate" | "listKey" | "position">>;

export function BoardPage({ bootstrap, updateBootstrap, backgroundOffline }: BoardPageProps) {
	const [membersOpen, setMembersOpen] = useState(false);
	const [draggedIid, setDraggedIid] = useState<number | null>(null);
	const [detailIid, setDetailIid] = useState<number | null>(null);
	const [undo, setUndo] = useState<{ cardIid: number; assigneeIds: number[]; assigneeNames: string[] } | null>(null);
	const localRetries = useRef(new Map<string, () => void>());
	const nextTemporaryIid = useRef(-1);
	const cards = bootstrap.board.cards;
	const lists = [...bootstrap.board.lists].sort((a, b) => a.position - b.position);
	const detailCard = cards.find((card) => card.issueIid === detailIid) ?? null;

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

	const handleCreate = (input: {
		title: string;
		description: string;
		teamKey: string;
		assigneeGitLabUserIds: number[];
		startDate: string | null;
		dueDate: string | null;
	}) => {
		const operationId = crypto.randomUUID();
		const temporaryIid = nextTemporaryIid.current;
		nextTemporaryIid.current -= 1;
		const optimistic: BoardCard = {
			issueIid: temporaryIid,
			issueId: null,
			title: input.title,
			description: input.description,
			webUrl: null,
			listKey: lists[0]?.key ?? "todo",
			position: cards.filter((card) => card.listKey === lists[0]?.key).length,
			teamKey: input.teamKey,
			assigneeGitLabUserIds: input.assigneeGitLabUserIds,
			startDate: input.startDate,
			dueDate: input.dueDate,
			labels: [],
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
		runCardMutation(card, { teamKey, assigneeGitLabUserIds: nextAssigneeIDs }, (operationId) => updateTeam(card, operationId, teamKey));
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

	const handleMove = (card: BoardCard, listKey: string) => {
		if (card.listKey === listKey) return;
		const position = cards.filter((item) => item.listKey === listKey && item.issueIid !== card.issueIid).length;
		runCardMutation(card, { listKey, position }, (operationId) => moveCard(card, operationId, listKey, position));
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
				<section className={styles.board} aria-label="SITCON 2027 工作看板">
					{lists.map((list) => {
						const listCards = cards.filter((card) => card.listKey === list.key).sort((a, b) => a.position - b.position);
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
									{listCards.length === 0 ? <p className={styles.emptyLane}>目前沒有卡片</p> : null}
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
					onDetails={(title, description) => handleDetails(detailCard, title, description)}
					onTeam={(teamKey) => handleTeam(detailCard, teamKey)}
					onMove={(listKey) => handleMove(detailCard, listKey)}
					onAssignee={(memberIds) => handleAssignee(detailCard, memberIds)}
					onStartDate={(startDate) => handleStartDate(detailCard, startDate)}
					onDueDate={(dueDate) => handleDueDate(detailCard, dueDate)}
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

function QuickCreate({
	bootstrap,
	onCreate
}: {
	bootstrap: Bootstrap;
	onCreate: (input: {
		title: string;
		description: string;
		teamKey: string;
		assigneeGitLabUserIds: number[];
		startDate: string | null;
		dueDate: string | null;
	}) => void;
}) {
	const defaultTeam = bootstrap.preferences.defaultTeamKey ?? bootstrap.teams.find((team) => team.active)?.key ?? "";
	const [title, setTitle] = useState("");
	const [teamKey, setTeamKey] = useState(defaultTeam);
	const [assignees, setAssignees] = useState<number[]>(preferredAssignees(bootstrap, defaultTeam));
	const [dueDate, setDueDate] = useState(taipeiDateAfter(7));
	const [clearedAssignees, setClearedAssignees] = useState<number[]>([]);
	const teams = bootstrap.teams.filter((team) => team.active).sort((a, b) => a.sortOrder - b.sortOrder);

	const changeTeam = (nextTeam: string) => {
		const compatible = assignees.filter((id) => memberById(bootstrap, id)?.teamKeys.includes(nextTeam));
		setClearedAssignees(assignees.filter((id) => !compatible.includes(id)));
		setAssignees(compatible);
		setTeamKey(nextTeam);
	};

	const submit = (event: React.FormEvent) => {
		event.preventDefault();
		const normalized = title.trim();
		if (!normalized || !teamKey) return;
		onCreate({ title: normalized, description: "", teamKey, assigneeGitLabUserIds: assignees, startDate: null, dueDate: dueDate || null });
		setTitle("");
	};

	return (
		<form className={styles.quickCreate} onSubmit={submit}>
			<label className={styles.srOnly} htmlFor="quick-team">
				新卡片組別
			</label>
			<select id="quick-team" value={teamKey} onChange={(event) => changeTeam(event.target.value)}>
				{teams.map((team) => (
					<option key={team.key} value={team.key}>
						{team.name}
					</option>
				))}
			</select>
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
			<AssigneePicker bootstrap={bootstrap} teamKey={teamKey} value={assignees} onChange={setAssignees} label="選擇新卡片 Assignee" />
			<label className={styles.dateControl} title="新卡片期限">
				<span className={styles.srOnly}>期限</span>
				<input type="date" value={dueDate} aria-label="新卡片期限" onChange={(event) => setDueDate(event.target.value)} />
			</label>
			<button type="submit" className={styles.createButton} disabled={!title.trim()} aria-label="建立卡片" title="建立卡片">
				<Plus size="1.125rem" aria-hidden="true" />
			</button>
			{clearedAssignees.length ? (
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
	onDetails,
	onTeam,
	onMove,
	onAssignee,
	onStartDate,
	onDueDate
}: {
	card: BoardCard;
	bootstrap: Bootstrap;
	onClose: () => void;
	onDetails: (title: string, description: string) => void;
	onTeam: (teamKey: string) => void;
	onMove: (listKey: string) => void;
	onAssignee: (memberIds: number[]) => void;
	onStartDate: (startDate: string | null) => void;
	onDueDate: (dueDate: string | null) => void;
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
		onClose();
	};

	return (
		<Dialog
			open
			onOpenChange={(open) => !open && onClose()}
			title={card.issueIid > 0 ? `#${card.issueIid} 卡片詳細資料` : "新卡片詳細資料"}
			description="細節與排程"
		>
			<form className={styles.cardDetail} onSubmit={save}>
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
						<select value={card.teamKey} onChange={(event) => onTeam(event.target.value)}>
							{teams.map((team) => (
								<option key={team.key} value={team.key}>
									{team.name}
								</option>
							))}
						</select>
					</label>
					<label>
						<span>狀態</span>
						<select value={card.listKey} onChange={(event) => onMove(event.target.value)}>
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
		</Dialog>
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
