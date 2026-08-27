import type { ReactNode } from "react";
import { classNames } from "../lib/classNames";

export type ToastTone = "info" | "success" | "danger";
export interface ToastMessage {
	id: string;
	title: string;
	description?: string;
	tone?: ToastTone;
	action?: ReactNode;
}
export interface ToastRegionProps {
	messages: ToastMessage[];
}

export function ToastRegion({ messages }: ToastRegionProps) {
	return (
		<div className="md-toast-region" aria-label="Notifications">
			{messages.map((message) => (
				<div className={classNames("md-toast", `md-toast--${message.tone ?? "info"}`)} key={message.id} role={message.tone === "danger" ? "alert" : "status"}>
					<h2 className="md-toast__title">{message.title}</h2>
					{message.description ? <p className="md-toast__description">{message.description}</p> : null}
					{message.action}
				</div>
			))}
		</div>
	);
}
