import { Avatar } from "@/shared/Avatar";
import { Check } from "lucide-react";
import { useMemo } from "react";
import styles from "./BoardPage.module.css";
import type { DirectoryMember, DirectoryTeam } from "./model";

interface GroupedMemberListProps {
	teams: DirectoryTeam[];
	members: DirectoryMember[];
	value: number[];
	currentUserId: number;
	onChange: (memberIds: number[]) => void;
	preferredTeamKey?: string;
}

export function GroupedMemberList({ teams, members, value, currentUserId, onChange, preferredTeamKey }: GroupedMemberListProps) {
	const groups = useMemo(() => memberGroups(teams, members, currentUserId, preferredTeamKey), [currentUserId, members, preferredTeamKey, teams]);

	const toggleMember = (gitLabUserId: number) => {
		onChange(value.includes(gitLabUserId) ? value.filter((id) => id !== gitLabUserId) : [...value, gitLabUserId]);
	};

	const toggleGroup = (groupMembers: DirectoryMember[]) => {
		const groupIds = new Set(groupMembers.map((member) => member.gitLabUserId));
		const allSelected = groupMembers.every((member) => value.includes(member.gitLabUserId));
		if (allSelected) {
			onChange(value.filter((id) => !groupIds.has(id)));
			return;
		}
		onChange([...new Set([...value, ...groupMembers.map((member) => member.gitLabUserId)])]);
	};

	return groups.map((group) => (
		<MemberGroup
			key={group.key}
			label={group.label}
			members={group.members}
			value={value}
			currentUserId={currentUserId}
			onToggleMember={toggleMember}
			onToggleGroup={toggleGroup}
		/>
	));
}

function MemberGroup({
	label,
	members,
	value,
	currentUserId,
	onToggleMember,
	onToggleGroup
}: {
	label: string;
	members: DirectoryMember[];
	value: number[];
	currentUserId: number;
	onToggleMember: (id: number) => void;
	onToggleGroup: (members: DirectoryMember[]) => void;
}) {
	const selectedCount = members.filter((member) => value.includes(member.gitLabUserId)).length;
	const allSelected = selectedCount === members.length;
	const partiallySelected = selectedCount > 0 && !allSelected;

	return (
		<section className={styles.memberGroup} aria-label={label}>
			<h3>
				<label className={styles.memberGroupHeading}>
					<input
						type="checkbox"
						checked={allSelected}
						ref={(input) => {
							if (input) input.indeterminate = partiallySelected;
						}}
						onChange={() => onToggleGroup(members)}
						aria-label={`全選${label}`}
					/>
					<span>{label}</span>
					<small>{members.length}</small>
				</label>
			</h3>
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
						onClick={() => onToggleMember(member.gitLabUserId)}
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

function memberGroups(teams: DirectoryTeam[], members: DirectoryMember[], currentUserId: number, preferredTeamKey?: string) {
	const activeTeams = teams
		.filter((team) => team.active)
		.sort((a, b) => {
			if (a.key === preferredTeamKey) return -1;
			if (b.key === preferredTeamKey) return 1;
			return a.sortOrder - b.sortOrder;
		});
	const activeTeamKeys = new Set(activeTeams.map((team) => team.key));
	const groups = activeTeams.flatMap((team) => {
		const teamMembers = sortMembers(
			members.filter((member) => member.teamKeys.includes(team.key)),
			currentUserId
		);
		return teamMembers.length ? [{ key: team.key, label: team.name, members: teamMembers }] : [];
	});
	const ungrouped = sortMembers(
		members.filter((member) => !member.teamKeys.some((teamKey) => activeTeamKeys.has(teamKey))),
		currentUserId
	);
	if (ungrouped.length) groups.push({ key: "ungrouped", label: "尚未分組", members: ungrouped });
	return groups;
}

function sortMembers(members: DirectoryMember[], currentUserId: number) {
	return [...members].sort((a, b) => {
		if (a.gitLabUserId === currentUserId && b.gitLabUserId !== currentUserId) return -1;
		if (b.gitLabUserId === currentUserId && a.gitLabUserId !== currentUserId) return 1;
		return a.displayName.localeCompare(b.displayName, "zh-Hant");
	});
}
