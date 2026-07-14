import type { ToastTone } from "@project-template/ui";
import { createContext, useContext } from "react";

export interface ToastContextValue {
	notify: (title: string, options?: { description?: string; tone?: ToastTone }) => void;
}
export const ToastContext = createContext<ToastContextValue | null>(null);

export function useToast() {
	const value = useContext(ToastContext);
	if (!value) throw new Error("useToast must be used inside ToastProvider");
	return value;
}
