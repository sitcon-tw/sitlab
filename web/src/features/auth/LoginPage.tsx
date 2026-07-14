import { GitBranch, LockKeyhole } from "lucide-react";
import styles from "./LoginPage.module.css";

export function LoginPage() {
	return (
		<main className={styles.page}>
			<section className={styles.login} aria-labelledby="login-title">
				<div className={styles.identity}>
					<div className={styles.mark} aria-hidden="true">
						<span>S</span>
						<span>27</span>
					</div>
					<div>
						<p className={styles.eyebrow}>SITCON / 2027</p>
						<h1 id="login-title">籌備工作看板</h1>
					</div>
				</div>
				<p className={styles.description}>使用 SITCON GitLab 帳號登入，繼續處理今年的籌備工作。</p>
				<a className={styles.loginButton} href="/api/v1/auth/gitlab">
					<GitBranch size="1.125rem" aria-hidden="true" />
					使用 GitLab 登入
				</a>
				<p className={styles.security}>
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
