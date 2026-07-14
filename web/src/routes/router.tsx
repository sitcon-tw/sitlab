import { AuthPage } from "@/features/auth/AuthPage";
import { AppShell } from "@/layouts/AppShell";
import { SpinnerLayout } from "@project-template/ui";
import { lazy, Suspense } from "react";
import { createBrowserRouter } from "react-router-dom";
import { NotFoundPage } from "./NotFoundPage";
import { ProtectedRoute } from "./ProtectedRoute";
import { WorkspaceIndex } from "./WorkspaceIndex";

const TasksPage = lazy(() => import("@/features/tasks/TasksPage").then((module) => ({ default: module.TasksPage })));
const taskRoute = (
	<Suspense fallback={<SpinnerLayout label="Loading tasks" />}>
		<TasksPage />
	</Suspense>
);

export const router = createBrowserRouter([
	{ path: "/login", element: <AuthPage mode="login" /> },
	{ path: "/register", element: <AuthPage mode="register" /> },
	{
		element: <ProtectedRoute />,
		children: [
			{
				element: <AppShell />,
				children: [
					{ index: true, element: <WorkspaceIndex /> },
					{ path: "workspaces/:workspaceId/tasks", element: taskRoute },
					{ path: "workspaces/:workspaceId/tasks/:taskId", element: taskRoute },
					{ path: "*", element: <NotFoundPage /> }
				]
			}
		]
	}
]);
