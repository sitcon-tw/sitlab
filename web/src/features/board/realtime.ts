import { apiBaseUrl } from "@/shared/api/client";

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
