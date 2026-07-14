import { ToastRegion, type ToastMessage, type ToastTone } from "@project-template/ui";
import { useCallback, useMemo, useState, type ReactNode } from "react";
import { ToastContext } from "./toastContext";

export function ToastProvider({ children }: { children: ReactNode }) {
	const [messages, setMessages] = useState<ToastMessage[]>([]);
	const notify = useCallback((title: string, options: { description?: string; tone?: ToastTone } = {}) => {
		const id = crypto.randomUUID();
		const message: ToastMessage = { id, title, ...options };
		setMessages((current) => [...current, message]);
		window.setTimeout(() => setMessages((current) => current.filter((item) => item.id !== id)), 4500);
	}, []);
	const value = useMemo(() => ({ notify }), [notify]);
	return (
		<ToastContext.Provider value={value}>
			{children}
			<ToastRegion messages={messages} />
		</ToastContext.Provider>
	);
}
