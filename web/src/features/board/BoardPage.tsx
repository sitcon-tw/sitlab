import { clearCsrfToken, errorMessage } from "@/shared/api/client";
import { Avatar } from "@/shared/Avatar";
import { CalendarDays, Check, ChevronDown, Cloud, CloudOff, ExternalLink, GripVertical, LogOut, Plus, RefreshCw, Users, Wifi } from "lucide-react";
import { useRef, useState } from "react";
import { AssigneePicker } from "./AssigneePicker";
import { createCard, logout, moveCard, retryOperation, savePreferences, updateAssignee, updateDueDate, updateTeam } from "./boardApi";
import styles from "./BoardPage.module.css";
import { MembersDrawer } from "./MembersDrawer";
import { memberById, preferredAssignee, taipeiDateAfter, type BoardCard, type Bootstrap } from "./model";

export interface BoardPageProps {
	bootstrap: Bootstrap;
	updateBootstrap: (update: (current: Bootstrap) => Bootstrap) => void;
	backgroundOffline: boolean;
}

type CardPatch = Partial<Pick<BoardCard, "teamKey" | "assigneeGitLabUserId" | "dueDate" | "listKey" | "position">>;

export function BoardPage({ bootstrap, updateBootstrap, backgroundOffline }: BoardPageProps) {
	const [membersOpen, setMembersOpen] = useState(false);
	const [draggedIid, setDraggedIid] = useState<number | null>(null);
	const [undo, setUndo] = useState<{ cardIid: number; assigneeId: number; assigneeName: string } | null>(null);
	const localRetries = useRef(new Map<string, () => void>());
	const nextTemporaryIid = useRef(-1);
	const cards = bootstrap.board.cards;
	const lists = [...bootstrap.board.lists].sort((a, b) => a.position - b.position);

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

	const handleCreate = (input: { title: string; teamKey: string; assigneeGitLabUserId: number | null; dueDate: string | null }) => {
		const operationId = crypto.randomUUID();
		const temporaryIid = nextTemporaryIid.current;
		nextTemporaryIid.current -= 1;
		const optimistic: BoardCard = {
			issueIid: temporaryIid,
			issueId: null,
			title: input.title,
			webUrl: null,
			listKey: lists[0]?.key ?? "todo",
			position: cards.filter((card) => card.listKey === lists[0]?.key).length,
			teamKey: input.teamKey,
			assigneeGitLabUserId: input.assigneeGitLabUserId,
			dueDate: input.dueDate,
			labels: [],
			syncState: "pending",
			syncError: null,
			pendingOperationId: operationId,
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
		const currentAssignee = memberById(bootstrap, card.assigneeGitLabUserId);
		const nextAssignee = currentAssignee?.teamKeys.includes(teamKey) ? currentAssignee.gitLabUserId : null;
		if (currentAssignee && nextAssignee === null) {
			setUndo({ cardIid: card.issueIid, assigneeId: currentAssignee.gitLabUserId, assigneeName: currentAssignee.displayName });
		}
		runCardMutation(card, { teamKey, assigneeGitLabUserId: nextAssignee }, (operationId) => updateTeam(card, operationId, teamKey));
	};

	const handleAssignee = (card: BoardCard, assigneeGitLabUserId: number | null) => {
		runCardMutation(card, { assigneeGitLabUserId }, (operationId) => updateAssignee(card, operationId, assigneeGitLabUserId));
	};

	const handleDueDate = (card: BoardCard, dueDate: string | null) => {
		runCardMutation(card, { dueDate }, (operationId) => updateDueDate(card, operationId, dueDate));
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
		if (card) handleAssignee(card, undo.assigneeId);
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
						<span>已清除不屬於新組別的 Assignee：{undo.assigneeName}</span>
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
											onTeam={(teamKey) => handleTeam(card, teamKey)}
											onAssignee={(memberId) => handleAssignee(card, memberId)}
											onDueDate={(dueDate) => handleDueDate(card, dueDate)}
											onMove={(listKey) => handleMove(card, listKey)}
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
			<MembersDrawer bootstrap={bootstrap} open={membersOpen} onOpenChange={setMembersOpen} />
		</div>
	);
}

function BoardHeader({ bootstrap, backgroundOffline, onMembers }: { bootstrap: Bootstrap; backgroundOffline: boolean; onMembers: () => void }) {
	const offline = backgroundOffline || bootstrap.sync.state === "offline";
	const syncing = !offline && bootstrap.sync.state === "syncing";
	const syncLabel = offline ? `離線 · ${relativeAge(bootstrap.sync.lastSuccessAt)}` : syncing ? "同步中" : "已同步";
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
				<span className={styles.productMark}>S</span>
				<strong>SITCON</strong>
				<span>/ 2027</span>
			</div>
			<nav className={styles.headerActions} aria-label="看板工具">
				<button type="button" className={styles.headerButton} onClick={onMembers}>
					<Users size="1rem" aria-hidden="true" />
					<span>成員</span>
				</button>
				<span
					className={styles.sync}
					data-state={offline ? "offline" : syncing ? "syncing" : "synced"}
					title={bootstrap.sync.message ?? `最後同步：${formatDateTime(bootstrap.sync.lastSuccessAt)}`}
				>
					{offline ? (
						<CloudOff size="0.9375rem" aria-hidden="true" />
					) : syncing ? (
						<Cloud size="0.9375rem" aria-hidden="true" />
					) : (
						<Wifi size="0.9375rem" aria-hidden="true" />
					)}
					{syncLabel}
				</span>
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
	onCreate: (input: { title: string; teamKey: string; assigneeGitLabUserId: number | null; dueDate: string | null }) => void;
}) {
	const defaultTeam = bootstrap.preferences.defaultTeamKey ?? bootstrap.teams.find((team) => team.active)?.key ?? "";
	const [title, setTitle] = useState("");
	const [teamKey, setTeamKey] = useState(defaultTeam);
	const [assignee, setAssignee] = useState<number | null>(preferredAssignee(bootstrap, defaultTeam));
	const [dueDate, setDueDate] = useState(taipeiDateAfter(7));
	const [clearedAssignee, setClearedAssignee] = useState<number | null>(null);
	const teams = bootstrap.teams.filter((team) => team.active).sort((a, b) => a.sortOrder - b.sortOrder);

	const changeTeam = (nextTeam: string) => {
		const member = memberById(bootstrap, assignee);
		if (member && !member.teamKeys.includes(nextTeam)) {
			setClearedAssignee(member.gitLabUserId);
			setAssignee(null);
		}
		setTeamKey(nextTeam);
	};

	const submit = (event: React.FormEvent) => {
		event.preventDefault();
		const normalized = title.trim();
		if (!normalized || !teamKey) return;
		onCreate({ title: normalized, teamKey, assigneeGitLabUserId: assignee, dueDate: dueDate || null });
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
			<AssigneePicker bootstrap={bootstrap} teamKey={teamKey} value={assignee} onChange={setAssignee} label="選擇新卡片 Assignee" />
			<label className={styles.dateControl} title="新卡片期限">
				<CalendarDays size="0.9375rem" aria-hidden="true" />
				<span className={styles.srOnly}>期限</span>
				<input type="date" value={dueDate} aria-label="新卡片期限" onChange={(event) => setDueDate(event.target.value)} />
			</label>
			<button type="submit" className={styles.createButton} disabled={!title.trim()} aria-label="建立卡片" title="建立卡片">
				<Plus size="1.125rem" aria-hidden="true" />
			</button>
			{clearedAssignee ? (
				<p className={styles.quickNotice} role="status">
					已清除不屬於此組別的 Assignee
					<button
						type="button"
						onClick={() => {
							setAssignee(clearedAssignee);
							setClearedAssignee(null);
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
	onTeam,
	onAssignee,
	onDueDate,
	onMove,
	onRetry
}: {
	card: BoardCard;
	bootstrap: Bootstrap;
	onDragStart: () => void;
	onTeam: (teamKey: string) => void;
	onAssignee: (memberId: number | null) => void;
	onDueDate: (dueDate: string | null) => void;
	onMove: (listKey: string) => void;
	onRetry: () => void;
}) {
	const team = bootstrap.teams.find((item) => item.key === card.teamKey);
	const title = team && !card.title.startsWith(team.titlePrefix) ? `${team.titlePrefix} ${card.title}` : card.title;
	const teams = bootstrap.teams.filter((item) => item.active).sort((a, b) => a.sortOrder - b.sortOrder);
	const lists = [...bootstrap.board.lists].sort((a, b) => a.position - b.position);
	const overdue = Boolean(card.dueDate && card.dueDate < taipeiDateAfter(0) && !lists.find((list) => list.key === card.listKey)?.closed);
	return (
		<article className={styles.card} data-sync={card.syncState} draggable onDragStart={onDragStart}>
			<div className={styles.cardTopline}>
				<GripVertical size="0.9375rem" aria-hidden="true" />
				<span>#{card.issueIid > 0 ? card.issueIid : "new"}</span>
				{card.syncState === "pending" || card.syncState === "processing" ? <small>同步中</small> : null}
				{card.webUrl ? (
					<a href={card.webUrl} target="_blank" rel="noreferrer" aria-label={`在 GitLab 開啟 ${title}`} title="在 GitLab 開啟">
						<ExternalLink size="0.875rem" aria-hidden="true" />
					</a>
				) : null}
			</div>
			<h3>{title}</h3>
			<div className={styles.cardMeta}>
				<label>
					<span className={styles.srOnly}>組別</span>
					<select aria-label={`${title}的組別`} value={card.teamKey} onChange={(event) => onTeam(event.target.value)}>
						{teams.map((item) => (
							<option key={item.key} value={item.key}>
								{item.name}
							</option>
						))}
					</select>
				</label>
				<label className={styles.cardStatus}>
					<span className={styles.srOnly}>狀態</span>
					<select aria-label={`${title}的狀態`} value={card.listKey} onChange={(event) => onMove(event.target.value)}>
						{lists.map((list) => (
							<option key={list.key} value={list.key}>
								{list.name}
							</option>
						))}
					</select>
				</label>
			</div>
			<footer className={styles.cardFooter}>
				<label className={styles.cardDate} data-overdue={overdue}>
					<CalendarDays size="0.875rem" aria-hidden="true" />
					<span className={styles.srOnly}>期限</span>
					<input type="date" aria-label={`${title}的期限`} value={card.dueDate ?? ""} onChange={(event) => onDueDate(event.target.value || null)} />
				</label>
				<AssigneePicker
					bootstrap={bootstrap}
					teamKey={card.teamKey}
					value={card.assigneeGitLabUserId}
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
