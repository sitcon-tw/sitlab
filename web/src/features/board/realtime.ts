import { api, apiBaseUrl, expectData } from "@/shared/api/client";
import type { components } from "@/shared/api/openapi";

export type SyncDelta = components["schemas"]["SyncDeltaResponse"];

export interface SyncEventHandlers {
	onDelta(delta: SyncDelta): void;
	onHeartbeat(checkpoint: string): void;
	onReset(reason: string, checkpoint: string): void;
	onConnectionChange(connected: boolean): void;
}

export async function fetchSyncDelta(since: string, limit?: number) {
	return expectData(
		await api.GET("/sync", {
			params: { query: { since, ...(limit === undefined ? {} : { limit }) } }
		})
	);
}

export function subscribeSyncEvents(since: string, handlers: SyncEventHandlers) {
	const query = new URLSearchParams({ since });
	const events = new EventSource(`${apiBaseUrl}/events/sync?${query}`, { withCredentials: true });
	const onOpen = () => handlers.onConnectionChange(true);
	const onError = () => handlers.onConnectionChange(false);
	const onSync = (event: Event) => {
		const delta = parseEvent<SyncDelta>(event);
		if (delta?.checkpoint && Array.isArray(delta.actions)) handlers.onDelta(delta);
	};
	const onHeartbeat = (event: Event) => {
		const heartbeat = parseEvent<{ checkpoint?: string }>(event);
		if (heartbeat?.checkpoint) handlers.onHeartbeat(heartbeat.checkpoint);
	};
	const onReset = (event: Event) => {
		const reset = parseEvent<{ reason?: string; checkpoint?: string }>(event);
		if (reset?.reason && reset.checkpoint) handlers.onReset(reset.reason, reset.checkpoint);
	};
	events.addEventListener("open", onOpen);
	events.addEventListener("error", onError);
	events.addEventListener("sync", onSync);
	events.addEventListener("heartbeat", onHeartbeat);
	events.addEventListener("reset", onReset);
	return () => {
		events.removeEventListener("open", onOpen);
		events.removeEventListener("error", onError);
		events.removeEventListener("sync", onSync);
		events.removeEventListener("heartbeat", onHeartbeat);
		events.removeEventListener("reset", onReset);
		events.close();
		handlers.onConnectionChange(false);
	};
}

export function subscribeBootstrapEvents(onRevision: (revision: string) => void) {
	const events = new EventSource(`${apiBaseUrl}/events/bootstrap`, { withCredentials: true });
	const onBootstrap = (event: Event) => {
		try {
			const { revision } = JSON.parse((event as MessageEvent<string>).data) as { revision?: string };
			if (revision) onRevision(revision);
		} catch {
			// Polling remains the recovery path for malformed events.
		}
	};
	events.addEventListener("bootstrap", onBootstrap);
	return () => {
		events.removeEventListener("bootstrap", onBootstrap);
		events.close();
	};
}

function parseEvent<T>(event: Event) {
	try {
		return JSON.parse((event as MessageEvent<string>).data) as T;
	} catch {
		return null;
	}
}
