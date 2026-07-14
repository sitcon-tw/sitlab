import { useSession } from "@/features/auth/SessionContext";
import { useTheme } from "@/shared/theme/themeContext";
import { Button, IconButton } from "@project-template/ui";
import { CheckSquare2, ListTodo, LogOut, Menu, Moon, Sun, X } from "lucide-react";
import { useState } from "react";
import { NavLink, Outlet, useParams } from "react-router-dom";
import styles from "./AppShell.module.css";
import { WorkspaceSwitcher } from "./WorkspaceSwitcher";

export function AppShell() {
	const { user, logout, submitting } = useSession();
	const { workspaceId } = useParams();
	const { theme, toggleTheme } = useTheme();
	const [navigationOpen, setNavigationOpen] = useState(false);
	const tasksPath = workspaceId ? `/workspaces/${encodeURIComponent(workspaceId)}/tasks` : "/";

	return (
		<div className={styles.shell}>
			<a className={styles.skipLink} href="#main-content">
				Skip to content
			</a>
			<header className={styles.mobileHeader}>
				<div className={styles.brand}>
					<CheckSquare2 size="1.25rem" aria-hidden="true" />
					<span>Project Template</span>
				</div>
				<IconButton
					label={navigationOpen ? "Close navigation" : "Open navigation"}
					icon={navigationOpen ? <X size="1.25rem" aria-hidden="true" /> : <Menu size="1.25rem" aria-hidden="true" />}
					onClick={() => setNavigationOpen((open) => !open)}
				/>
			</header>
			<aside className={styles.sidebar} data-open={navigationOpen}>
				<div className={styles.brand}>
					<CheckSquare2 size="1.25rem" aria-hidden="true" />
					<span>Project Template</span>
				</div>
				<WorkspaceSwitcher />
				<nav className={styles.navigation} aria-label="Workspace navigation">
					<NavLink to={tasksPath} onClick={() => setNavigationOpen(false)}>
						<ListTodo size="1.125rem" aria-hidden="true" />
						<span>Tasks</span>
					</NavLink>
				</nav>
				<div className={styles.account}>
					<div className={styles.userSummary}>
						<strong>{user?.displayName}</strong>
						<span>{user?.email}</span>
					</div>
					<div className={styles.accountActions}>
						<IconButton
							label={`Use ${theme === "dark" ? "light" : "dark"} theme`}
							icon={theme === "dark" ? <Sun size="1rem" aria-hidden="true" /> : <Moon size="1rem" aria-hidden="true" />}
							onClick={toggleTheme}
						/>
						<Button size="sm" variant="ghost" loading={submitting} leadingIcon={<LogOut size="1rem" aria-hidden="true" />} onClick={() => void logout()}>
							Sign out
						</Button>
					</div>
				</div>
			</aside>
			{navigationOpen ? <button className={styles.navScrim} aria-label="Close navigation" onClick={() => setNavigationOpen(false)} /> : null}
			<main id="main-content" className={styles.content}>
				<Outlet />
			</main>
		</div>
	);
}
