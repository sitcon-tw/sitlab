import { useCallback, useEffect, useState } from "react";

export type Theme = "light" | "dark";

// Keep in sync with the inline bootstrap script in web/index.html.
const STORAGE_KEY = "sitcon-board-theme";

function systemTheme(): Theme {
	return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

function readStoredTheme(): Theme | null {
	try {
		const stored = localStorage.getItem(STORAGE_KEY);
		return stored === "light" || stored === "dark" ? stored : null;
	} catch {
		return null;
	}
}

export function useTheme() {
	const [theme, setTheme] = useState<Theme>(() => (document.documentElement.dataset.theme as Theme | undefined) ?? readStoredTheme() ?? systemTheme());

	useEffect(() => {
		document.documentElement.dataset.theme = theme;
	}, [theme]);

	useEffect(() => {
		if (readStoredTheme()) return;
		const media = window.matchMedia("(prefers-color-scheme: dark)");
		const onChange = (event: MediaQueryListEvent) => setTheme(event.matches ? "dark" : "light");
		media.addEventListener("change", onChange);
		return () => media.removeEventListener("change", onChange);
	}, []);

	const toggleTheme = useCallback(() => {
		setTheme((current) => {
			const next: Theme = current === "dark" ? "light" : "dark";
			try {
				localStorage.setItem(STORAGE_KEY, next);
			} catch {
				// Storage unavailable (e.g. private browsing); the toggle still works for this session.
			}
			return next;
		});
	}, []);

	return { theme, toggleTheme };
}
