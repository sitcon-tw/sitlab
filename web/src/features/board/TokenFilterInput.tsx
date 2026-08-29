import { Chip, IconButton, StaticChip, TextField } from "@project-template/ui";
import { X } from "lucide-react";
import { useId, useMemo, useRef, useState } from "react";
import styles from "./BoardPage.module.css";
import { activeMembers, filterDirectoryMembers, memberById, normalizePickerQuery, type Bootstrap, type ProjectLabel } from "./model";
import { TagSwatch } from "./TagSwatch";

type SuggestionCategory = "team" | "member" | "label";

const tokenCategories: Array<{ kind: SuggestionCategory; label: string; hint: string; aliases: string[] }> = [
	{ kind: "team", label: "組別", hint: "篩選單一組別", aliases: ["組別", "team"] },
	{ kind: "member", label: "負責人", hint: "可加入多位負責人", aliases: ["負責人", "member", "assignee"] },
	{ kind: "label", label: "Label", hint: "可加入多個 Labels", aliases: ["label", "標籤"] }
];

const emptyNotes: Record<SuggestionCategory, string> = { team: "找不到組別", member: "找不到負責人", label: "找不到 Label" };

type FilterToken =
	{ kind: "team"; key: string; text: string } | { kind: "member"; id: number; text: string } | { kind: "label"; name: string; meta: ProjectLabel | undefined };

type SuggestionOption =
	| { kind: "category"; category: SuggestionCategory; label: string; hint: string }
	| { kind: "team"; key: string; name: string }
	| { kind: "member"; id: number; displayName: string; username: string }
	| { kind: "label"; label: ProjectLabel };

export interface TokenFilterInputProps {
	bootstrap: Bootstrap;
	query: string;
	teamKey: string;
	memberIds: number[];
	labels: string[];
	/** The same unfiltered catalog LabelOptions uses, so tokens mirror the picker. */
	projectLabels: ProjectLabel[];
	labelsLoading: boolean;
	onQueryChange: (query: string) => void;
	onTeamChange: (teamKey: string) => void;
	onMemberIdsChange: (memberIds: number[]) => void;
	onLabelsChange: (labels: string[]) => void;
	className?: string | undefined;
}

/**
 * GitLab-style token search: plain text stays the debounced card search, while
 * the suggestion popup inserts removable 組別/負責人/Label tokens. Tokens are a
 * pure view of the lifted filter state, so the three pickers and this field can
 * never disagree. The popup follows the quick-action composer's combobox
 * pattern (design.md deviation 12); the native radio/checkbox selection
 * surfaces stay in the pickers.
 */
export function TokenFilterInput({
	bootstrap,
	query,
	teamKey,
	memberIds,
	labels,
	projectLabels,
	labelsLoading,
	onQueryChange,
	onTeamChange,
	onMemberIdsChange,
	onLabelsChange,
	className
}: TokenFilterInputProps) {
	const [pending, setPending] = useState<SuggestionCategory | null>(null);
	const [valueQuery, setValueQuery] = useState("");
	const [focused, setFocused] = useState(false);
	const [dismissed, setDismissed] = useState(false);
	const [activeIndex, setActiveIndex] = useState(0);
	const menuId = useId();
	const pendingHintId = useId();
	const rootRef = useRef<HTMLDivElement>(null);
	const focusInput = () => document.getElementById("board-card-search")?.focus();

	const teams = useMemo(() => bootstrap.teams.filter((team) => team.active).sort((a, b) => a.sortOrder - b.sortOrder), [bootstrap]);

	const tokens = useMemo<FilterToken[]>(() => {
		const list: FilterToken[] = [];
		const team = teams.find((entry) => entry.key === teamKey);
		if (team) list.push({ kind: "team", key: team.key, text: team.name });
		for (const id of memberIds) {
			const member = memberById(bootstrap, id);
			if (member) list.push({ kind: "member", id, text: member.displayName });
		}
		const metadata = new Map(projectLabels.map((label) => [label.name, label]));
		for (const name of labels) list.push({ kind: "label", name, meta: metadata.get(name) });
		return list;
	}, [teams, teamKey, memberIds, labels, projectLabels, bootstrap]);

	const removeToken = (token: FilterToken) => {
		if (token.kind === "team") onTeamChange("");
		else if (token.kind === "member") onMemberIdsChange(memberIds.filter((id) => id !== token.id));
		else onLabelsChange(labels.filter((name) => name !== token.name));
		focusInput();
	};

	const options = useMemo<SuggestionOption[]>(() => {
		if (pending === "team") {
			const normalized = normalizePickerQuery(valueQuery);
			return teams
				.filter((team) => team.key !== teamKey)
				.filter((team) => !normalized || normalizePickerQuery(`${team.name} ${team.key} ${team.titlePrefix}`).includes(normalized))
				.map((team) => ({ kind: "team" as const, key: team.key, name: team.name }));
		}
		if (pending === "member") {
			return filterDirectoryMembers(activeMembers(bootstrap), valueQuery)
				.filter((member) => !memberIds.includes(member.gitLabUserId))
				.map((member) => ({ kind: "member" as const, id: member.gitLabUserId, displayName: member.displayName, username: member.username }));
		}
		if (pending === "label") {
			const normalized = normalizePickerQuery(valueQuery);
			return projectLabels
				.filter((label) => !labels.includes(label.name))
				.filter((label) => !normalized || normalizePickerQuery(`${label.name} ${label.description ?? ""}`).includes(normalized))
				.map((label) => ({ kind: "label" as const, label }));
		}
		const normalized = normalizePickerQuery(query);
		return tokenCategories
			.filter((category) => !normalized || category.aliases.some((alias) => normalizePickerQuery(alias).startsWith(normalized)))
			.map((category) => ({ kind: "category" as const, category: category.kind, label: category.label, hint: category.hint }));
	}, [pending, valueQuery, query, teams, teamKey, bootstrap, memberIds, projectLabels, labels]);

	// Plain-text search typing keeps the popup closed unless the text still
	// prefixes a category alias, so searching stays visually quiet.
	const open = focused && (pending !== null || (!dismissed && options.length > 0));
	const clampedIndex = Math.min(activeIndex, Math.max(options.length - 1, 0));
	const pendingCategory = pending ? tokenCategories.find((category) => category.kind === pending) : undefined;

	const choose = (index: number) => {
		const option = options[index];
		if (!option) return;
		if (option.kind === "category") {
			// The typed alias prefix was a token request, not search text.
			onQueryChange("");
			setPending(option.category);
		} else {
			if (option.kind === "team") onTeamChange(option.key);
			else if (option.kind === "member") onMemberIdsChange([...memberIds, option.id]);
			else onLabelsChange([...labels, option.label.name]);
			setPending(null);
		}
		setValueQuery("");
		setActiveIndex(0);
	};

	const changeText = (next: string) => {
		setDismissed(false);
		setActiveIndex(0);
		if (pending) setValueQuery(next);
		else onQueryChange(next);
	};

	const cancelPending = () => {
		setPending(null);
		setValueQuery("");
		setActiveIndex(0);
	};

	const onKeyDown = (event: React.KeyboardEvent<HTMLInputElement>) => {
		if (event.key === "ArrowDown") {
			event.preventDefault();
			if (!open) {
				setDismissed(false);
				setActiveIndex(0);
				return;
			}
			if (options.length) setActiveIndex((clampedIndex + 1) % options.length);
			return;
		}
		if (event.key === "ArrowUp") {
			if (!open || !options.length) return;
			event.preventDefault();
			setActiveIndex((clampedIndex - 1 + options.length) % options.length);
			return;
		}
		if (event.key === "Enter") {
			if (!open || !options.length) return;
			event.preventDefault();
			choose(clampedIndex);
			return;
		}
		if (event.key === "Escape") {
			// Ladder: cancel a pending token, else close the popup, else keep the
			// existing clear-and-blur contract.
			event.stopPropagation();
			if (pending !== null) {
				cancelPending();
				return;
			}
			if (open) {
				setDismissed(true);
				return;
			}
			onQueryChange("");
			event.currentTarget.blur();
			return;
		}
		if (event.key === "Backspace") {
			if (pending !== null && valueQuery === "") {
				event.preventDefault();
				cancelPending();
				return;
			}
			const input = event.currentTarget;
			const lastToken = tokens[tokens.length - 1];
			if (pending === null && input.selectionStart === 0 && input.selectionEnd === 0 && lastToken) {
				event.preventDefault();
				removeToken(lastToken);
			}
		}
	};

	const hasTokens = tokens.length > 0 || pending !== null;
	const leading = hasTokens ? (
		<>
			{tokens.map((token) => (
				<Chip
					key={token.kind === "team" ? `team:${token.key}` : token.kind === "member" ? `member:${token.id}` : `label:${token.name}`}
					variant="input"
					label={token.kind === "team" ? `組別：${token.text}` : token.kind === "member" ? `負責人：${token.text}` : token.name}
					leading={token.kind === "label" ? <TagSwatch label={token.meta} /> : undefined}
					tabIndex={-1}
					onClick={() => {
						setPending(token.kind);
						setValueQuery("");
						setActiveIndex(0);
						focusInput();
					}}
					onRemove={() => removeToken(token)}
					removeLabel={
						token.kind === "team" ? `移除組別篩選 ${token.text}` : token.kind === "member" ? `移除負責人篩選 ${token.text}` : `移除 Label 篩選 ${token.name}`
					}
					removeIcon={<X size="0.875rem" aria-hidden="true" />}
				/>
			))}
			{pendingCategory ? <StaticChip size="sm" variant="suggestion" label={`${pendingCategory.label}：`} /> : null}
		</>
	) : undefined;

	return (
		<div
			ref={rootRef}
			className={className}
			data-tokens={hasTokens || undefined}
			onBlur={(event) => {
				if (rootRef.current?.contains(event.relatedTarget as Node | null)) return;
				setFocused(false);
				cancelPending();
			}}
		>
			<TextField
				id="board-card-search"
				dense
				type="search"
				label="搜尋卡片"
				value={pending ? valueQuery : query}
				placeholder="標題、編號、人員或 Label"
				autoComplete="off"
				alwaysFloatLabel={hasTokens}
				leading={leading}
				role="combobox"
				aria-autocomplete="list"
				aria-expanded={open}
				aria-controls={open ? menuId : undefined}
				aria-activedescendant={open && options.length ? `${menuId}-${clampedIndex}` : undefined}
				aria-describedby={pendingCategory ? pendingHintId : undefined}
				onFocus={() => {
					setFocused(true);
					setDismissed(false);
				}}
				onChange={(event) => changeText(event.target.value)}
				onKeyDown={onKeyDown}
			/>
			{pendingCategory ? (
				<span id={pendingHintId} className={styles.srOnly}>
					正在選擇{pendingCategory.label}
				</span>
			) : null}
			{query && !pending ? <IconButton size="sm" label="清除卡片搜尋" icon={<X size="1rem" aria-hidden="true" />} onClick={() => onQueryChange("")} /> : null}
			{open ? (
				<div id={menuId} className={`md-menu ${styles.tokenMenu}`} role="listbox" aria-label={pendingCategory ? `選擇${pendingCategory.label}` : "篩選類別"}>
					{options.map((option, index) => (
						<button
							type="button"
							role="option"
							id={`${menuId}-${index}`}
							aria-selected={index === clampedIndex}
							className="md-menu-item md-state-layer"
							key={
								option.kind === "category" ? option.category : option.kind === "team" ? option.key : option.kind === "member" ? option.id : option.label.name
							}
							onMouseDown={(event) => event.preventDefault()}
							onClick={() => choose(index)}
						>
							{option.kind === "category" ? (
								<>
									<span className="md-menu-item__label">{option.label}</span>
									<span className="md-typescale-body-small">{option.hint}</span>
								</>
							) : option.kind === "team" ? (
								<span className="md-menu-item__label">{option.name}</span>
							) : option.kind === "member" ? (
								<>
									<span className="md-menu-item__label">{option.displayName}</span>
									<span className="md-typescale-body-small">@{option.username}</span>
								</>
							) : (
								<>
									<TagSwatch label={option.label} />
									<span className="md-menu-item__label">{option.label.name}</span>
								</>
							)}
						</button>
					))}
					{pending === "label" && labelsLoading ? (
						<p className={styles.tokenMenuNote} role="status">
							載入中...
						</p>
					) : null}
					{pending !== null && options.length === 0 && !labelsLoading ? <p className={styles.tokenMenuNote}>{emptyNotes[pending]}</p> : null}
				</div>
			) : null}
		</div>
	);
}
