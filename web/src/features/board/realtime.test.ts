import { afterEach, describe, expect, it, vi } from "vitest";
import { subscribeBootstrapEvents } from "./realtime";

class EventSourceFake {
	static instance: EventSourceFake | null = null;
	readonly url: string;
	readonly withCredentials: boolean;
	closed = false;
	private listener: EventListener | null = null;

	constructor(url: string | URL, options?: EventSourceInit) {
		this.url = String(url);
		this.withCredentials = options?.withCredentials ?? false;
		EventSourceFake.instance = this;
	}

	addEventListener(_type: string, listener: EventListenerOrEventListenerObject) {
		this.listener = typeof listener === "function" ? listener : listener.handleEvent.bind(listener);
	}

	removeEventListener() {
		this.listener = null;
	}

	close() {
		this.closed = true;
	}

	emit(data: string) {
		this.listener?.(new MessageEvent("bootstrap", { data }));
	}
}

describe("bootstrap realtime subscription", () => {
	afterEach(() => {
		vi.unstubAllGlobals();
		EventSourceFake.instance = null;
	});

	it("uses credentials, emits valid revisions, and closes cleanly", () => {
		vi.stubGlobal("EventSource", EventSourceFake);
		const onRevision = vi.fn();
		const unsubscribe = subscribeBootstrapEvents(onRevision);
		const events = EventSourceFake.instance;
		expect(events?.url).toBe("/api/v1/events/bootstrap");
		expect(events?.withCredentials).toBe(true);

		events?.emit('{"revision":"12"}');
		events?.emit("not-json");
		expect(onRevision).toHaveBeenCalledOnce();
		expect(onRevision).toHaveBeenCalledWith("12");

		unsubscribe();
		expect(events?.closed).toBe(true);
	});
});
