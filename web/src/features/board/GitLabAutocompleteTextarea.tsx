import { TextAreaField, type TextAreaFieldProps } from "@project-template/ui";
import { useQuery } from "@tanstack/react-query";
import { useCallback, useDeferredValue, useEffect, useId, useMemo, useRef, useState, type ChangeEvent, type KeyboardEvent } from "react";
import { createPortal } from "react-dom";
import { listQuickActions, listQuickActionSuggestions } from "./boardApi";
import styles from "./GitLabAutocompleteTextarea.module.css";
import type { Bootstrap } from "./model";
import { autocompleteAt, suggestionRequestAt } from "./quickActionAutocomplete";
import { measureTextareaCaret, placeCaretPopover, type CaretPopoverPosition } from "./textareaCaret";
import { useProjectLabels } from "./useProjectLabels";

type Props = Omit<TextAreaFieldProps, "value" | "onChange"> & {
	bootstrap: Bootstrap;
	issueIid?: number;
	value: string;
	onValueChange: (value: string) => void;
};

export function GitLabAutocompleteTextarea({
	bootstrap,
	issueIid,
	value,
	onValueChange,
	onKeyDown,
	onClick,
	onKeyUp,
	onSelect,
	onScroll,
	onFocus,
	onBlur,
	...props
}: Props) {
	const inputRef = useRef<HTMLTextAreaElement>(null);
	const [cursor, setCursor] = useState(value.length);
	const [focused, setFocused] = useState(false);
	const [activeIndex, setActiveIndex] = useState(0);
	const [dismissed, setDismissed] = useState<string | null>(null);
	const [menuPosition, setMenuPosition] = useState<CaretPopoverPosition | null>(null);
	const menuId = useId();
	const commandsQuery = useQuery({ queryKey: ["sitcon", "quick-actions", issueIid ?? "create"], queryFn: () => listQuickActions(issueIid), staleTime: 60_000 });
	const labelsQuery = useProjectLabels();
	const suggestionRequest = useMemo(() => suggestionRequestAt(value, cursor, commandsQuery.data ?? []), [value, cursor, commandsQuery.data]);
	const deferredRequest = useDeferredValue(suggestionRequest);
	const suggestionsQuery = useQuery({
		queryKey: ["sitcon", "quick-action-suggestions", issueIid ?? "create", deferredRequest?.kind, deferredRequest?.query],
		queryFn: () => listQuickActionSuggestions(deferredRequest!.kind, deferredRequest!.query, issueIid),
		enabled: focused && Boolean(deferredRequest),
		staleTime: 30_000
	});
	const result = useMemo(
		() =>
			autocompleteAt(value, cursor, {
				bootstrap,
				commands: commandsQuery.data ?? [],
				labels: labelsQuery.data ?? [],
				suggestions: suggestionsQuery.data ?? []
			}),
		[value, cursor, bootstrap, commandsQuery.data, labelsQuery.data, suggestionsQuery.data]
	);
	const signature = result ? `${result.start}:${result.end}:${value.slice(result.start, result.end)}` : "";
	const open = focused && Boolean(result?.items.length) && dismissed !== signature;
	const selectedIndex = result?.items.length ? Math.min(activeIndex, result.items.length - 1) : 0;

	const positionMenu = useCallback((textarea = inputRef.current) => {
		if (!textarea) return;
		setMenuPosition(
			placeCaretPopover(measureTextareaCaret(textarea), textarea.getBoundingClientRect().width, {
				width: window.innerWidth,
				height: window.innerHeight
			})
		);
	}, []);
	const updateCursor = (textarea: HTMLTextAreaElement) => {
		setCursor(textarea.selectionStart);
		setActiveIndex(0);
		positionMenu(textarea);
	};
	const change = (event: ChangeEvent<HTMLTextAreaElement>) => {
		const textarea = event.currentTarget;
		onValueChange(textarea.value);
		setCursor(textarea.selectionStart);
		setActiveIndex(0);
		setDismissed(null);
		positionMenu(textarea);
	};
	const choose = (index: number) => {
		const item = result?.items[index];
		if (!item || !result) return;
		const next = value.slice(0, result.start) + item.insertText + value.slice(result.end);
		const nextCursor = result.start + item.insertText.length;
		onValueChange(next);
		setCursor(nextCursor);
		setDismissed(null);
		requestAnimationFrame(() => {
			inputRef.current?.focus();
			inputRef.current?.setSelectionRange(nextCursor, nextCursor);
			positionMenu();
		});
	};
	useEffect(() => {
		if (!open) return;
		let frame = 0;
		const reposition = () => {
			cancelAnimationFrame(frame);
			frame = requestAnimationFrame(() => positionMenu());
		};
		window.addEventListener("resize", reposition);
		document.addEventListener("scroll", reposition, true);
		return () => {
			cancelAnimationFrame(frame);
			window.removeEventListener("resize", reposition);
			document.removeEventListener("scroll", reposition, true);
		};
	}, [open, positionMenu]);
	const keyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
		if (open && result) {
			if (event.key === "ArrowDown") {
				event.preventDefault();
				setActiveIndex((index) => (index + 1) % result.items.length);
				return;
			}
			if (event.key === "ArrowUp") {
				event.preventDefault();
				setActiveIndex((index) => (index - 1 + result.items.length) % result.items.length);
				return;
			}
			if ((event.key === "Enter" && !event.metaKey && !event.ctrlKey) || event.key === "Tab") {
				event.preventDefault();
				choose(selectedIndex);
				return;
			}
			if (event.key === "Escape") {
				event.preventDefault();
				setDismissed(signature);
				return;
			}
		}
		onKeyDown?.(event);
	};

	return (
		<div>
			<TextAreaField
				{...props}
				ref={inputRef}
				value={value}
				onChange={change}
				onKeyDown={keyDown}
				onClick={(event) => {
					updateCursor(event.currentTarget);
					onClick?.(event);
				}}
				onKeyUp={(event) => {
					updateCursor(event.currentTarget);
					onKeyUp?.(event);
				}}
				onSelect={(event) => {
					updateCursor(event.currentTarget);
					onSelect?.(event);
				}}
				onScroll={(event) => {
					positionMenu(event.currentTarget);
					onScroll?.(event);
				}}
				onFocus={(event) => {
					setFocused(true);
					positionMenu(event.currentTarget);
					onFocus?.(event);
				}}
				onBlur={(event) => {
					setFocused(false);
					onBlur?.(event);
				}}
				role="combobox"
				aria-autocomplete="list"
				aria-expanded={open}
				aria-controls={open ? menuId : undefined}
				aria-activedescendant={open ? `${menuId}-${selectedIndex}` : undefined}
			/>
			{open && result && menuPosition
				? createPortal(
						<div
							id={menuId}
							className={`md-menu ${styles.menu}`}
							role="listbox"
							aria-label="GitLab autocomplete"
							style={{
								left: menuPosition.left,
								top: menuPosition.top,
								bottom: menuPosition.bottom,
								width: menuPosition.width,
								maxHeight: menuPosition.maxHeight
							}}
						>
							{result.items.map((item, index) => (
								<button
									type="button"
									role="option"
									id={`${menuId}-${index}`}
									aria-selected={index === selectedIndex}
									className="md-menu-item md-state-layer"
									key={item.key}
									onMouseDown={(event) => event.preventDefault()}
									onClick={() => choose(index)}
								>
									{index === selectedIndex ? <span className={styles.selectionMarker} aria-hidden="true" /> : null}
									<code className="md-menu-item__label">{item.label}</code>
									<span className="md-typescale-body-small">{item.detail}</span>
								</button>
							))}
						</div>,
						document.body
					)
				: null}
		</div>
	);
}
