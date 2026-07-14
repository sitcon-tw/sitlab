import { Avatar } from "@/shared/Avatar";
import { Drawer } from "@project-template/ui";
import styles from "./BoardPage.module.css";
import type { Bootstrap } from "./model";

export function MembersDrawer({ bootstrap, open, onOpenChange }: { bootstrap: Bootstrap; open: boolean; onOpenChange: (open: boolean) => void }) {
	const teams = bootstrap.teams.filter((team) => team.active).sort((a, b) => a.sortOrder - b.sortOrder);
	const active = bootstrap.members.filter((member) => member.state === "active");
	const ungrouped = active.filter((member) => member.teamKeys.length === 0);
	return (
		<Drawer open={open} onOpenChange={onOpenChange} title="籌備團隊" description={`${active.length} 位 GitLab 專案成員`}>
			<div className={styles.directory}>
				{teams.map((team) => {
					const members = active.filter((member) => member.teamKeys.includes(team.key));
					return (
						<section className={styles.directoryTeam} key={team.key}>
							<h3>
								{team.name} <span>{members.length}</span>
							</h3>
							<ul>
								{members.map((member) => (
									<li key={member.gitLabUserId}>
										<Avatar name={member.displayName} src={member.avatarUrl} size="sm" />
										<a href={member.profileUrl} target="_blank" rel="noreferrer">
											{member.displayName} <small>@{member.username}</small>
										</a>
									</li>
								))}
							</ul>
						</section>
					);
				})}
				{ungrouped.length ? (
					<section className={styles.directoryTeam}>
						<h3>
							尚未分組 <span>{ungrouped.length}</span>
						</h3>
						<ul>
							{ungrouped.map((member) => (
								<li key={member.gitLabUserId}>
									<Avatar name={member.displayName} src={member.avatarUrl} size="sm" />
									<a href={member.profileUrl} target="_blank" rel="noreferrer">
										{member.displayName} <small>@{member.username}</small>
									</a>
								</li>
							))}
						</ul>
					</section>
				) : null}
			</div>
		</Drawer>
	);
}
