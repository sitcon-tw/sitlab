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
							<h3 className="md-typescale-title-small">
								{team.name} <span className="md-typescale-label-small">{members.length}</span>
							</h3>
							<ul className="md-list">
								{members.map((member) => (
									<li key={member.gitLabUserId}>
										<a className="md-list-item md-list-item--two-line md-state-layer" href={member.profileUrl} target="_blank" rel="noreferrer">
											<span className="md-list-item__leading">
												<Avatar name={member.displayName} src={member.avatarUrl} size="sm" />
											</span>
											<span className="md-list-item__text">
												<span className="md-list-item__headline">{member.displayName}</span>
												<span className="md-list-item__supporting">@{member.username}</span>
											</span>
										</a>
									</li>
								))}
							</ul>
						</section>
					);
				})}
				{ungrouped.length ? (
					<section className={styles.directoryTeam}>
						<h3 className="md-typescale-title-small">
							尚未分組 <span className="md-typescale-label-small">{ungrouped.length}</span>
						</h3>
						<ul className="md-list">
							{ungrouped.map((member) => (
								<li key={member.gitLabUserId}>
									<a className="md-list-item md-list-item--two-line md-state-layer" href={member.profileUrl} target="_blank" rel="noreferrer">
										<span className="md-list-item__leading">
											<Avatar name={member.displayName} src={member.avatarUrl} size="sm" />
										</span>
										<span className="md-list-item__text">
											<span className="md-list-item__headline">{member.displayName}</span>
											<span className="md-list-item__supporting">@{member.username}</span>
										</span>
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
