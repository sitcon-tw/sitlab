import { savePreferences } from "@/features/board/boardApi";
import { activeMembers, type Bootstrap, type DirectoryTeam } from "@/features/board/model";
import { Avatar } from "@/shared/Avatar";
import { errorMessage } from "@/shared/api/client";
import { Button, IconButton, Panel, TopAppBar } from "@project-template/ui";
import { Check, ChevronDown, ChevronUp } from "lucide-react";
import { useState } from "react";
import styles from "./OnboardingPage.module.css";

export interface OnboardingPageProps {
	bootstrap: Bootstrap;
	updateBootstrap: (update: (current: Bootstrap) => Bootstrap) => void;
}

export function OnboardingPage({ bootstrap, updateBootstrap }: OnboardingPageProps) {
	const suggestedTeam = bootstrap.preferences.defaultTeamKey ?? bootstrap.preferences.directoryTeamKeys[0] ?? "";
	const [selectedTeam, setSelectedTeam] = useState(suggestedTeam);
	const [expandedTeam, setExpandedTeam] = useState<string | null>(suggestedTeam || null);
	const [submitting, setSubmitting] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const teams = bootstrap.teams.filter((team) => team.active).sort((a, b) => a.sortOrder - b.sortOrder);

	const confirm = async () => {
		if (!selectedTeam || submitting) return;
		setSubmitting(true);
		setError(null);
		try {
			const response = await savePreferences(selectedTeam);
			updateBootstrap((current) => ({ ...current, preferences: response.preferences }));
		} catch (cause) {
			setError(errorMessage(cause, "目前無法儲存主要組別，請再試一次。"));
		} finally {
			setSubmitting(false);
		}
	};

	return (
		<main className={styles.page}>
			<TopAppBar className={styles.header} headline="SITCON / 2027" trailing={<span className="md-typescale-label-large">@{bootstrap.me.username}</span>} />
			<section className={styles.content} aria-labelledby="onboarding-title">
				<div className={styles.intro}>
					<p className={`md-typescale-label-small ${styles.step}`}>初次設定</p>
					<h1 className="md-typescale-headline-large" id="onboarding-title">
						選擇你的主要組別
					</h1>
					<p className="md-typescale-body-large">這會成為快速開卡的預設組別。請查看成員後再確認。</p>
				</div>
				<div className={styles.teams} role="radiogroup" aria-label="主要組別">
					{teams.map((team) => (
						<TeamChoice
							key={team.key}
							team={team}
							bootstrap={bootstrap}
							selected={selectedTeam === team.key}
							expanded={expandedTeam === team.key}
							onSelect={() => setSelectedTeam(team.key)}
							onToggle={() => setExpandedTeam((current) => (current === team.key ? null : team.key))}
						/>
					))}
				</div>
				<footer className={styles.footer}>
					{error ? (
						<p className="md-typescale-body-medium" role="alert">
							{error}
						</p>
					) : (
						<span />
					)}
					<Button
						variant="filled"
						disabled={!selectedTeam}
						loading={submitting}
						loadingLabel="儲存中"
						leadingIcon={<Check size="1rem" aria-hidden="true" />}
						onClick={confirm}
					>
						確認主要組別
					</Button>
				</footer>
			</section>
		</main>
	);
}

function TeamChoice({
	team,
	bootstrap,
	selected,
	expanded,
	onSelect,
	onToggle
}: {
	team: DirectoryTeam;
	bootstrap: Bootstrap;
	selected: boolean;
	expanded: boolean;
	onSelect: () => void;
	onToggle: () => void;
}) {
	const members = activeMembers(bootstrap).filter((member) => member.teamKeys.includes(team.key));
	return (
		<Panel className={styles.team} data-selected={selected}>
			<div className={styles.teamSummary}>
				<label className={`md-list-item md-list-item--two-line ${styles.teamSelect}`}>
					<input type="radio" className="md-radio" name="primary-team" checked={selected} onChange={onSelect} />
					<span className="md-list-item__text">
						<span className="md-list-item__headline">{team.name}</span>
						<span className="md-list-item__supporting">{members.length} 人</span>
					</span>
					<span className={styles.avatars} aria-hidden="true">
						{members.slice(0, 3).map((member) => (
							<Avatar key={member.gitLabUserId} name={member.displayName} src={member.avatarUrl} size="sm" />
						))}
						{members.length > 3 ? <i className="md-typescale-label-small">+{members.length - 3}</i> : null}
					</span>
				</label>
				<IconButton
					className={styles.expand}
					label={`${expanded ? "收合" : "展開"}${team.name}成員`}
					aria-expanded={expanded}
					icon={expanded ? <ChevronUp size="1rem" aria-hidden="true" /> : <ChevronDown size="1rem" aria-hidden="true" />}
					onClick={onToggle}
				/>
			</div>
			{expanded ? (
				<ul className={`md-list ${styles.memberList}`}>
					{members.length ? (
						members.map((member) => (
							<li className="md-list-item md-list-item--two-line" key={member.gitLabUserId}>
								<span className="md-list-item__leading">
									<Avatar name={member.displayName} src={member.avatarUrl} size="sm" />
								</span>
								<span className="md-list-item__text">
									<span className="md-list-item__headline">{member.displayName}</span>
									<span className="md-list-item__supporting">@{member.username}</span>
								</span>
							</li>
						))
					) : (
						<li className="md-list-item">目前沒有可指派成員</li>
					)}
				</ul>
			) : null}
		</Panel>
	);
}
