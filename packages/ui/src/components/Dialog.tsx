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
			<DialogPrimitive.Title className="pt-dialog__title">{title}</DialogPrimitive.Title>
			{description ? <DialogPrimitive.Description className="pt-dialog__description">{description}</DialogPrimitive.Description> : null}
		</div>
	);
}

export function Dialog({ open, onOpenChange, title, description, children, footer, trigger }: DialogProps) {
	return (
		<DialogPrimitive.Root open={open} onOpenChange={onOpenChange}>
			{trigger ? <DialogPrimitive.Trigger asChild>{trigger}</DialogPrimitive.Trigger> : null}
			<DialogPrimitive.Portal>
				<DialogPrimitive.Overlay className="pt-overlay" />
				<DialogPrimitive.Content className="pt-dialog">
					<header className="pt-dialog__header">
						<DialogHeader title={title} description={description} />
						<DialogPrimitive.Close asChild>
							<IconButton label="Close dialog" icon={<X size="1.125rem" aria-hidden="true" />} />
						</DialogPrimitive.Close>
					</header>
					<div className="pt-dialog__body">{children}</div>
					{footer ? <footer className="pt-dialog__footer">{footer}</footer> : null}
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
				<DialogPrimitive.Overlay className="pt-overlay" />
				<DialogPrimitive.Content className="pt-drawer">
					<header className="pt-dialog__header">
						<DialogHeader title={title} description={description} />
						<DialogPrimitive.Close asChild>
							<IconButton label="Close drawer" icon={<X size="1.125rem" aria-hidden="true" />} />
						</DialogPrimitive.Close>
					</header>
					<div className="pt-dialog__body">{children}</div>
					{footer ? <footer className="pt-dialog__footer">{footer}</footer> : null}
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
	onConfirm: () => void;
	busy?: boolean;
	destructive?: boolean;
}

export function ConfirmDialog({ open, onOpenChange, title, description, confirmLabel, onConfirm, busy = false, destructive = false }: ConfirmDialogProps) {
	return (
		<AlertDialogPrimitive.Root open={open} onOpenChange={onOpenChange}>
			<AlertDialogPrimitive.Portal>
				<AlertDialogPrimitive.Overlay className="pt-overlay" />
				<AlertDialogPrimitive.Content className="pt-dialog pt-alert-dialog">
					<div className="pt-dialog__header">
						<div>
							<AlertDialogPrimitive.Title className="pt-dialog__title">{title}</AlertDialogPrimitive.Title>
							<AlertDialogPrimitive.Description className="pt-dialog__description">{description}</AlertDialogPrimitive.Description>
						</div>
					</div>
					<div className="pt-dialog__footer">
						<AlertDialogPrimitive.Cancel asChild>
							<Button variant="secondary">Cancel</Button>
						</AlertDialogPrimitive.Cancel>
						<AlertDialogPrimitive.Action asChild>
							<Button variant={destructive ? "danger" : "primary"} loading={busy} onClick={onConfirm}>
								{confirmLabel}
							</Button>
						</AlertDialogPrimitive.Action>
					</div>
				</AlertDialogPrimitive.Content>
			</AlertDialogPrimitive.Portal>
		</AlertDialogPrimitive.Root>
	);
}
