import { Avatar } from "@/shared/Avatar";
import { Dialog } from "@project-template/ui";
import { Check, Search, UsersRound, X } from "lucide-react";
import { useMemo, useState } from "react";
import styles from "./BoardPage.module.css";
import { GroupedMemberList } from "./GroupedMemberList";
import { activeMembers, filterDirectoryMembers, type Bootstrap, type DirectoryMember } from "./model";

export interface AssigneePickerProps {
	bootstrap: Bootstrap;
	teamKey: string;
	value: number[];
	onChange: (gitLabUserIds: number[]) => void;
	label: string;
	compact?: boolean;
}

export function AssigneePicker({ bootstrap, teamKey, value, onChange, label, compact = false }: AssigneePickerProps) {
	const [open, setOpen] = useState(false);
	const [query, setQuery] = useState("");
	const selected = value.flatMap((id) => {
		const member = bootstrap.members.find((candidate) => candidate.gitLabUserId === id);
		return member ? [member] : [];
	});
	const members = useMemo(() => filterDirectoryMembers(activeMembers(bootstrap), query), [bootstrap, query]);
	const teamName = bootstrap.teams.find((team) => team.key === teamKey)?.name ?? "目前組別";

	const changeOpen = (next: boolean) => {
		setOpen(next);
		if (!next) setQuery("");
	};
	return (
		<>
			<button
				type="button"
				className={compact ? styles.assigneeCompact : styles.assigneeControl}
				aria-label={label}
				title={label}
				onClick={() => setOpen(true)}
			>
				{selected.length ? (
					<span className={styles.assigneeStack} aria-hidden="true">
						{selected.slice(0, 3).map((member) => (
							<Avatar key={member.gitLabUserId} name={member.displayName} src={member.avatarUrl} size="sm" />
						))}
					</span>
				) : (
					<UsersRound size="1rem" aria-hidden="true" />
				)}
				{compact ? null : <span>{assigneeLabel(selected)}</span>}
				{compact && selected.length > 3 ? <small>+{selected.length - 3}</small> : null}
			</button>
			<Dialog open={open} onOpenChange={changeOpen} title="選擇 Assignee" description={`${teamName}與其他專案成員`}>
				<div className={styles.pickerSearch}>
					<Search size="1rem" aria-hidden="true" />
					<input
						autoFocus
						type="search"
						value={query}
						onChange={(event) => setQuery(event.target.value)}
						placeholder="搜尋姓名或 GitLab 帳號"
						aria-label="搜尋成員"
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
							<strong>未指派</strong>
							<small>清除所有負責人</small>
						</span>
						{value.length === 0 ? <Check size="1rem" aria-hidden="true" /> : null}
					</button>
					<GroupedMemberList
						teams={bootstrap.teams}
						members={members}
						value={value}
						currentUserId={bootstrap.me.gitLabUserId}
						onChange={onChange}
						preferredTeamKey={teamKey}
					/>
					{members.length === 0 ? <p className={styles.noResults}>找不到符合的可指派成員</p> : null}
				</div>
				<div className={styles.pickerFooter}>
					<span>{value.length ? `已選擇 ${value.length} 人` : "尚未指派"}</span>
					<button type="button" onClick={() => changeOpen(false)}>
						完成
					</button>
				</div>
			</Dialog>
		</>
	);
}

function assigneeLabel(members: DirectoryMember[]) {
	if (!members.length) return "未指派";
	if (members.length === 1) return members[0]?.displayName ?? "1 位負責人";
	return `${members.length} 位負責人`;
}
