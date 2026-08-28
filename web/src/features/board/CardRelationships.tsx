import { errorMessage } from "@/shared/api/client";
import { Avatar } from "@/shared/Avatar";
import { Badge, Button, ConfirmDialog, Dialog, IconButton, Menu, MenuItem, SelectField, Spinner, StaticChip, TextField } from "@project-template/ui";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ChevronDown, ChevronRight, ExternalLink, Link2, ListPlus, Plus, RefreshCw, X } from "lucide-react";
import { useDeferredValue, useEffect, useId, useMemo, useRef, useState } from "react";
import styles from "./CardRelationships.module.css";
import type { BoardCard, Bootstrap, LinkedWorkItem, WorkItemLinkType, WorkItemSummary } from "./model";
import {
	attachChildItem,
	createChildItem,
	createLinkedItems,
	deleteLinkedItem,
	detachChildItem,
	listChildItems,
	listLinkedItems,
	relationshipKeys,
	searchRelationshipCandidates
} from "./relationApi";

const linkGroups: { type: WorkItemLinkType; label: string }[] = [
	{ type: "is_blocked_by", label: "Is blocked by" },
	{ type: "blocks", label: "Blocks" },
	{ type: "relates_to", label: "Related to" }
];

type Removal = { kind: "child" | "linked"; item: WorkItemSummary };
type RelationPage = Awaited<ReturnType<typeof listChildItems>> | Awaited<ReturnType<typeof listLinkedItems>>;

export interface CardRelationshipsProps {
	card: BoardCard;
	bootstrap: Bootstrap;
	onOpenBoardCard: (issueIid: number) => void;
}

export function CardRelationships({ card, bootstrap, onOpenBoardCard }: CardRelationshipsProps) {
	const client = useQueryClient();
	const childContentID = useId();
	const linkedContentID = useId();
	const [childrenOpen, setChildrenOpen] = useState(true);
	const [linkedOpen, setLinkedOpen] = useState(true);
	const [childDialog, setChildDialog] = useState<"create" | "attach" | null>(null);
	const [linkedDialogQuery, setLinkedDialogQuery] = useState<string | null>(null);
	const [removal, setRemoval] = useState<Removal | null>(null);
	const childrenToggled = useRef(false);
	const linkedToggled = useRef(false);
	const childAddButton = useRef<HTMLButtonElement>(null);
	const linkedAddButton = useRef<HTMLButtonElement>(null);
	const enabled = card.issueIid > 0;
	const localIssueIIDs = useMemo(() => new Set(bootstrap.board.cards.map((item) => item.issueIid)), [bootstrap.board.cards]);
	const childrenQuery = useQuery({
		queryKey: relationshipKeys.children(card.issueIid),
		queryFn: () => listChildItems(card.issueIid),
		enabled
	});
	const linkedQuery = useQuery({
		queryKey: relationshipKeys.linked(card.issueIid),
		queryFn: () => listLinkedItems(card.issueIid),
		enabled
	});

	useEffect(() => {
		if (childrenQuery.isSuccess && childrenQuery.data.totalCount === 0 && !childrenToggled.current) setChildrenOpen(false);
	}, [childrenQuery.data?.totalCount, childrenQuery.isSuccess]);
	useEffect(() => {
		if (linkedQuery.isSuccess && linkedQuery.data.totalCount === 0 && !linkedToggled.current) setLinkedOpen(false);
	}, [linkedQuery.data?.totalCount, linkedQuery.isSuccess]);

	const invalidate = async (kind: "child" | "linked") => {
		await Promise.all([
			client.invalidateQueries({ queryKey: kind === "child" ? relationshipKeys.children(card.issueIid) : relationshipKeys.linked(card.issueIid) }),
			client.invalidateQueries({ queryKey: ["sitcon", "card-comments", card.issueIid] })
		]);
	};
	const removalMutation = useMutation({
		mutationFn: (value: Removal) =>
			value.kind === "child" ? detachChildItem(card.issueIid, value.item.gitLabWorkItemId) : deleteLinkedItem(card.issueIid, value.item.gitLabWorkItemId),
		onMutate: async (value) => {
			const key = value.kind === "child" ? relationshipKeys.children(card.issueIid) : relationshipKeys.linked(card.issueIid);
			await client.cancelQueries({ queryKey: key });
			const previous = client.getQueryData<RelationPage>(key);
			client.setQueryData<RelationPage>(key, (current) => {
				if (!current) return current;
				const items = current.items.filter((item) => item.gitLabWorkItemId !== value.item.gitLabWorkItemId);
				return { ...current, items, totalCount: Math.max(0, current.totalCount - (current.items.length - items.length)) } as RelationPage;
			});
			setRemoval(null);
			return { key, previous };
		},
		onError: (_error, _value, context) => {
			if (context?.previous) client.setQueryData(context.key, context.previous);
		},
		onSettled: (_data, _error, value) => {
			void invalidate(value.kind);
		}
	});
	const beginRemoval = (value: Removal) => {
		removalMutation.reset();
		setRemoval(value);
	};
	const openChildDialog = (mode: "create" | "attach") => {
		// Let the dropdown finish its close/focus cycle before the modal takes
		// focus. Opening both Radix layers in the same event causes focus churn.
		window.setTimeout(() => setChildDialog(mode), 0);
	};

	if (!enabled) {
		return (
			<section className={styles.root} aria-label="Work item relationships">
				<p className={styles.unsynced}>卡片同步完成後即可管理 Child items 與 Linked items。</p>
			</section>
		);
	}

	return (
		<section className={styles.root} aria-label="Work item relationships">
			<RelationshipSection
				title="Child items"
				count={childrenQuery.data?.totalCount}
				open={childrenOpen}
				contentID={childContentID}
				onToggle={() => {
					childrenToggled.current = true;
					setChildrenOpen((current) => !current);
				}}
				action={
					<Menu
						label="新增 Child item"
						align="end"
						trigger={<IconButton ref={childAddButton} label="新增 Child item" size="sm" icon={<Plus size="1rem" aria-hidden="true" />} />}
					>
						<MenuItem leading={<ListPlus size="1rem" aria-hidden="true" />} onSelect={() => openChildDialog("create")}>
							建立新 Task
						</MenuItem>
						<MenuItem leading={<Link2 size="1rem" aria-hidden="true" />} onSelect={() => openChildDialog("attach")}>
							加入既有 Task
						</MenuItem>
					</Menu>
				}
			>
				<QueryState query={childrenQuery} retry={() => void childrenQuery.refetch()} empty="尚無 Child item。">
					{childrenQuery.data?.items.map((item) => (
						<WorkItemRow
							key={item.gitLabWorkItemId}
							item={item}
							local={item.type === "issue" && localIssueIIDs.has(item.iid)}
							onOpenBoardCard={onOpenBoardCard}
							onRemove={() => beginRemoval({ kind: "child", item })}
							removeLabel={`解除 Child item #${item.iid}`}
						/>
					))}
				</QueryState>
			</RelationshipSection>

			<RelationshipSection
				title="Linked items"
				count={linkedQuery.data?.totalCount}
				open={linkedOpen}
				contentID={linkedContentID}
				onToggle={() => {
					linkedToggled.current = true;
					setLinkedOpen((current) => !current);
				}}
				action={
					<IconButton
						ref={linkedAddButton}
						label="新增 Linked item"
						size="sm"
						icon={<Plus size="1rem" aria-hidden="true" />}
						onClick={() => setLinkedDialogQuery("")}
					/>
				}
			>
				<QueryState query={linkedQuery} retry={() => void linkedQuery.refetch()} empty="尚無 Linked item。">
					{linkGroups.map((group) => {
						const items = linkedQuery.data?.items.filter((item) => item.linkType === group.type) ?? [];
						return items.length ? (
							<div className={styles.linkGroup} key={group.type}>
								<h4>{group.label}</h4>
								{items.map((item) => (
									<WorkItemRow
										key={item.gitLabWorkItemId}
										item={item}
										local={item.type === "issue" && localIssueIIDs.has(item.iid)}
										onOpenBoardCard={onOpenBoardCard}
										onRemove={() => beginRemoval({ kind: "linked", item })}
										removeLabel={`移除 Linked item #${item.iid}`}
									/>
								))}
							</div>
						) : null;
					})}
				</QueryState>
			</RelationshipSection>

			{childDialog !== null ? (
				<ChildDialog
					mode={childDialog}
					issueIid={card.issueIid}
					boardCards={bootstrap.board.cards}
					onOpenChange={(open) => {
						if (open) return;
						setChildDialog(null);
						restoreFocus(childAddButton);
					}}
					onLinkInstead={(query) => {
						setChildDialog(null);
						window.setTimeout(() => setLinkedDialogQuery(query), 0);
					}}
					onSaved={async () => {
						setChildDialog(null);
						await invalidate("child");
						restoreFocus(childAddButton);
					}}
				/>
			) : null}
			{linkedDialogQuery !== null ? (
				<LinkDialog
					initialQuery={linkedDialogQuery}
					issueIid={card.issueIid}
					onOpenChange={(open) => {
						if (!open) {
							setLinkedDialogQuery(null);
							restoreFocus(linkedAddButton);
						}
					}}
					onSaved={async () => {
						setLinkedDialogQuery(null);
						await invalidate("linked");
						restoreFocus(linkedAddButton);
					}}
				/>
			) : null}
			<ConfirmDialog
				open={removal !== null}
				onOpenChange={(open) => {
					if (!open) setRemoval(null);
				}}
				title={removal?.kind === "child" ? "解除 Child item？" : "移除 Linked item？"}
				description={
					removal?.kind === "child"
						? `只會解除 #${removal.item.iid} 的 parent，不會刪除 Task。`
						: `會移除與 #${removal?.item.iid ?? ""} 的雙向關聯，不會刪除 work item。`
				}
				confirmLabel="確認移除"
				cancelLabel="取消"
				destructive
				onConfirm={() => removal && removalMutation.mutate(removal)}
			/>
			{removalMutation.isError ? (
				<p className={styles.inlineError} role="alert">
					{errorMessage(removalMutation.error, "無法移除關聯，請重試。")}
				</p>
			) : null}
		</section>
	);
}

function RelationshipSection({
	title,
	count,
	open,
	contentID,
	onToggle,
	action,
	children
}: {
	title: string;
	count: number | undefined;
	open: boolean;
	contentID: string;
	onToggle: () => void;
	action: React.ReactNode;
	children: React.ReactNode;
}) {
	return (
		<section className={styles.section}>
			<header className={styles.sectionHeader}>
				<button type="button" className={styles.sectionToggle} aria-expanded={open} aria-controls={contentID} onClick={onToggle}>
					{open ? <ChevronDown size="1rem" aria-hidden="true" /> : <ChevronRight size="1rem" aria-hidden="true" />}
					<h3>{title}</h3>
					{count !== undefined ? <Badge tone="neutral">{count}</Badge> : null}
				</button>
				{action}
			</header>
			{open ? (
				<div className={styles.sectionContent} id={contentID}>
					{children}
				</div>
			) : null}
		</section>
	);
}

function QueryState({
	query,
	retry,
	empty,
	children
}: {
	query: { isLoading: boolean; isError: boolean; error: unknown; data: { items: unknown[] } | undefined };
	retry: () => void;
	empty: string;
	children: React.ReactNode;
}) {
	if (query.isLoading)
		return (
			<p className={styles.state} role="status">
				<Spinner size="sm" label="載入關聯" /> 載入中…
			</p>
		);
	if (query.isError)
		return (
			<div className={styles.error} role="alert">
				<span>{errorMessage(query.error, "無法載入 work item 關聯。")}</span>
				<Button variant="text" size="sm" leadingIcon={<RefreshCw size="0.875rem" aria-hidden="true" />} onClick={retry}>
					重試
				</Button>
			</div>
		);
	if (query.data?.items.length === 0) return <p className={styles.state}>{empty}</p>;
	return <div className={styles.items}>{children}</div>;
}

function WorkItemRow({
	item,
	local,
	onOpenBoardCard,
	onRemove,
	removeLabel
}: {
	item: WorkItemSummary | LinkedWorkItem;
	local: boolean;
	onOpenBoardCard: (issueIid: number) => void;
	onRemove: () => void;
	removeLabel: string;
}) {
	const title = (
		<>
			<span className={styles.type}>{item.type === "task" ? "Task" : "Issue"}</span>
			<span className={styles.title}>{item.title}</span>
			<span className={styles.iid}>#{item.iid}</span>
		</>
	);
	return (
		<article className={styles.item} data-state={item.state}>
			<div className={styles.itemTopline}>
				{local ? (
					<button type="button" className={styles.itemLink} onClick={() => onOpenBoardCard(item.iid)}>
						{title}
					</button>
				) : (
					<a className={styles.itemLink} href={item.webUrl} target="_blank" rel="noreferrer">
						{title} <ExternalLink size="0.75rem" aria-hidden="true" />
					</a>
				)}
				<IconButton label={removeLabel} tone="error" size="sm" icon={<X size="0.875rem" aria-hidden="true" />} onClick={onRemove} />
			</div>
			<div className={styles.metadata}>
				{item.status ? <StaticChip label={item.status.name} variant="suggestion" /> : <span>{item.state === "closed" ? "Closed" : "Open"}</span>}
				{item.assignees.length ? (
					<span className={styles.assignees} title={item.assignees.map((assignee) => assignee.displayName).join(", ")}>
						{item.assignees.slice(0, 3).map((assignee) => (
							<Avatar key={assignee.gitLabUserId} name={assignee.displayName} src={assignee.avatarUrl} size="sm" />
						))}
					</span>
				) : null}
				{item.startDate || item.dueDate ? <span>{[item.startDate, item.dueDate].filter(Boolean).join(" → ")}</span> : null}
			</div>
			{item.labels.length ? (
				<div className={styles.labels} aria-label="Labels">
					{item.labels.map((label) => (
						<StaticChip key={label.name} label={label.name} variant="suggestion" />
					))}
				</div>
			) : null}
		</article>
	);
}

function ChildDialog({
	mode,
	issueIid,
	boardCards,
	onOpenChange,
	onLinkInstead,
	onSaved
}: {
	mode: "create" | "attach";
	issueIid: number;
	boardCards: readonly BoardCard[];
	onOpenChange: (open: boolean) => void;
	onLinkInstead: (query: string) => void;
	onSaved: () => Promise<void>;
}) {
	const [title, setTitle] = useState("");
	const [query, setQuery] = useState("");
	const [selected, setSelected] = useState<number | null>(null);
	const debouncedQuery = useDebouncedValue(query, 250);
	const validSearch = isValidSearch(debouncedQuery);
	const matchingBoardIssue = useMemo(() => findMatchingBoardIssue(boardCards, debouncedQuery), [boardCards, debouncedQuery]);
	const candidates = useQuery({
		queryKey: relationshipKeys.candidates(issueIid, "child", debouncedQuery),
		queryFn: () => searchRelationshipCandidates(issueIid, "child", debouncedQuery),
		enabled: mode === "attach" && validSearch
	});
	const mutation = useMutation({
		mutationFn: async () => {
			if (mode === "create") {
				await createChildItem(issueIid, title.trim());
				return;
			}
			await attachChildItem(issueIid, selected ?? 0);
		},
		onSuccess: onSaved
	});
	return (
		<Dialog
			open
			onOpenChange={onOpenChange}
			title={mode === "create" ? "建立 Child Task" : "加入既有 Task"}
			description={mode === "create" ? "Task 會直接建立在此 Issue 之下。" : "只列出同專案、目前沒有 parent 的 Task。"}
			footer={
				<>
					<Button variant="text" onClick={() => onOpenChange(false)}>
						取消
					</Button>
					<Button loading={mutation.isPending} disabled={mode === "create" ? !title.trim() : selected === null} onClick={() => mutation.mutate()}>
						{mode === "create" ? "建立 Task" : "加入 Child item"}
					</Button>
				</>
			}
		>
			<div className={styles.dialogBody}>
				{mode === "create" ? (
					<TextField label="Task 標題" value={title} maxLength={255} autoFocus onChange={(event) => setTitle(event.target.value)} />
				) : (
					<>
						<TextField label="搜尋 Task 標題或 #IID" value={query} autoFocus onChange={(event) => setQuery(event.target.value)} />
						<CandidateList
							query={candidates}
							selected={selected === null ? [] : [selected]}
							onSelect={setSelected}
							prompt="輸入至少 2 個字元或 IID。"
							empty={
								matchingBoardIssue ? (
									<div className={styles.emptyCandidate}>
										<p>#{matchingBoardIssue.issueIid} 是 Issue，不能作為 Issue 的 Child item；Child item 只接受尚無 parent 的 Task。</p>
										<Button variant="text" size="sm" onClick={() => onLinkInstead(query.trim())}>
											改用 Linked item
										</Button>
									</div>
								) : (
									<p className={styles.state}>找不到尚無 parent 的 Task；Issue 請改用 Linked items。</p>
								)
							}
						/>
					</>
				)}
				{mutation.isError ? (
					<p className={styles.inlineError} role="alert">
						{errorMessage(mutation.error, "無法儲存 Child item。")}
					</p>
				) : null}
			</div>
		</Dialog>
	);
}

function LinkDialog({
	initialQuery,
	issueIid,
	onOpenChange,
	onSaved
}: {
	initialQuery: string;
	issueIid: number;
	onOpenChange: (open: boolean) => void;
	onSaved: () => Promise<void>;
}) {
	const [linkType, setLinkType] = useState<WorkItemLinkType>("relates_to");
	const [query, setQuery] = useState(initialQuery);
	const [selected, setSelected] = useState<number[]>([]);
	const debouncedQuery = useDebouncedValue(query, 250);
	const candidates = useQuery({
		queryKey: relationshipKeys.candidates(issueIid, "linked", debouncedQuery),
		queryFn: () => searchRelationshipCandidates(issueIid, "linked", debouncedQuery),
		enabled: isValidSearch(debouncedQuery)
	});
	const mutation = useMutation({
		mutationFn: () => createLinkedItems(issueIid, selected, linkType),
		onSuccess: onSaved
	});
	const toggleSelected = (workItemID: number) => {
		setSelected((current) => (current.includes(workItemID) ? current.filter((id) => id !== workItemID) : [...current, workItemID]));
	};
	return (
		<Dialog
			open
			onOpenChange={onOpenChange}
			title="新增 Linked item"
			description="可連結同專案的 Issue 或 Task。Blocks 類型視 GitLab tier 而定。"
			footer={
				<>
					<Button variant="text" onClick={() => onOpenChange(false)}>
						取消
					</Button>
					<Button loading={mutation.isPending} disabled={selected.length === 0} onClick={() => mutation.mutate()}>
						{selected.length > 0 ? `新增 ${selected.length} 個關聯` : "新增關聯"}
					</Button>
				</>
			}
		>
			<div className={styles.dialogBody}>
				<SelectField
					label="關聯類型"
					value={linkType}
					onValueChange={(value) => setLinkType(value as WorkItemLinkType)}
					options={linkGroups.map((group) => ({ value: group.type, label: group.label }))}
				/>
				<TextField label="搜尋 Issue、Task 或 #IID" value={query} autoFocus onChange={(event) => setQuery(event.target.value)} />
				<CandidateList query={candidates} selected={selected} onSelect={toggleSelected} prompt="輸入至少 2 個字元或 IID。" multiple />
				{selected.length > 0 ? (
					<p className={styles.selectionCount} role="status">
						已選擇 {selected.length} 個 work item
					</p>
				) : null}
				{mutation.isError ? (
					<p className={styles.inlineError} role="alert">
						{errorMessage(mutation.error, "無法新增 Linked item。")}
					</p>
				) : null}
			</div>
		</Dialog>
	);
}

function CandidateList({
	query,
	selected,
	onSelect,
	prompt,
	empty,
	multiple = false
}: {
	query: { isLoading: boolean; isError: boolean; error: unknown; data: WorkItemSummary[] | undefined };
	selected: readonly number[];
	onSelect: (id: number) => void;
	prompt: string;
	empty?: React.ReactNode;
	multiple?: boolean;
}) {
	if (query.isLoading) return <p className={styles.state}>搜尋中…</p>;
	if (query.isError)
		return (
			<p className={styles.inlineError} role="alert">
				{errorMessage(query.error, "無法搜尋 work item。")}
			</p>
		);
	if (!query.data) return <p className={styles.state}>{prompt}</p>;
	if (query.data.length === 0) return empty ?? <p className={styles.state}>找不到可加入的 work item。</p>;
	return (
		<div className={styles.candidates} role={multiple ? "group" : "radiogroup"} aria-label="Work item 搜尋結果">
			{query.data.map((item) => (
				<label key={item.gitLabWorkItemId} className={styles.candidate}>
					<input
						type={multiple ? "checkbox" : "radio"}
						name={multiple ? undefined : "relationship-candidate"}
						checked={selected.includes(item.gitLabWorkItemId)}
						onChange={() => onSelect(item.gitLabWorkItemId)}
					/>
					<span>
						<strong>{item.title}</strong>
						<small>
							{item.type === "task" ? "Task" : "Issue"} #{item.iid}
						</small>
					</span>
				</label>
			))}
		</div>
	);
}

function useDebouncedValue(value: string, delay: number) {
	const deferredValue = useDeferredValue(value);
	const [debounced, setDebounced] = useState(deferredValue);
	useEffect(() => {
		const timeout = window.setTimeout(() => setDebounced(deferredValue.trim()), delay);
		return () => window.clearTimeout(timeout);
	}, [deferredValue, delay]);
	return debounced;
}

function isValidSearch(value: string) {
	const normalized = value.trim();
	return /^#?[0-9]+$/.test(normalized) || Array.from(normalized).length >= 2;
}

function findMatchingBoardIssue(cards: readonly BoardCard[], value: string) {
	const normalized = value.trim().replace(/^#/, "").toLocaleLowerCase("zh-Hant");
	if (!normalized) return undefined;
	if (/^[0-9]+$/.test(normalized)) return cards.find((card) => String(card.issueIid) === normalized);
	return cards.find((card) => card.title.toLocaleLowerCase("zh-Hant").includes(normalized));
}

function restoreFocus(ref: React.RefObject<HTMLButtonElement | null>) {
	window.requestAnimationFrame(() => ref.current?.focus());
}
