import * as AlertDialogPrimitive from "@radix-ui/react-alert-dialog";
import * as DialogPrimitive from "@radix-ui/react-dialog";
import { X } from "lucide-react";
import type { ReactNode } from "react";
import { Button } from "./Button";
import { IconButton } from "./IconButton";

export interface DialogProps {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	title: string;
	description?: string | undefined;
	children: ReactNode;
	footer?: ReactNode | undefined;
	trigger?: ReactNode | undefined;
}

function DialogHeader({ title, description }: Pick<DialogProps, "title" | "description">) {
	return (
		<div>
			<DialogPrimitive.Title className="md-dialog__title">{title}</DialogPrimitive.Title>
			{description ? <DialogPrimitive.Description className="md-dialog__description">{description}</DialogPrimitive.Description> : null}
		</div>
	);
}

export function Dialog({ open, onOpenChange, title, description, children, footer, trigger }: DialogProps) {
	return (
		<DialogPrimitive.Root open={open} onOpenChange={onOpenChange}>
			{trigger ? <DialogPrimitive.Trigger asChild>{trigger}</DialogPrimitive.Trigger> : null}
			<DialogPrimitive.Portal>
				<DialogPrimitive.Overlay className="md-overlay" />
				<DialogPrimitive.Content className="md-dialog">
					<header className="md-dialog__header">
						<DialogHeader title={title} description={description} />
						<DialogPrimitive.Close asChild>
							<IconButton label="Close dialog" icon={<X size="1.125rem" aria-hidden="true" />} />
						</DialogPrimitive.Close>
					</header>
					<div className="md-dialog__body">{children}</div>
					{footer ? <footer className="md-dialog__footer">{footer}</footer> : null}
				</DialogPrimitive.Content>
			</DialogPrimitive.Portal>
		</DialogPrimitive.Root>
	);
}

export type DrawerProps = DialogProps;

export function Drawer({ open, onOpenChange, title, description, children, footer, trigger }: DrawerProps) {
	return (
		<DialogPrimitive.Root open={open} onOpenChange={onOpenChange}>
			{trigger ? <DialogPrimitive.Trigger asChild>{trigger}</DialogPrimitive.Trigger> : null}
			<DialogPrimitive.Portal>
				<DialogPrimitive.Overlay className="md-overlay" />
				<DialogPrimitive.Content className="md-drawer">
					<header className="md-dialog__header">
						<DialogHeader title={title} description={description} />
						<DialogPrimitive.Close asChild>
							<IconButton label="Close drawer" icon={<X size="1.125rem" aria-hidden="true" />} />
						</DialogPrimitive.Close>
					</header>
					<div className="md-dialog__body">{children}</div>
					{footer ? <footer className="md-dialog__footer">{footer}</footer> : null}
				</DialogPrimitive.Content>
			</DialogPrimitive.Portal>
		</DialogPrimitive.Root>
	);
}

export interface ConfirmDialogProps {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	title: string;
	description: string;
	confirmLabel: string;
	/** Defaults to "Cancel"; set it when the surrounding UI is not in English. */
	cancelLabel?: string;
	onConfirm: () => void;
	busy?: boolean;
	destructive?: boolean;
}

export function ConfirmDialog({
	open,
	onOpenChange,
	title,
	description,
	confirmLabel,
	cancelLabel = "Cancel",
	onConfirm,
	busy = false,
	destructive = false
}: ConfirmDialogProps) {
	return (
		<AlertDialogPrimitive.Root open={open} onOpenChange={onOpenChange}>
			<AlertDialogPrimitive.Portal>
				<AlertDialogPrimitive.Overlay className="md-overlay" />
				<AlertDialogPrimitive.Content className="md-dialog md-alert-dialog">
					<div className="md-dialog__header">
						<div>
							<AlertDialogPrimitive.Title className="md-dialog__title">{title}</AlertDialogPrimitive.Title>
							<AlertDialogPrimitive.Description className="md-dialog__description">{description}</AlertDialogPrimitive.Description>
						</div>
					</div>
					<div className="md-dialog__footer">
						<AlertDialogPrimitive.Cancel asChild>
							<Button variant="text">{cancelLabel}</Button>
						</AlertDialogPrimitive.Cancel>
						<AlertDialogPrimitive.Action asChild>
							<Button variant="text" tone={destructive ? "error" : "primary"} loading={busy} onClick={onConfirm}>
								{confirmLabel}
							</Button>
						</AlertDialogPrimitive.Action>
					</div>
				</AlertDialogPrimitive.Content>
			</AlertDialogPrimitive.Portal>
		</AlertDialogPrimitive.Root>
	);
}
