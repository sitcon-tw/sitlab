import { Avatar } from "@/shared/Avatar";
import { Button, Chip, Dialog, EmptyState, IconButton, TextField } from "@project-template/ui";
import { Check, UsersRound, X } from "lucide-react";
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
	const preferredTeamKey = bootstrap.preferences.defaultTeamKey ?? bootstrap.preferences.directoryTeamKeys[0] ?? "";

	const changeOpen = (next: boolean) => {
		setOpen(next);
		if (!next) setQuery("");
	};
	const trigger = (
		<Chip
			variant="input"
			className={compact ? styles.assigneeCompact : styles.assigneeControl}
			label={assigneeLabel(selected)}
			aria-label={label}
			title={label}
			leading={
				selected.length ? (
					<span className={styles.assigneeStack} aria-hidden="true">
						{selected.slice(0, 3).map((member) => (
							<Avatar key={member.gitLabUserId} name={member.displayName} src={member.avatarUrl} size="sm" />
						))}
					</span>
				) : (
					<UsersRound size="1rem" aria-hidden="true" />
				)
			}
			trailing={compact && selected.length > 3 ? <small>+{selected.length - 3}</small> : undefined}
			onClick={() => setOpen(true)}
		/>
	);
	return (
		<>
			{trigger}
			<Dialog
				open={open}
				onOpenChange={changeOpen}
				title="選擇 Assignee"
				description={`${teamName}與其他專案成員`}
				footer={
					<div className={styles.pickerFooter}>
						<span className="md-typescale-label-small">{value.length ? `已選擇 ${value.length} 人` : "尚未指派"}</span>
						<Button variant="text" onClick={() => changeOpen(false)}>
							完成
						</Button>
					</div>
				}
			>
				<div className={styles.pickerSearch}>
					<TextField
						autoFocus
						type="search"
						label="搜尋成員"
						value={query}
						onChange={(event) => setQuery(event.target.value)}
						placeholder="搜尋姓名或 GitLab 帳號"
					/>
					{query ? <IconButton size="sm" label="清除搜尋" icon={<X size="1rem" aria-hidden="true" />} onClick={() => setQuery("")} /> : null}
				</div>
				<div className={`md-list ${styles.pickerList}`}>
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
							<span className="md-list-item__headline">未指派</span>
							<span className="md-list-item__supporting">清除所有負責人</span>
						</span>
						{value.length === 0 ? (
							<span className="md-list-item__trailing">
								<Check size="1.125rem" aria-hidden="true" />
							</span>
						) : null}
					</button>
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
			</Dialog>
		</>
	);
}

function assigneeLabel(members: DirectoryMember[]) {
	if (!members.length) return "未指派";
	if (members.length === 1) return members[0]?.displayName ?? "1 位負責人";
	return `${members.length} 位負責人`;
}
