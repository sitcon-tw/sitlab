import { Badge, Button, Dialog, EmptyState, IconButton, Menu, MenuItem, SelectField, TextField } from "@project-template/ui";
import { Check, ChevronDown, Settings, SlidersHorizontal, UsersRound, X } from "lucide-react";
import { useId, useMemo, useRef, useState, type ReactNode } from "react";
import styles from "./BoardPage.module.css";
import { GroupedMemberList } from "./GroupedMemberList";
import { LabelManagerDialog } from "./LabelManagerDialog";
import {
	activeMembers,
	filterDirectoryMembers,
	normalizePickerQuery,
	type BoardSortMode,
	type BoardViewMode,
	type Bootstrap,
	type DirectoryMember,
	type DirectoryTeam,
	type ProjectLabel
} from "./model";
import { TagSwatch } from "./TagSwatch";
import { TokenFilterInput } from "./TokenFilterInput";
import { useProjectLabels } from "./useProjectLabels";

export interface BoardFiltersProps {
	bootstrap: Bootstrap;
	query: string;
	teamKey: string;
	memberIds: number[];
	labels: string[];
	sortMode: BoardSortMode;
	viewMode: BoardViewMode;
	visibleCount: number;
	totalCount: number;
	onQueryChange: (query: string) => void;
	onTeamChange: (teamKey: string) => void;
	onMemberIdsChange: (memberIds: number[]) => void;
	onLabelsChange: (labels: string[]) => void;
	onSortModeChange: (sortMode: BoardSortMode) => void;
	onClear: () => void;
}

const SORT_OPTIONS: ReadonlyArray<{ value: BoardSortMode; label: string }> = [
	{ value: "manual", label: "手動順序" },
	{ value: "due-asc", label: "Due date：近到遠" },
	{ value: "due-desc", label: "Due date：遠到近" },
	{ value: "start-asc", label: "Start date：近到遠" },
	{ value: "start-desc", label: "Start date：遠到近" },
	{ value: "updated-desc", label: "Updated time：新到舊" },
	{ value: "updated-asc", label: "Updated time：舊到新" }
];

export function BoardFilters({
	bootstrap,
	query,
	teamKey,
	memberIds,
	labels,
	sortMode,
	viewMode,
	visibleCount,
	totalCount,
	onQueryChange,
	onTeamChange,
	onMemberIdsChange,
	onLabelsChange,
	onSortModeChange,
	onClear
}: BoardFiltersProps) {
	const [compactOpen, setCompactOpen] = useState(false);
	const [managerOpen, setManagerOpen] = useState(false);
	const labelsQuery = useProjectLabels();
	const teams = useMemo(() => bootstrap.teams.filter((team) => team.active).sort((a, b) => a.sortOrder - b.sortOrder), [bootstrap.teams]);
	const selectedMembers = useMemo(
		() => memberIds.flatMap((id) => bootstrap.members.find((candidate) => candidate.gitLabUserId === id) ?? []),
		[bootstrap.members, memberIds]
	);
	const filtersActive = Boolean(query.trim() || teamKey || memberIds.length || labels.length);
	const showSort = viewMode === "board";
	const advancedCount = Number(Boolean(teamKey)) + Number(memberIds.length > 0) + Number(labels.length > 0) + Number(showSort && sortMode !== "due-asc");
	const sortLabel = SORT_OPTIONS.find((option) => option.value === sortMode)?.label ?? "Due date：近到遠";

	return (
		<>
			<section className={styles.filters} aria-label={viewMode === "gantt" ? "篩選甘特圖" : "篩選看板"} data-view={viewMode}>
				{showSort ? (
					<Menu
						label="排序方式"
						className={styles.sortMenu}
						trigger={
							<button type="button" className={`${styles.sortControl} ${styles.desktopFilterControl}`} aria-label="排序方式" data-value={sortMode}>
								<span className={styles.sortText}>
									<span className={styles.sortLabel}>排序方式</span>
									<span className={styles.sortValue}>{sortLabel}</span>
								</span>
								<ChevronDown className={styles.sortChevron} size="1.25rem" aria-hidden="true" />
							</button>
						}
					>
						{SORT_OPTIONS.map((option) => (
							<MenuItem
								key={option.value}
								selected={sortMode === option.value}
								leading={sortMode === option.value ? <Check size="1.125rem" aria-hidden="true" /> : <span aria-hidden="true" />}
								onSelect={() => onSortModeChange(option.value)}
							>
								{option.label}
							</MenuItem>
						))}
					</Menu>
				) : null}

				<SearchablePicker
					className={`${styles.filterTeam} ${styles.desktopFilterControl}`}
					label="搜尋組別"
					summary={teams.find((team) => team.key === teamKey)?.name ?? "所有組別"}
					placeholder="輸入組別名稱"
				>
					{(pickerQuery, close) => <TeamOptions teams={teams} query={pickerQuery} value={teamKey} onChange={onTeamChange} close={close} />}
				</SearchablePicker>

				<SearchablePicker
					className={`${styles.filterPeople} ${styles.desktopFilterControl}`}
					label="搜尋負責人"
					summary={memberFilterLabel(selectedMembers)}
					placeholder="姓名或 GitLab 帳號"
				>
					{(pickerQuery) => <MemberOptions bootstrap={bootstrap} query={pickerQuery} value={memberIds} onChange={onMemberIdsChange} />}
				</SearchablePicker>

				<SearchablePicker
					className={`${styles.filterLabels} ${styles.desktopFilterControl}`}
					label="搜尋 Label"
					summary={labels.length ? `Labels ${labels.length}` : "所有 Labels"}
					placeholder="名稱或說明"
				>
					{(pickerQuery) => (
						<LabelOptions
							query={pickerQuery}
							value={labels}
							onChange={onLabelsChange}
							labels={labelsQuery.data ?? []}
							loading={labelsQuery.isLoading}
							error={labelsQuery.isError}
							onRetry={() => void labelsQuery.refetch()}
						/>
					)}
				</SearchablePicker>

				<IconButton
					className={`${styles.filterManageLabels} ${styles.desktopFilterControl}`}
					label="管理 Labels"
					title="管理 Labels"
					icon={<Settings size="1.25rem" aria-hidden="true" />}
					onClick={() => setManagerOpen(true)}
				/>

				<TokenFilterInput
					className={styles.filterSearch}
					bootstrap={bootstrap}
					query={query}
					teamKey={teamKey}
					memberIds={memberIds}
					labels={labels}
					projectLabels={labelsQuery.data ?? []}
					labelsLoading={labelsQuery.isLoading}
					onQueryChange={onQueryChange}
					onTeamChange={onTeamChange}
					onMemberIdsChange={onMemberIdsChange}
					onLabelsChange={onLabelsChange}
				/>

				<Button
					className={styles.compactFilterButton}
					variant="tonal"
					leadingIcon={<SlidersHorizontal size="1.125rem" aria-hidden="true" />}
					onClick={() => setCompactOpen(true)}
				>
					{showSort ? "篩選與排序" : "篩選"}
					{advancedCount ? ` (${advancedCount})` : ""}
				</Button>

				<span className={styles.filterResult} role="status" aria-live="polite">
					{visibleCount} / {totalCount} {viewMode === "gantt" ? "個開啟 Issue" : "張卡片"}
				</span>
				<IconButton
					className={styles.clearFilters}
					data-visible={filtersActive}
					disabled={!filtersActive}
					aria-hidden={!filtersActive}
					label="清除篩選"
					title="清除篩選"
					icon={<X size="1.25rem" aria-hidden="true" />}
					onClick={onClear}
				/>
			</section>

			<CompactFiltersDialog
				open={compactOpen}
				onOpenChange={setCompactOpen}
				bootstrap={bootstrap}
				teams={teams}
				teamKey={teamKey}
				memberIds={memberIds}
				labels={labels}
				sortMode={sortMode}
				showSort={showSort}
				projectLabels={labelsQuery.data ?? []}
				labelsLoading={labelsQuery.isLoading}
				labelsError={labelsQuery.isError}
				onRetryLabels={() => void labelsQuery.refetch()}
				onTeamChange={onTeamChange}
				onMemberIdsChange={onMemberIdsChange}
				onLabelsChange={onLabelsChange}
				onSortModeChange={onSortModeChange}
				onClearAdvanced={() => {
					onTeamChange("");
					onMemberIdsChange([]);
					onLabelsChange([]);
				}}
				onManageLabels={() => {
					setCompactOpen(false);
					setManagerOpen(true);
				}}
			/>
			<LabelManagerDialog open={managerOpen} onOpenChange={setManagerOpen} bootstrap={bootstrap} />
		</>
	);
}

function SearchablePicker({ className, label, summary, placeholder, children }: SearchablePickerProps) {
	const [open, setOpen] = useState(false);
	const [query, setQuery] = useState("");
	const rootRef = useRef<HTMLDivElement>(null);
	const panelId = useId();
	const close = () => {
		setOpen(false);
		setQuery("");
	};
	return (
		<div
			ref={rootRef}
			className={`${styles.searchableFilter} ${className}`}
			onBlur={(event) => {
				if (!event.currentTarget.contains(event.relatedTarget as Node | null)) {
					setOpen(false);
					setQuery("");
				}
			}}
			onKeyDown={(event) => {
				if (event.key !== "Escape" || !open) return;
				event.preventDefault();
				event.stopPropagation();
				close();
				rootRef.current?.querySelector<HTMLInputElement>("input")?.focus();
			}}
		>
			<TextField
				dense
				type="search"
				role="combobox"
				label={label}
				value={open ? query : summary}
				placeholder={placeholder}
				autoComplete="off"
				aria-autocomplete="list"
				aria-expanded={open}
				aria-controls={open ? panelId : undefined}
				onFocus={(event) => {
					setQuery("");
					setOpen(true);
					event.currentTarget.select();
				}}
				onChange={(event) => {
					setOpen(true);
					setQuery(event.target.value);
				}}
				onKeyDown={(event) => {
					if (event.key !== "ArrowDown" || !open) return;
					event.preventDefault();
					rootRef.current?.querySelector<HTMLElement>(".md-menu input, .md-menu button")?.focus();
				}}
			/>
			<ChevronDown className={styles.searchableFilterChevron} size="1.125rem" aria-hidden="true" />
			{open ? (
				<div id={panelId} className={`md-menu ${styles.searchableFilterMenu}`} aria-label={label}>
					{children(query, close)}
				</div>
			) : null}
		</div>
	);
}

interface SearchablePickerProps {
	className: string;
	label: string;
	summary: string;
	placeholder: string;
	children: (query: string, close: () => void) => ReactNode;
}

function TeamOptions({ teams, query, value, onChange, close }: TeamOptionsProps) {
	const normalized = normalizePickerQuery(query);
	const filtered = teams.filter((team) => !normalized || normalizePickerQuery(`${team.name} ${team.key} ${team.titlePrefix}`).includes(normalized));
	return (
		<div className={`md-list ${styles.searchableFilterList}`}>
			{!normalized ? <TeamOption label="所有組別" teamKey="" checked={value === ""} onChange={onChange} {...(close ? { close } : {})} /> : null}
			{filtered.map((team) => (
				<TeamOption key={team.key} label={team.name} teamKey={team.key} checked={value === team.key} onChange={onChange} {...(close ? { close } : {})} />
			))}
			{filtered.length === 0 ? <EmptyState title="找不到組別" description="請調整搜尋條件後再試一次。" /> : null}
		</div>
	);
}

interface TeamOptionsProps {
	teams: DirectoryTeam[];
	query: string;
	value: string;
	onChange: (teamKey: string) => void;
	close?: () => void;
}

function TeamOption({ label, teamKey, checked, onChange, close }: TeamOptionProps) {
	return (
		<label className={`md-list-item ${styles.teamFilterOption}`}>
			<span className="md-list-item__leading">
				<input
					type="radio"
					className="md-radio"
					name="board-team-filter"
					checked={checked}
					onChange={() => {
						onChange(teamKey);
						close?.();
					}}
				/>
			</span>
			<span className="md-list-item__text">
				<span className="md-list-item__headline">{label}</span>
			</span>
		</label>
	);
}

interface TeamOptionProps {
	label: string;
	teamKey: string;
	checked: boolean;
	onChange: (teamKey: string) => void;
	close?: () => void;
}

function MemberOptions({
	bootstrap,
	query,
	value,
	onChange
}: {
	bootstrap: Bootstrap;
	query: string;
	value: number[];
	onChange: (memberIds: number[]) => void;
}) {
	const members = useMemo(() => filterDirectoryMembers(activeMembers(bootstrap), query), [bootstrap, query]);
	const preferredTeamKey = bootstrap.preferences.defaultTeamKey ?? bootstrap.preferences.directoryTeamKeys[0] ?? "";
	return (
		<div className={`md-list ${styles.searchableFilterList}`}>
			{!query.trim() ? (
				<button
					type="button"
					className={`md-list-item md-list-item--two-line md-state-layer ${styles.memberOption}`}
					data-selected={value.length === 0}
					onClick={() => onChange([])}
				>
					<span className="md-list-item__leading">
						<UsersRound size="1.125rem" aria-hidden="true" />
					</span>
					<span className="md-list-item__text">
						<span className="md-list-item__headline">所有人</span>
						<span className="md-list-item__supporting">不限制負責人</span>
					</span>
					{value.length === 0 ? (
						<span className="md-list-item__trailing">
							<Check size="1.125rem" aria-hidden="true" />
						</span>
					) : null}
				</button>
			) : null}
			<GroupedMemberList
				teams={bootstrap.teams}
				members={members}
				value={value}
				currentUserId={bootstrap.me.gitLabUserId}
				onChange={onChange}
				preferredTeamKey={preferredTeamKey}
			/>
			{members.length === 0 ? <EmptyState title="找不到成員" description="請調整搜尋條件後再試一次。" /> : null}
		</div>
	);
}

function LabelOptions({ query, value, onChange, labels, loading, error, onRetry }: LabelOptionsProps) {
	const normalized = normalizePickerQuery(query);
	const filtered = labels.filter((label) => !normalized || normalizePickerQuery(`${label.name} ${label.description ?? ""}`).includes(normalized));
	const metadata = new Map(labels.map((label) => [label.name, label]));
	const toggle = (label: string) => onChange(value.includes(label) ? value.filter((current) => current !== label) : [...value, label]);
	return (
		<div className={`md-list ${styles.searchableFilterList}`}>
			{value.map((label) =>
				filtered.some((candidate) => candidate.name === label) ? null : (
					<LabelOption
						key={label}
						label={metadata.get(label) ?? { id: 0, name: label, color: "", textColor: "", description: null }}
						selected
						onToggle={toggle}
					/>
				)
			)}
			{filtered.map((label) => (
				<LabelOption key={label.name} label={label} selected={value.includes(label.name)} onToggle={toggle} />
			))}
			{loading ? <p role="status">載入中...</p> : null}
			{error ? (
				<EmptyState
					title="無法載入 Labels"
					description="請重新載入專案 Labels。"
					action={
						<Button variant="text" onClick={onRetry}>
							重新載入 Labels
						</Button>
					}
				/>
			) : null}
			{!loading && !error && filtered.length === 0 ? <EmptyState title="找不到 Label" description="請調整搜尋條件後再試一次。" /> : null}
		</div>
	);
}

interface LabelOptionsProps {
	query: string;
	value: string[];
	onChange: (labels: string[]) => void;
	labels: ProjectLabel[];
	loading: boolean;
	error: boolean;
	onRetry: () => void;
}

function LabelOption({ label, selected, onToggle }: { label: ProjectLabel; selected: boolean; onToggle: (label: string) => void }) {
	return (
		<label className={`md-list-item md-list-item--two-line ${styles.labelFilterOption}`}>
			<span className="md-list-item__leading">
				<input className="md-checkbox" type="checkbox" checked={selected} onChange={() => onToggle(label.name)} />
			</span>
			<TagSwatch label={label} />
			<span className="md-list-item__text">
				<span className="md-list-item__headline">{label.name}</span>
				{label.description ? <span className="md-list-item__supporting">{label.description}</span> : null}
			</span>
		</label>
	);
}

function CompactFiltersDialog(props: CompactFiltersDialogProps) {
	const {
		open,
		onOpenChange,
		bootstrap,
		teams,
		teamKey,
		memberIds,
		labels,
		sortMode,
		showSort,
		projectLabels,
		labelsLoading,
		labelsError,
		onRetryLabels,
		onTeamChange,
		onMemberIdsChange,
		onLabelsChange,
		onSortModeChange,
		onClearAdvanced,
		onManageLabels
	} = props;
	const [teamQuery, setTeamQuery] = useState("");
	const [memberQuery, setMemberQuery] = useState("");
	const [labelQuery, setLabelQuery] = useState("");
	const changeOpen = (next: boolean) => {
		onOpenChange(next);
		if (!next) {
			setTeamQuery("");
			setMemberQuery("");
			setLabelQuery("");
		}
	};
	const hasAdvancedFilters = Boolean(teamKey || memberIds.length || labels.length);
	return (
		<Dialog
			open={open}
			onOpenChange={changeOpen}
			title={showSort ? "篩選與排序" : "篩選甘特圖"}
			description={showSort ? "選取後立即更新看板" : "選取後立即更新甘特圖"}
			footer={
				<div className={styles.pickerFooter}>
					<Button variant="text" disabled={!hasAdvancedFilters} onClick={onClearAdvanced}>
						清除進階篩選
					</Button>
					<Button variant="text" onClick={() => changeOpen(false)}>
						完成
					</Button>
				</div>
			}
		>
			<div className={styles.compactFilters}>
				{showSort ? (
					<SelectField
						label="排序方式"
						value={sortMode}
						options={SORT_OPTIONS.map((option) => ({ value: option.value, label: option.label }))}
						onValueChange={(value) => onSortModeChange(value as BoardSortMode)}
					/>
				) : null}
				<CompactFilterSection title="組別" count={teamKey ? 1 : 0}>
					<TextField type="search" label="搜尋組別" value={teamQuery} onChange={(event) => setTeamQuery(event.target.value)} />
					<TeamOptions teams={teams} query={teamQuery} value={teamKey} onChange={onTeamChange} />
				</CompactFilterSection>
				<CompactFilterSection title="負責人" count={memberIds.length}>
					<TextField type="search" label="搜尋負責人" value={memberQuery} onChange={(event) => setMemberQuery(event.target.value)} />
					<MemberOptions bootstrap={bootstrap} query={memberQuery} value={memberIds} onChange={onMemberIdsChange} />
				</CompactFilterSection>
				<CompactFilterSection title="Labels" count={labels.length}>
					<div className={styles.compactLabelHeading}>
						<TextField type="search" label="搜尋 Label" value={labelQuery} onChange={(event) => setLabelQuery(event.target.value)} />
						<IconButton label="管理 Labels" title="管理 Labels" icon={<Settings size="1.125rem" aria-hidden="true" />} onClick={onManageLabels} />
					</div>
					<LabelOptions
						query={labelQuery}
						value={labels}
						onChange={onLabelsChange}
						labels={projectLabels}
						loading={labelsLoading}
						error={labelsError}
						onRetry={onRetryLabels}
					/>
				</CompactFilterSection>
			</div>
		</Dialog>
	);
}

interface CompactFiltersDialogProps {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	bootstrap: Bootstrap;
	teams: DirectoryTeam[];
	teamKey: string;
	memberIds: number[];
	labels: string[];
	sortMode: BoardSortMode;
	showSort: boolean;
	projectLabels: ProjectLabel[];
	labelsLoading: boolean;
	labelsError: boolean;
	onRetryLabels: () => void;
	onTeamChange: (teamKey: string) => void;
	onMemberIdsChange: (memberIds: number[]) => void;
	onLabelsChange: (labels: string[]) => void;
	onSortModeChange: (sortMode: BoardSortMode) => void;
	onClearAdvanced: () => void;
	onManageLabels: () => void;
}

function CompactFilterSection({ title, count, children }: { title: string; count: number; children: ReactNode }) {
	return (
		<section className={styles.compactFilterSection} aria-label={title}>
			<header>
				<h3 className="md-typescale-title-small">{title}</h3>
				{count ? <Badge tone="neutral">{count}</Badge> : null}
			</header>
			{children}
		</section>
	);
}

function memberFilterLabel(members: DirectoryMember[]) {
	if (!members.length) return "所有人";
	if (members.length <= 2) return members.map((member) => member.displayName).join("、");
	return `${members[0]?.displayName ?? "成員"}等 ${members.length} 人`;
}
