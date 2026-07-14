import { errorMessage } from "@/shared/api/client";
import { Button, Panel, TextField } from "@project-template/ui";
import { CheckSquare2 } from "lucide-react";
import { useState, type FormEvent } from "react";
import { Link, Navigate, useLocation, useNavigate } from "react-router-dom";
import styles from "./AuthPage.module.css";
import { useSession } from "./SessionContext";

export type AuthMode = "login" | "register";

export function AuthPage({ mode }: { mode: AuthMode }) {
	const { user, submitting, login, register } = useSession();
	const navigate = useNavigate();
	const location = useLocation();
	const [displayName, setDisplayName] = useState("");
	const [email, setEmail] = useState("");
	const [password, setPassword] = useState("");
	const [error, setError] = useState<string | null>(null);
	const isRegister = mode === "register";

	if (user) return <Navigate to="/" replace />;

	async function submit(event: FormEvent) {
		event.preventDefault();
		setError(null);
		try {
			if (isRegister) await register({ displayName: displayName.trim(), email: email.trim(), password });
			else await login({ email: email.trim(), password });
			const target = typeof location.state === "object" && location.state && "from" in location.state ? String(location.state.from) : "/";
			navigate(target, { replace: true });
		} catch (caught) {
			setError(errorMessage(caught, isRegister ? "Could not create your account." : "Email or password is incorrect."));
		}
	}

	return (
		<main className={styles.page}>
			<section className={styles.intro} aria-labelledby="auth-heading">
				<div className={styles.brand}>
					<CheckSquare2 size="1.5rem" aria-hidden="true" />
					<span>Project Template</span>
				</div>
				<h1 id="auth-heading">{isRegister ? "Create your account" : "Welcome back"}</h1>
				<p>{isRegister ? "Create workspaces and keep the work that matters moving." : "Sign in to continue to your workspaces."}</p>
			</section>
			<Panel className={styles.formPanel} padded>
				<form className={styles.form} onSubmit={submit}>
					{isRegister ? (
						<TextField
							label="Name"
							name="displayName"
							autoComplete="name"
							required
							value={displayName}
							onChange={(event) => setDisplayName(event.target.value)}
						/>
					) : null}
					<TextField label="Email" name="email" type="email" autoComplete="email" required value={email} onChange={(event) => setEmail(event.target.value)} />
					<TextField
						label="Password"
						name="password"
						type="password"
						autoComplete={isRegister ? "new-password" : "current-password"}
						minLength={12}
						required
						description={isRegister ? "Use at least 12 characters." : undefined}
						value={password}
						onChange={(event) => setPassword(event.target.value)}
					/>
					{error ? (
						<div className={styles.error} role="alert">
							{error}
						</div>
					) : null}
					<Button type="submit" size="lg" loading={submitting}>
						{isRegister ? "Create account" : "Sign in"}
					</Button>
				</form>
				<p className={styles.switchMode}>
					{isRegister ? "Already have an account?" : "New here?"}{" "}
					<Link to={isRegister ? "/login" : "/register"}>{isRegister ? "Sign in" : "Create an account"}</Link>
				</p>
			</Panel>
		</main>
	);
}
