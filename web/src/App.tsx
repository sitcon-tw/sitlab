import { SessionProvider } from "@/features/auth/SessionProvider";
import { ThemeProvider } from "@/shared/theme/ThemeProvider";
import { ToastProvider } from "@/shared/toast/ToastProvider";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider } from "react-router-dom";
import { router } from "./routes/router";

const queryClient = new QueryClient({
	defaultOptions: {
		queries: { retry: 1, refetchOnWindowFocus: false },
		mutations: { retry: false }
	}
});

export function App() {
	return (
		<ThemeProvider>
			<QueryClientProvider client={queryClient}>
				<SessionProvider>
					<ToastProvider>
						<RouterProvider router={router} />
					</ToastProvider>
				</SessionProvider>
			</QueryClientProvider>
		</ThemeProvider>
	);
}
