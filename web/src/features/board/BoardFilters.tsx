import { Chip, Dialog, IconButton, Menu, MenuItem, SelectField } from "@project-template/ui";
import { Check, ChevronDown, Search, Settings, UsersRound, X } from "lucide-react";
import { useMemo, useState } from "react";
import styles from "./BoardPage.module.css";
import { GroupedMemberList } from "./GroupedMemberList";
import { LabelManagerDialog } from "./LabelManagerDialog";
import { activeMembers, filterDirectoryMembers, type BoardSortMode, type Bootstrap, type DirectoryMember, type ProjectLabel } from "./model";
import { TagSwatch } from "./TagSwatch";
import { useProjectLabels } from "./useProjectLabels";

export interface BoardFiltersProps {
	bootstrap: Bootstrap;
	teamKey: string;
	memberIds: number[];
	labels: string[];
	sortMode: BoardSortMode;
	visibleCount: number;
	totalCount: number;
	onTeamChange: (teamKey: string) => void;
	onMemberIdsChange: (memberIds: number[]) => void;
	onLabelsChange: (labels: string[]) => void;
	onSortModeChange: (sortMode: BoardSortMode) => void;
	onClear: () => void;
}

export function BoardFilters({
	bootstrap,
	teamKey,
	memberIds,
	labels,
	sortMode,
	visibleCount,
	totalCount,
	onTeamChange,
	onMemberIdsChange,
	onLabelsChange,
	onSortModeChange,
	onClear
}: BoardFiltersProps) {
	const teams = bootstrap.teams.filter((team) => team.active).sort((a, b) => a.sortOrder - b.sortOrder);
	const active = Boolean(teamKey || memberIds.length || labels.length);

	return (
		<section className={styles.filters} aria-label="篩選看板">
			<SelectField
				dense
				className={styles.sortControl}
				label="排序方式"
				value={sortMode}
				onChange={(event) => onSortModeChange(event.target.value as BoardSortMode)}
				options={[
					{ value: "manual", label: "手動順序" },
					{ value: "due-asc", label: "Due date：近到遠" },
					{ value: "due-desc", label: "Due date：遠到近" },
					{ value: "start-asc", label: "Start date：近到遠" },
					{ value: "start-desc", label: "Start date：遠到近" },
					{ value: "updated-desc", label: "Updated time：新到舊" },
					{ value: "updated-asc", label: "Updated time：舊到新" }
				]}
			/>
			<Menu
				label="篩選組別"
				trigger={
					<Chip
						variant="filter"
						selected={Boolean(teamKey)}
						label={teams.find((team) => team.key === teamKey)?.name ?? "所有組別"}
						aria-label="篩選組別"
						trailing={<ChevronDown size="1.125rem" aria-hidden="true" />}
					/>
				}
			>
				<MenuItem selected={teamKey === ""} onSelect={() => onTeamChange("")}>
					所有組別
				</MenuItem>
				{teams.map((team) => (
					<MenuItem key={team.key} selected={teamKey === team.key} onSelect={() => onTeamChange(team.key)}>
						{team.name}
					</MenuItem>
				))}
			</Menu>
			<MemberFilter bootstrap={bootstrap} value={memberIds} onChange={onMemberIdsChange} />
			<LabelFilter value={labels} onChange={onLabelsChange} bootstrap={bootstrap} />
			<span className={styles.filterResult} role="status" aria-live="polite">
				{visibleCount} / {totalCount} 張卡片
			</span>
			<IconButton
				className={styles.clearFilters}
				data-visible={active}
				disabled={!active}
				aria-hidden={!active}
				label="清除篩選"
				title="清除篩選"
				icon={<X size="1.25rem" aria-hidden="true" />}
				onClick={onClear}
			/>
		</section>
	);
}

function LabelFilter({ value, onChange, bootstrap }: { value: string[]; onChange: (labels: string[]) => void; bootstrap: Bootstrap }) {
	const [open, setOpen] = useState(false);
	const [managerOpen, setManagerOpen] = useState(false);
	const [query, setQuery] = useState("");
	const labelsQuery = useProjectLabels();
	const normalizedQuery = query.trim().toLocaleLowerCase("zh-Hant");
	const labels = (labelsQuery.data ?? []).filter(
		(label) => !normalizedQuery || `${label.name} ${label.description ?? ""}`.toLocaleLowerCase("zh-Hant").includes(normalizedQuery)
	);
	const metadata = new Map(labelsQuery.data?.map((label) => [label.name, label]));
	const changeOpen = (next: boolean) => {
		setOpen(next);
		if (!next) setQuery("");
	};
	const toggle = (label: string) => onChange(value.includes(label) ? value.filter((current) => current !== label) : [...value, label]);

	return (
		<>
			<Chip
				className={styles.filterLabels}
				variant="filter"
				selected={value.length > 0}
				label={value.length ? `Labels ${value.length}` : "所有 Labels"}
				aria-label="篩選 Label"
				title={value.length ? value.join("、") : "所有 Labels"}
				trailing={<ChevronDown size="1.125rem" aria-hidden="true" />}
				onClick={() => setOpen(true)}
			/>
			<IconButton
				className={styles.filterManageLabels}
				label="管理 Labels"
				title="管理 Labels"
				icon={<Settings size="1.25rem" aria-hidden="true" />}
				onClick={() => setManagerOpen(true)}
			/>
			<LabelManagerDialog open={managerOpen} onOpenChange={setManagerOpen} bootstrap={bootstrap} />
			<Dialog open={open} onOpenChange={changeOpen} title="篩選 Label" description="卡片必須包含所有選取的 Labels">
				<div className={styles.pickerSearch}>
					<Search size="1rem" aria-hidden="true" />
					<input
						autoFocus
						type="search"
						value={query}
						onChange={(event) => setQuery(event.target.value)}
						placeholder="搜尋 Label 名稱或說明"
						aria-label="搜尋篩選 Label"
					/>
					{query ? (
						<button type="button" aria-label="清除搜尋" onClick={() => setQuery("")}>
							<X size="0.875rem" aria-hidden="true" />
						</button>
					) : null}
				</div>
				<div className={styles.pickerList}>
					{value.map((label) =>
						labels.some((candidate) => candidate.name === label) ? null : (
							<LabelOption
								key={label}
								label={metadata.get(label) ?? { id: 0, name: label, color: "", textColor: "", description: null }}
								selected
								onToggle={toggle}
							/>
						)
					)}
					{labels.map((label) => (
						<LabelOption key={label.name} label={label} selected={value.includes(label.name)} onToggle={toggle} />
					))}
					{labelsQuery.isLoading ? <p role="status">載入中...</p> : null}
					{labelsQuery.isError ? (
						<button type="button" className={styles.memberOption} onClick={() => void labelsQuery.refetch()}>
							重新載入 Labels
						</button>
					) : null}
					{labelsQuery.isSuccess && labels.length === 0 ? <p className={styles.noResults}>找不到符合的 Label</p> : null}
				</div>
				<div className={styles.pickerFooter}>
					<button type="button" disabled={value.length === 0} onClick={() => onChange([])}>
						清除
					</button>
					<span>{value.length ? `已選擇 ${value.length} 個` : "所有 Labels"}</span>
					<button type="button" onClick={() => changeOpen(false)}>
						完成
					</button>
				</div>
			</Dialog>
		</>
	);
}

function LabelOption({ label, selected, onToggle }: { label: ProjectLabel; selected: boolean; onToggle: (label: string) => void }) {
	return (
		<label className={styles.labelFilterOption}>
			<input type="checkbox" checked={selected} onChange={() => onToggle(label.name)} />
			<TagSwatch label={label} />
			<span>
				<strong>{label.name}</strong>
				{label.description ? <small>{label.description}</small> : null}
			</span>
		</label>
	);
}

function MemberFilter({ bootstrap, value, onChange }: { bootstrap: Bootstrap; value: number[]; onChange: (memberIds: number[]) => void }) {
	const [open, setOpen] = useState(false);
	const [query, setQuery] = useState("");
	const selected = value.flatMap((id) => {
		const member = bootstrap.members.find((candidate) => candidate.gitLabUserId === id);
		return member ? [member] : [];
	});
	const members = useMemo(() => filterDirectoryMembers(activeMembers(bootstrap), query), [bootstrap, query]);
	const preferredTeamKey = bootstrap.preferences.defaultTeamKey ?? bootstrap.preferences.directoryTeamKeys[0] ?? "";

	const changeOpen = (next: boolean) => {
		setOpen(next);
		if (!next) setQuery("");
	};
	return (
		<>
			<Chip
				className={styles.filterPeople}
				variant="filter"
				selected={selected.length > 0}
				label={memberFilterLabel(selected)}
				aria-label="篩選負責人"
				title={selected.length ? selected.map((member) => member.displayName).join("、") : "所有人"}
				leading={<UsersRound size="1.125rem" aria-hidden="true" />}
				trailing={<ChevronDown size="1.125rem" aria-hidden="true" />}
				onClick={() => setOpen(true)}
			/>
			<Dialog open={open} onOpenChange={changeOpen} title="篩選負責人" description="可複選專案成員">
				<div className={styles.pickerSearch}>
					<Search size="1rem" aria-hidden="true" />
					<input
						autoFocus
						type="search"
						value={query}
						onChange={(event) => setQuery(event.target.value)}
						placeholder="搜尋姓名或 GitLab 帳號"
						aria-label="搜尋篩選成員"
					/>
					{query ? (
						<button type="button" aria-label="清除搜尋" onClick={() => setQuery("")}>
							<X size="0.875rem" aria-hidden="true" />
						</button>
					) : null}
				</div>
				<div className={styles.pickerList}>
					<button type="button" className={styles.memberOption} data-selected={value.length === 0} onClick={() => onChange([])}>
						<span className={styles.unassignedAvatar}>
							<UsersRound size="1rem" aria-hidden="true" />
						</span>
						<span>
							<strong>所有人</strong>
							<small>不限制負責人</small>
						</span>
						{value.length === 0 ? <Check size="1rem" aria-hidden="true" /> : null}
					</button>
					<GroupedMemberList
						teams={bootstrap.teams}
						members={members}
						value={value}
						currentUserId={bootstrap.me.gitLabUserId}
						onChange={onChange}
						preferredTeamKey={preferredTeamKey}
					/>
					{members.length === 0 ? <p className={styles.noResults}>找不到符合的成員</p> : null}
				</div>
				<div className={styles.pickerFooter}>
					<span>{value.length ? `已選擇 ${value.length} 人` : "所有人"}</span>
					<button type="button" onClick={() => changeOpen(false)}>
						完成
					</button>
				</div>
			</Dialog>
		</>
	);
}

function memberFilterLabel(members: DirectoryMember[]) {
	if (!members.length) return "所有人";
	if (members.length <= 2) return members.map((member) => member.displayName).join("、");
	return `${members[0]?.displayName ?? "成員"}等 ${members.length} 人`;
}
