import { Dialog } from "@project-template/ui";
import { ArrowUpDown, Check, ChevronDown, Filter, Search, UsersRound, X } from "lucide-react";
import { useMemo, useState } from "react";
import styles from "./BoardPage.module.css";
import { GroupedMemberList } from "./GroupedMemberList";
import { activeMembers, filterDirectoryMembers, type BoardSortMode, type Bootstrap, type DirectoryMember } from "./model";

export interface BoardFiltersProps {
	bootstrap: Bootstrap;
	teamKey: string;
	memberIds: number[];
	sortMode: BoardSortMode;
	visibleCount: number;
	totalCount: number;
	onTeamChange: (teamKey: string) => void;
	onMemberIdsChange: (memberIds: number[]) => void;
	onSortModeChange: (sortMode: BoardSortMode) => void;
	onClear: () => void;
}

export function BoardFilters({
	bootstrap,
	teamKey,
	memberIds,
	sortMode,
	visibleCount,
	totalCount,
	onTeamChange,
	onMemberIdsChange,
	onSortModeChange,
	onClear
}: BoardFiltersProps) {
	const teams = bootstrap.teams.filter((team) => team.active).sort((a, b) => a.sortOrder - b.sortOrder);
	const active = Boolean(teamKey || memberIds.length);

	return (
		<section className={styles.filters} aria-label="篩選看板">
			<label className={styles.sortControl}>
				<span>
					<ArrowUpDown size="0.9375rem" aria-hidden="true" />
					<strong>Sort by</strong>
				</span>
				<select aria-label="排序方式" value={sortMode} onChange={(event) => onSortModeChange(event.target.value as BoardSortMode)}>
					<option value="manual">手動順序</option>
					<option value="due-asc">Due date：近到遠</option>
					<option value="due-desc">Due date：遠到近</option>
					<option value="start-asc">Start date：近到遠</option>
					<option value="start-desc">Start date：遠到近</option>
					<option value="updated-desc">Updated time：新到舊</option>
					<option value="updated-asc">Updated time：舊到新</option>
				</select>
			</label>
			<div className={styles.filterHeading}>
				<Filter size="0.9375rem" aria-hidden="true" />
				<strong>篩選</strong>
			</div>
			<label className={styles.filterTeam}>
				<span className={styles.srOnly}>篩選組別</span>
				<select aria-label="篩選組別" value={teamKey} onChange={(event) => onTeamChange(event.target.value)}>
					<option value="">所有組別</option>
					{teams.map((team) => (
						<option key={team.key} value={team.key}>
							{team.name}
						</option>
					))}
				</select>
			</label>
			<MemberFilter bootstrap={bootstrap} value={memberIds} onChange={onMemberIdsChange} />
			<span className={styles.filterResult} role="status" aria-live="polite">
				{visibleCount} / {totalCount} 張卡片
			</span>
			<button
				type="button"
				className={styles.clearFilters}
				data-visible={active}
				disabled={!active}
				aria-hidden={!active}
				aria-label="清除篩選"
				title="清除篩選"
				onClick={onClear}
			>
				<X size="1rem" aria-hidden="true" />
			</button>
		</section>
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

	const changeOpen = (next: boolean) => {
		setOpen(next);
		if (!next) setQuery("");
	};
	return (
		<>
			<button
				type="button"
				className={styles.filterPeople}
				aria-label="篩選負責人"
				title={selected.length ? selected.map((member) => member.displayName).join("、") : "所有人"}
				onClick={() => setOpen(true)}
			>
				<UsersRound size="0.9375rem" aria-hidden="true" />
				<span>{memberFilterLabel(selected)}</span>
				<ChevronDown size="0.875rem" aria-hidden="true" />
			</button>
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
					<GroupedMemberList teams={bootstrap.teams} members={members} value={value} currentUserId={bootstrap.me.gitLabUserId} onChange={onChange} />
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
