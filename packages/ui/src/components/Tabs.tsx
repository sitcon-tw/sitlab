import * as RadixTabs from "@radix-ui/react-tabs";
import type { ReactNode } from "react";

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

export function Tabs({ items, defaultValue, value, onValueChange, label }: TabsProps) {
	const fallback = items[0]?.value;
	const initialValue = defaultValue ?? fallback;
	const rootProps = {
		...(value !== undefined ? { value } : {}),
		...(onValueChange ? { onValueChange } : {}),
		...(value === undefined && initialValue !== undefined ? { defaultValue: initialValue } : {})
	};
	return (
		<RadixTabs.Root {...rootProps}>
			<RadixTabs.List className="pt-tabs__list" aria-label={label}>
				{items.map((item) => (
					<RadixTabs.Trigger className="pt-tab" key={item.value} value={item.value} disabled={item.disabled}>
						{item.label}
					</RadixTabs.Trigger>
				))}
			</RadixTabs.List>
			{items.map((item) => (
				<RadixTabs.Content className="pt-tabs__content" key={item.value} value={item.value}>
					{item.content}
				</RadixTabs.Content>
			))}
		</RadixTabs.Root>
	);
}
