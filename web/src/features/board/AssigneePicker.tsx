import { Avatar } from "@/shared/Avatar";
import { Dialog } from "@project-template/ui";
import { Search, UserRound, X } from "lucide-react";
import { useMemo, useState } from "react";
import styles from "./BoardPage.module.css";
import { activeMembers, memberById, type Bootstrap, type DirectoryMember } from "./model";

export interface AssigneePickerProps {
	bootstrap: Bootstrap;
	teamKey: string;
	value: number | null;
	onChange: (gitLabUserId: number | null) => void;
	label: string;
	compact?: boolean;
}

export function AssigneePicker({ bootstrap, teamKey, value, onChange, label, compact = false }: AssigneePickerProps) {
	const [open, setOpen] = useState(false);
	const [query, setQuery] = useState("");
	const selected = memberById(bootstrap, value);
	const members = useMemo(() => sortMembers(activeMembers(bootstrap), teamKey, bootstrap.me.gitLabUserId, query), [bootstrap, query, teamKey]);
	const teamName = bootstrap.teams.find((team) => team.key === teamKey)?.name ?? "目前組別";
	const teamMembers = members.filter((member) => member.teamKeys.includes(teamKey));
	const otherMembers = members.filter((member) => !member.teamKeys.includes(teamKey));

	const choose = (gitLabUserId: number | null) => {
		onChange(gitLabUserId);
		setOpen(false);
		setQuery("");
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
				{selected ? <Avatar name={selected.displayName} src={selected.avatarUrl} size="sm" /> : <UserRound size="1rem" aria-hidden="true" />}
				{compact ? null : <span>{selected ? selected.displayName : "未指派"}</span>}
			</button>
			<Dialog open={open} onOpenChange={setOpen} title="選擇 Assignee" description={`優先顯示${teamName}成員，也可以搜尋全部專案成員。`}>
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
					<button type="button" className={styles.memberOption} data-selected={value === null} onClick={() => choose(null)}>
						<span className={styles.unassignedAvatar}>
							<UserRound size="1rem" aria-hidden="true" />
						</span>
						<span>
							<strong>未指派</strong>
							<small>暫不指定負責人</small>
						</span>
					</button>
					<MemberGroup label={teamName} members={teamMembers} value={value} currentUserId={bootstrap.me.gitLabUserId} onChoose={choose} />
					<MemberGroup label="其他組別的人" members={otherMembers} value={value} currentUserId={bootstrap.me.gitLabUserId} onChoose={choose} />
					{members.length === 0 ? <p className={styles.noResults}>找不到符合的可指派成員</p> : null}
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
	onChoose
}: {
	label: string;
	members: DirectoryMember[];
	value: number | null;
	currentUserId: number;
	onChoose: (id: number) => void;
}) {
	if (!members.length) return null;
	return (
		<section className={styles.memberGroup} aria-label={label}>
			<h3>{label}</h3>
			{members.map((member) => (
				<button
					type="button"
					className={styles.memberOption}
					data-selected={value === member.gitLabUserId}
					key={member.gitLabUserId}
					onClick={() => onChoose(member.gitLabUserId)}
				>
					<Avatar name={member.displayName} src={member.avatarUrl} />
					<span>
						<strong>{member.displayName}</strong>
						<small>
							@{member.username}
							{member.gitLabUserId === currentUserId ? " · 你" : ""}
						</small>
					</span>
				</button>
			))}
		</section>
	);
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
