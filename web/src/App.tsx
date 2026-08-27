import { LoginPage } from "@/features/auth/LoginPage";
import { BoardPage } from "@/features/board/BoardPage";
import { bootstrapQueryKey, refreshBootstrap } from "@/features/board/bootstrap";
import type { Bootstrap } from "@/features/board/model";
import { fetchSyncDelta, subscribeBootstrapEvents, subscribeSyncEvents, type SyncDelta } from "@/features/board/realtime";
import { applySyncActions, compareCheckpoints, isImmediateSuccessor } from "@/features/board/syncActions";
import { OnboardingPage } from "@/features/onboarding/OnboardingPage";
import { ApiError } from "@/shared/api/client";
import { Button } from "@project-template/ui";
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
	const deltaEnabled = import.meta.env.VITE_SITCON_SYNC_DELTA !== "false";
	const [boardDragging, setBoardDragging] = useState(false);
	const [streamConnected, setStreamConnected] = useState(false);
	const [streamEpoch, setStreamEpoch] = useState(0);
	const boardDraggingRef = useRef(false);
	const missedWhileDraggingRef = useRef(false);
	const inflightOperations = useRef(new Map<string, () => void>());
	const catchUpRequestedRef = useRef(false);
	const catchUpPromiseRef = useRef<Promise<void> | null>(null);
	const resetPromiseRef = useRef<Promise<void> | null>(null);
	const bootstrapQuery = useQuery({
		queryKey: bootstrapQueryKey,
		queryFn: refreshBootstrap,
		initialData: initialBootstrap,
		refetchInterval: demo || boardDragging ? false : deltaEnabled ? (streamConnected ? false : 30_000) : 5_000,
		staleTime: demo || deltaEnabled ? Infinity : 4_000
	});
	const bootstrap = bootstrapQuery.data;

	const resetBootstrap = useCallback(() => {
		if (resetPromiseRef.current) return resetPromiseRef.current;
		const reset = refreshBootstrap()
			.then((fresh) => {
				client.setQueryData(bootstrapQueryKey, fresh);
				setStreamEpoch((current) => current + 1);
			})
			.finally(() => {
				resetPromiseRef.current = null;
			});
		resetPromiseRef.current = reset;
		return reset;
	}, [client]);

	const requestCatchUp = useCallback(() => {
		if (boardDraggingRef.current) {
			missedWhileDraggingRef.current = true;
			return;
		}
		catchUpRequestedRef.current = true;
		if (catchUpPromiseRef.current) return;

		const catchUp = (async () => {
			while (catchUpRequestedRef.current && !boardDraggingRef.current) {
				catchUpRequestedRef.current = false;
				let since = client.getQueryData<Bootstrap>(bootstrapQueryKey)?.revision;
				if (!since) return;
				for (;;) {
					if (boardDraggingRef.current) {
						missedWhileDraggingRef.current = true;
						return;
					}
					let delta: SyncDelta;
					try {
						delta = await fetchSyncDelta(since);
					} catch (cause) {
						if (cause instanceof ApiError && cause.code === "SYNC_CHECKPOINT_TOO_OLD") {
							await resetBootstrap();
							return;
						}
						throw cause;
					}
					if (boardDraggingRef.current) {
						missedWhileDraggingRef.current = true;
						return;
					}
					client.setQueryData<Bootstrap>(bootstrapQueryKey, (current) =>
						current
							? applySyncActions(current, delta.checkpoint, delta.actions, {
									dragging: false,
									inflightOperationIds: inflightOperations.current
								})
							: current
					);
					since = delta.checkpoint;
					if (!delta.hasMore) break;
				}
			}
		})()
			.catch(() => {
				// The disconnected-stream poll remains the coarse recovery path for a
				// transient catch-up failure.
				void client.invalidateQueries({ queryKey: bootstrapQueryKey });
			})
			.finally(() => {
				catchUpPromiseRef.current = null;
			});
		catchUpPromiseRef.current = catchUp;
	}, [client, resetBootstrap]);

	const handleStreamDelta = useCallback(
		(delta: SyncDelta) => {
			if (boardDraggingRef.current) {
				missedWhileDraggingRef.current = true;
				return;
			}
			const current = client.getQueryData<Bootstrap>(bootstrapQueryKey);
			if (!current || compareCheckpoints(delta.checkpoint, current.revision) <= 0) return;
			const firstUnseen = delta.actions.find((action) => compareCheckpoints(action.syncId, current.revision) > 0);
			if (!firstUnseen || !isImmediateSuccessor(current.revision, firstUnseen.syncId)) {
				requestCatchUp();
				return;
			}
			client.setQueryData<Bootstrap>(bootstrapQueryKey, (latest) =>
				latest
					? applySyncActions(latest, delta.checkpoint, delta.actions, {
							dragging: boardDraggingRef.current,
							inflightOperationIds: inflightOperations.current
						})
					: latest
			);
		},
		[client, requestCatchUp]
	);

	useEffect(() => {
		if (demo || deltaEnabled) return;
		return subscribeBootstrapEvents((revision) => {
			if (boardDraggingRef.current) return;
			const current = client.getQueryData<Bootstrap>(bootstrapQueryKey);
			if (revision !== current?.revision) void client.invalidateQueries({ queryKey: bootstrapQueryKey });
		});
	}, [client, deltaEnabled, demo]);

	useEffect(() => {
		if (demo || !deltaEnabled) return;
		const checkpoint = client.getQueryData<Bootstrap>(bootstrapQueryKey)?.revision;
		if (!checkpoint) return;
		return subscribeSyncEvents(checkpoint, {
			onDelta: handleStreamDelta,
			onHeartbeat: (serverCheckpoint) => {
				const current = client.getQueryData<Bootstrap>(bootstrapQueryKey);
				if (current && compareCheckpoints(serverCheckpoint, current.revision) > 0) requestCatchUp();
			},
			onReset: () => {
				setStreamConnected(false);
				void resetBootstrap().catch(() => client.invalidateQueries({ queryKey: bootstrapQueryKey }));
			},
			onConnectionChange: setStreamConnected
		});
	}, [client, deltaEnabled, demo, handleStreamDelta, requestCatchUp, resetBootstrap, streamEpoch]);

	useEffect(() => {
		if (demo || !deltaEnabled) return;
		const onVisible = () => {
			if (document.visibilityState === "visible") requestCatchUp();
		};
		window.addEventListener("online", requestCatchUp);
		document.addEventListener("visibilitychange", onVisible);
		return () => {
			window.removeEventListener("online", requestCatchUp);
			document.removeEventListener("visibilitychange", onVisible);
		};
	}, [deltaEnabled, demo, requestCatchUp]);

	const handleBoardDraggingChange = useCallback(
		(dragging: boolean) => {
			boardDraggingRef.current = dragging;
			setBoardDragging(dragging);
			if (dragging) {
				void client.cancelQueries({ queryKey: bootstrapQueryKey });
				return;
			}
			if (missedWhileDraggingRef.current) {
				missedWhileDraggingRef.current = false;
				requestCatchUp();
			}
		},
		[client, requestCatchUp]
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
			inflightOperations={inflightOperations}
		/>
	);
}

function StartupError({ message }: { message: string }) {
	return (
		<main className="sb-startup-error">
			<p className="sb-brand">SITCON / 2027</p>
			<h1>無法開啟工作看板</h1>
			<p>{message}</p>
			<Button variant="filled" onClick={() => window.location.reload()}>
				重新整理
			</Button>
		</main>
	);
}
