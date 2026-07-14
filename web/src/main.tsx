import "@project-template/ui/styles";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App } from "./App";
import { loadInitialBootstrap } from "./features/board/bootstrap";
import "./index.css";

const root = document.getElementById("root");
if (!root) throw new Error("Application root not found.");

let initialError: string | null = null;
const initialBootstrap = await loadInitialBootstrap().catch((error: unknown) => {
	initialError = error instanceof Error ? error.message : "目前無法連線到 SITCON Board。";
	return null;
});

createRoot(root).render(
	<StrictMode>
		<App initialBootstrap={initialBootstrap} initialError={initialError} />
	</StrictMode>
);
