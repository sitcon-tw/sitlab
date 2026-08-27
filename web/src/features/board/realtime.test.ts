import { afterEach, describe, expect, it, vi } from "vitest";
import { subscribeBootstrapEvents, subscribeSyncEvents } from "./realtime";

class EventSourceFake {
	static instance: EventSourceFake | null = null;
	readonly url: string;
	readonly withCredentials: boolean;
	closed = false;
	private listeners = new Map<string, Set<EventListener>>();

	constructor(url: string | URL, options?: EventSourceInit) {
		this.url = String(url);
		this.withCredentials = options?.withCredentials ?? false;
		EventSourceFake.instance = this;
	}

	addEventListener(type: string, listener: EventListenerOrEventListenerObject) {
		const normalized = typeof listener === "function" ? listener : listener.handleEvent.bind(listener);
		const listeners = this.listeners.get(type) ?? new Set<EventListener>();
		listeners.add(normalized);
		this.listeners.set(type, listeners);
	}

	removeEventListener(type: string, listener: EventListenerOrEventListenerObject) {
		if (typeof listener === "function") this.listeners.get(type)?.delete(listener);
	}

	close() {
		this.closed = true;
	}

	emit(type: string, data?: string) {
		const event = data === undefined ? new Event(type) : new MessageEvent(type, { data });
		for (const listener of this.listeners.get(type) ?? []) listener(event);
	}
}

describe("realtime subscriptions", () => {
	afterEach(() => {
		vi.unstubAllGlobals();
		EventSourceFake.instance = null;
	});

	it("keeps the bootstrap stream available as the rollout fallback", () => {
		vi.stubGlobal("EventSource", EventSourceFake);
		const onRevision = vi.fn();
		const unsubscribe = subscribeBootstrapEvents(onRevision);
		const events = EventSourceFake.instance;
		expect(events?.url).toBe("/api/v1/events/bootstrap");
		expect(events?.withCredentials).toBe(true);

		events?.emit("bootstrap", '{"revision":"12"}');
		events?.emit("bootstrap", "not-json");
		expect(onRevision).toHaveBeenCalledOnce();
		expect(onRevision).toHaveBeenCalledWith("12");

		unsubscribe();
		expect(events?.closed).toBe(true);
	});

	it("routes sync, heartbeat, reset, and connection events", () => {
		vi.stubGlobal("EventSource", EventSourceFake);
		const handlers = {
			onDelta: vi.fn(),
			onHeartbeat: vi.fn(),
			onReset: vi.fn(),
			onConnectionChange: vi.fn()
		};
		const unsubscribe = subscribeSyncEvents("10", handlers);
		const events = EventSourceFake.instance;
		expect(events?.url).toBe("/api/v1/events/sync?since=10");
		expect(events?.withCredentials).toBe(true);

		events?.emit("open");
		events?.emit("sync", '{"checkpoint":"11","actions":[],"hasMore":false}');
		events?.emit("sync", "not-json");
		events?.emit("heartbeat", '{"checkpoint":"12"}');
		events?.emit("reset", '{"reason":"checkpointTooOld","checkpoint":"13"}');
		events?.emit("error");

		expect(handlers.onDelta).toHaveBeenCalledOnce();
		expect(handlers.onDelta).toHaveBeenCalledWith({ checkpoint: "11", actions: [], hasMore: false });
		expect(handlers.onHeartbeat).toHaveBeenCalledWith("12");
		expect(handlers.onReset).toHaveBeenCalledWith("checkpointTooOld", "13");
		expect(handlers.onConnectionChange).toHaveBeenNthCalledWith(1, true);
		expect(handlers.onConnectionChange).toHaveBeenNthCalledWith(2, false);

		unsubscribe();
		expect(events?.closed).toBe(true);
	});
});
