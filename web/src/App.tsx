import { LoginPage } from "@/features/auth/LoginPage";
import { BoardPage } from "@/features/board/BoardPage";
import { bootstrapQueryKey, refreshBootstrap } from "@/features/board/bootstrap";
import type { Bootstrap } from "@/features/board/model";
import { subscribeBootstrapEvents } from "@/features/board/realtime";
import { OnboardingPage } from "@/features/onboarding/OnboardingPage";
import { QueryClient, QueryClientProvider, useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useRef, useState } from "react";

const queryClient = new QueryClient({
	defaultOptions: {
		queries: { retry: false, refetchOnWindowFocus: false },
		mutations: { retry: false }
	}
});

export interface AppProps {
	initialBootstrap: Bootstrap | null;
	initialError?: string | null;
}

export function App({ initialBootstrap, initialError = null }: AppProps) {
	if (initialError) return <StartupError message={initialError} />;
	if (!initialBootstrap) return <LoginPage />;

	return (
		<QueryClientProvider client={queryClient}>
			<AuthenticatedApp initialBootstrap={initialBootstrap} />
		</QueryClientProvider>
	);
}

function AuthenticatedApp({ initialBootstrap }: { initialBootstrap: Bootstrap }) {
	const client = useQueryClient();
	const demo = import.meta.env.VITE_SITCON_DEMO === "true";
	const [boardDragging, setBoardDragging] = useState(false);
	const boardDraggingRef = useRef(false);
	const bootstrapQuery = useQuery({
		queryKey: bootstrapQueryKey,
		queryFn: refreshBootstrap,
		initialData: initialBootstrap,
		refetchInterval: demo || boardDragging ? false : 5_000,
		staleTime: demo ? Infinity : 4_000
	});
	const bootstrap = bootstrapQuery.data;
	useEffect(() => {
		if (demo) return;
		return subscribeBootstrapEvents((revision) => {
			if (boardDraggingRef.current) return;
			const current = client.getQueryData<Bootstrap>(bootstrapQueryKey);
			if (revision !== current?.revision) void client.invalidateQueries({ queryKey: bootstrapQueryKey });
		});
	}, [client, demo]);
	const handleBoardDraggingChange = useCallback(
		(dragging: boolean) => {
			boardDraggingRef.current = dragging;
			setBoardDragging(dragging);
			if (dragging) void client.cancelQueries({ queryKey: bootstrapQueryKey });
		},
		[client]
	);
	const updateBootstrap = (update: (current: Bootstrap) => Bootstrap) => {
		client.setQueryData<Bootstrap>(bootstrapQueryKey, (current) => update(current ?? bootstrap));
	};

	if (!bootstrap.preferences.confirmedAt) {
		return <OnboardingPage bootstrap={bootstrap} updateBootstrap={updateBootstrap} />;
	}

	return (
		<BoardPage
			bootstrap={bootstrap}
			updateBootstrap={updateBootstrap}
			backgroundOffline={bootstrapQuery.isRefetchError}
			onDraggingChange={handleBoardDraggingChange}
		/>
	);
}

function StartupError({ message }: { message: string }) {
	return (
		<main className="sb-startup-error">
			<p className="sb-brand">SITCON / 2027</p>
			<h1>無法開啟工作看板</h1>
			<p>{message}</p>
			<button type="button" className="sb-button sb-button-primary" onClick={() => window.location.reload()}>
				重新整理
			</button>
		</main>
	);
}
