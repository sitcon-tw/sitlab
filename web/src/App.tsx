import { LoginPage } from "@/features/auth/LoginPage";
import { BoardPage } from "@/features/board/BoardPage";
import { refreshBootstrap } from "@/features/board/bootstrap";
import type { Bootstrap } from "@/features/board/model";
import { OnboardingPage } from "@/features/onboarding/OnboardingPage";
import { QueryClient, QueryClientProvider, useQuery, useQueryClient } from "@tanstack/react-query";

const bootstrapKey = ["sitcon", "bootstrap"] as const;
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
	const bootstrapQuery = useQuery({
		queryKey: bootstrapKey,
		queryFn: refreshBootstrap,
		initialData: initialBootstrap,
		refetchInterval: demo ? false : 30_000,
		staleTime: demo ? Infinity : 25_000
	});
	const bootstrap = bootstrapQuery.data;
	const updateBootstrap = (update: (current: Bootstrap) => Bootstrap) => {
		client.setQueryData<Bootstrap>(bootstrapKey, (current) => update(current ?? bootstrap));
	};

	if (!bootstrap.preferences.confirmedAt) {
		return <OnboardingPage bootstrap={bootstrap} updateBootstrap={updateBootstrap} />;
	}

	return <BoardPage bootstrap={bootstrap} updateBootstrap={updateBootstrap} backgroundOffline={bootstrapQuery.isRefetchError} />;
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
