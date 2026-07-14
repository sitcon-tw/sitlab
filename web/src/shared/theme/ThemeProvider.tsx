import { useEffect, useMemo, useState, type ReactNode } from "react";
import { ThemeContext, type Theme } from "./themeContext";
const storageKey = "project-template:theme";

function initialTheme(): Theme {
	if (typeof window === "undefined") return "light";
	const saved = window.localStorage.getItem(storageKey);
	if (saved === "light" || saved === "dark") return saved;
	return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

export function ThemeProvider({ children }: { children: ReactNode }) {
	const [theme, setTheme] = useState<Theme>(initialTheme);
	useEffect(() => {
		document.documentElement.dataset.theme = theme;
		window.localStorage.setItem(storageKey, theme);
	}, [theme]);
	const value = useMemo(() => ({ theme, setTheme, toggleTheme: () => setTheme((current) => (current === "dark" ? "light" : "dark")) }), [theme]);
	return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}
