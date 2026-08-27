import * as RadixTabs from "@radix-ui/react-tabs";
import { useCallback, useLayoutEffect, useRef, useState, type ReactNode } from "react";

export interface TabItem {
	value: string;
	label: string;
	content: ReactNode;
	disabled?: boolean;
}

export interface TabsProps {
	items: TabItem[];
	defaultValue?: string | undefined;
	value?: string | undefined;
	onValueChange?: ((value: string) => void) | undefined;
	label: string;
}

/**
 * Material Design 3 primary tabs.
 *
 * The active indicator slides. Radix owns selection; this component tracks the
 * active value so it can measure the matching trigger and publish
 * --md-tab-indicator-{left,width} on the list, which the ::after reads.
 */
export function Tabs({ items, defaultValue, value, onValueChange, label }: TabsProps) {
	const fallback = items[0]?.value;
	const [internalValue, setInternalValue] = useState(() => defaultValue ?? fallback ?? "");
	const activeValue = value ?? internalValue;
	const listRef = useRef<HTMLDivElement>(null);

	const measure = useCallback(() => {
		const list = listRef.current;
		if (!list) return;
		const active = list.querySelector<HTMLElement>('[data-state="active"]');
		if (!active) return;
		list.style.setProperty("--md-tab-indicator-left", `${active.offsetLeft}px`);
		list.style.setProperty("--md-tab-indicator-width", `${active.offsetWidth}px`);
	}, []);

	useLayoutEffect(() => {
		measure();
		const list = listRef.current;
		// jsdom has no ResizeObserver in some setups; the indicator simply stops
		// tracking resizes there rather than throwing.
		if (!list || typeof ResizeObserver === "undefined") return;
		const observer = new ResizeObserver(measure);
		observer.observe(list);
		return () => observer.disconnect();
	}, [measure, activeValue, items]);

	const handleValueChange = (next: string) => {
		if (value === undefined) setInternalValue(next);
		onValueChange?.(next);
	};

	return (
		<RadixTabs.Root value={activeValue} onValueChange={handleValueChange}>
			<RadixTabs.List className="md-tabs__list" aria-label={label} ref={listRef}>
				{items.map((item) => (
					<RadixTabs.Trigger className="md-tab md-state-layer" key={item.value} value={item.value} disabled={item.disabled}>
						{item.label}
					</RadixTabs.Trigger>
				))}
			</RadixTabs.List>
			{items.map((item) => (
				<RadixTabs.Content className="md-tabs__content" key={item.value} value={item.value}>
					{item.content}
				</RadixTabs.Content>
			))}
		</RadixTabs.Root>
	);
}
