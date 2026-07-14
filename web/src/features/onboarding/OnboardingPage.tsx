import { savePreferences } from "@/features/board/boardApi";
import { activeMembers, type Bootstrap, type DirectoryTeam } from "@/features/board/model";
import { Avatar } from "@/shared/Avatar";
import { errorMessage } from "@/shared/api/client";
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
			<header className={styles.header}>
				<p className={styles.brand}>SITCON / 2027</p>
				<span>@{bootstrap.me.username}</span>
			</header>
			<section className={styles.content} aria-labelledby="onboarding-title">
				<div className={styles.intro}>
					<p className={styles.step}>初次設定</p>
					<h1 id="onboarding-title">選擇你的主要組別</h1>
					<p>這會成為快速開卡的預設組別。請查看成員後再確認。</p>
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
					{error ? <p role="alert">{error}</p> : <span />}
					<button type="button" className="sb-button sb-button-primary" disabled={!selectedTeam || submitting} onClick={confirm}>
						<Check size="1rem" aria-hidden="true" />
						{submitting ? "儲存中..." : "確認主要組別"}
					</button>
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
		<article className={styles.team} data-selected={selected}>
			<div className={styles.teamSummary}>
				<button type="button" className={styles.teamSelect} role="radio" aria-checked={selected} onClick={onSelect}>
					<span className={styles.radio}>{selected ? <Check size="0.75rem" aria-hidden="true" /> : null}</span>
					<span>
						<strong>{team.name}</strong>
						<small>{members.length} 人</small>
					</span>
					<span className={styles.avatars}>
						{members.slice(0, 3).map((member) => (
							<Avatar key={member.gitLabUserId} name={member.displayName} src={member.avatarUrl} size="sm" />
						))}
						{members.length > 3 ? <i>+{members.length - 3}</i> : null}
					</span>
				</button>
				<button
					type="button"
					className={styles.expand}
					aria-label={`${expanded ? "收合" : "展開"}${team.name}成員`}
					aria-expanded={expanded}
					onClick={onToggle}
				>
					{expanded ? <ChevronUp size="1rem" aria-hidden="true" /> : <ChevronDown size="1rem" aria-hidden="true" />}
				</button>
			</div>
			{expanded ? (
				<ul className={styles.memberList}>
					{members.length ? (
						members.map((member) => (
							<li key={member.gitLabUserId}>
								<Avatar name={member.displayName} src={member.avatarUrl} size="sm" />
								<span>
									{member.displayName} <small>@{member.username}</small>
								</span>
							</li>
						))
					) : (
						<li className={styles.empty}>目前沒有可指派成員</li>
					)}
				</ul>
			) : null}
		</article>
	);
}
