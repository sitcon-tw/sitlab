import { Button } from "@project-template/ui";
import { GitBranch, LockKeyhole } from "lucide-react";
import styles from "./LoginPage.module.css";

export function LoginPage() {
	return (
		<main className={styles.page}>
			<section className={styles.login} aria-labelledby="login-title">
				<div className={styles.identity}>
					<div className={styles.mark} aria-hidden="true">
						<span className="md-typescale-title-large">S</span>
						<span className="md-typescale-label-medium">27</span>
					</div>
					<div>
						<p className={`md-typescale-label-small ${styles.eyebrow}`}>SITCON / 2027</p>
						<h1 className="md-typescale-headline-medium" id="login-title">
							籌備工作看板
						</h1>
					</div>
				</div>
				<p className={`md-typescale-body-large ${styles.description}`}>使用 SITCON GitLab 帳號登入，繼續處理今年的籌備工作。</p>
				<Button asChild variant="filled" leadingIcon={<GitBranch size="1.125rem" aria-hidden="true" />}>
					<a href="/api/v1/auth/gitlab">使用 GitLab 登入</a>
				</Button>
				<p className={`md-typescale-body-small ${styles.security}`}>
					<LockKeyhole size="0.875rem" aria-hidden="true" />
					僅限 sitcon-tw/2027 專案成員
				</p>
			</section>
			<div className={styles.boardPreview} aria-hidden="true">
				{[3, 2, 3, 1].map((count, column) => (
					<div className={styles.previewLane} key={column}>
						<span />
						{Array.from({ length: count }, (_, index) => (
							<i key={index} />
						))}
					</div>
				))}
			</div>
		</main>
	);
}
