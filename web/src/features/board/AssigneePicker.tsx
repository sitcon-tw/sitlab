import { Avatar } from "@/shared/Avatar";
import { Dialog } from "@project-template/ui";
import { Check, Search, UsersRound, X } from "lucide-react";
import { useMemo, useState } from "react";
import styles from "./BoardPage.module.css";
import { activeMembers, type Bootstrap, type DirectoryMember } from "./model";

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
	const members = useMemo(() => sortMembers(activeMembers(bootstrap), teamKey, bootstrap.me.gitLabUserId, query), [bootstrap, query, teamKey]);
	const teamName = bootstrap.teams.find((team) => team.key === teamKey)?.name ?? "目前組別";
	const teamMembers = members.filter((member) => member.teamKeys.includes(teamKey));
	const otherMembers = members.filter((member) => !member.teamKeys.includes(teamKey));

	const changeOpen = (next: boolean) => {
		setOpen(next);
		if (!next) setQuery("");
	};
	const toggle = (gitLabUserId: number) => {
		onChange(value.includes(gitLabUserId) ? value.filter((id) => id !== gitLabUserId) : [...value, gitLabUserId]);
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
					<MemberGroup label={teamName} members={teamMembers} value={value} currentUserId={bootstrap.me.gitLabUserId} onToggle={toggle} />
					<MemberGroup label="其他組別的人" members={otherMembers} value={value} currentUserId={bootstrap.me.gitLabUserId} onToggle={toggle} />
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

function MemberGroup({
	label,
	members,
	value,
	currentUserId,
	onToggle
}: {
	label: string;
	members: DirectoryMember[];
	value: number[];
	currentUserId: number;
	onToggle: (id: number) => void;
}) {
	if (!members.length) return null;
	return (
		<section className={styles.memberGroup} aria-label={label}>
			<h3>{label}</h3>
			{members.map((member) => {
				const selected = value.includes(member.gitLabUserId);
				return (
					<button
						type="button"
						role="checkbox"
						aria-checked={selected}
						className={styles.memberOption}
						data-selected={selected}
						key={member.gitLabUserId}
						onClick={() => onToggle(member.gitLabUserId)}
					>
						<Avatar name={member.displayName} src={member.avatarUrl} />
						<span>
							<strong>{member.displayName}</strong>
							<small>
								@{member.username}
								{member.gitLabUserId === currentUserId ? " · 你" : ""}
							</small>
						</span>
						{selected ? <Check size="1rem" aria-hidden="true" /> : null}
					</button>
				);
			})}
		</section>
	);
}

function assigneeLabel(members: DirectoryMember[]) {
	if (!members.length) return "未指派";
	if (members.length === 1) return members[0]?.displayName ?? "1 位負責人";
	return `${members.length} 位負責人`;
}

function sortMembers(members: DirectoryMember[], teamKey: string, currentUserId: number, query: string) {
	const normalized = query.trim().toLocaleLowerCase("zh-Hant");
	return members
		.filter((member) => !normalized || `${member.displayName} ${member.username}`.toLocaleLowerCase("zh-Hant").includes(normalized))
		.sort((a, b) => {
			if (a.gitLabUserId === currentUserId && b.gitLabUserId !== currentUserId) return -1;
			if (b.gitLabUserId === currentUserId && a.gitLabUserId !== currentUserId) return 1;
			const aInTeam = a.teamKeys.includes(teamKey);
			const bInTeam = b.teamKeys.includes(teamKey);
			if (aInTeam !== bInTeam) return aInTeam ? -1 : 1;
			return a.displayName.localeCompare(b.displayName, "zh-Hant");
		});
}
