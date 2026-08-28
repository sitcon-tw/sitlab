import * as DropdownMenuPrimitive from "@radix-ui/react-dropdown-menu";
import type { ReactNode } from "react";
import { classNames } from "../lib/classNames";
import { useRipple } from "../lib/useRipple";

export interface MenuProps {
	trigger: ReactNode;
	children: ReactNode;
	label: string;
	align?: "start" | "center" | "end";
	open?: boolean | undefined;
	onOpenChange?: ((open: boolean) => void) | undefined;
	className?: string | undefined;
	portalled?: boolean | undefined;
}

/**
 * Accessible product menu.
 *
 * Replaces the <details>-based popovers the board used to hand-roll, which had
 * no focus trap, no roving tabindex, and no Escape handling — they were closed
 * imperatively by writing `ref.current.open = false`.
 */
export function Menu({ trigger, children, label, align = "start", open, onOpenChange, className, portalled = true }: MenuProps) {
	const rootProps = {
		...(open !== undefined ? { open } : {}),
		...(onOpenChange ? { onOpenChange } : {})
	};
	const content = (
		<DropdownMenuPrimitive.Content className={classNames("md-menu", className)} align={align} sideOffset={4} aria-label={label} collisionPadding={8}>
			{children}
		</DropdownMenuPrimitive.Content>
	);
	return (
		<DropdownMenuPrimitive.Root {...rootProps}>
			<DropdownMenuPrimitive.Trigger asChild>{trigger}</DropdownMenuPrimitive.Trigger>
			{portalled ? <DropdownMenuPrimitive.Portal>{content}</DropdownMenuPrimitive.Portal> : content}
		</DropdownMenuPrimitive.Root>
	);
}

export interface MenuItemProps {
	children: ReactNode;
	onSelect?: (() => void) | undefined;
	leading?: ReactNode;
	trailing?: ReactNode;
	disabled?: boolean | undefined;
	/** Renders the row as a checkable option and reflects its state. */
	selected?: boolean | undefined;
	className?: string | undefined;
}

export function MenuItem({ children, onSelect, leading, trailing, disabled, selected, className }: MenuItemProps) {
	const ripple = useRipple();
	const content = (
		<>
			{leading ? <span className="md-menu-item__leading">{leading}</span> : null}
			<span className="md-menu-item__label">{children}</span>
			{trailing ? <span className="md-menu-item__trailing">{trailing}</span> : null}
			{ripple.rippleNodes}
		</>
	);
	const shared = {
		className: classNames("md-menu-item", "md-state-layer", className),
		onPointerDown: ripple.onPointerDown,
		...(disabled === undefined ? {} : { disabled }),
		...(onSelect ? { onSelect } : {})
	};
	if (selected === undefined) return <DropdownMenuPrimitive.Item {...shared}>{content}</DropdownMenuPrimitive.Item>;
	return (
		<DropdownMenuPrimitive.CheckboxItem {...shared} checked={selected}>
			{content}
		</DropdownMenuPrimitive.CheckboxItem>
	);
}

export function MenuDivider() {
	return <DropdownMenuPrimitive.Separator className="md-divider" />;
}
