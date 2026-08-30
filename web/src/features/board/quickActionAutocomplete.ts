import type { Bootstrap, ProjectLabel, QuickActionCommand, QuickActionSuggestion, QuickActionSuggestionKind } from "./model";

export type AutocompleteKind = "command" | "status" | "type" | QuickActionSuggestionKind;

export interface AutocompleteItem {
	key: string;
	kind: AutocompleteKind;
	label: string;
	detail: string;
	insertText: string;
}

export interface AutocompleteResult {
	start: number;
	end: number;
	items: AutocompleteItem[];
}

interface Context {
	bootstrap: Bootstrap;
	commands: QuickActionCommand[];
	labels: ProjectLabel[];
	suggestions?: QuickActionSuggestion[];
}

export interface SuggestionRequest {
	kind: QuickActionSuggestionKind;
	query: string;
}

const commandPattern = /^\/([A-Za-z_]*)$/;
const commandLinePattern = /^\/([A-Za-z_]+)(?:\s+(.*))?$/;

export function autocompleteAt(value: string, cursor: number, context: Context): AutocompleteResult | null {
	const safeCursor = Math.max(0, Math.min(cursor, value.length));
	const lineStart = value.lastIndexOf("\n", safeCursor - 1) + 1;
	const line = value.slice(lineStart, safeCursor);
	const commandMatch = commandPattern.exec(line);
	if (commandMatch) {
		const query = commandMatch[1]!.toLowerCase();
		const items = context.commands
			.filter((command) => command.name.toLowerCase().includes(query) || command.aliases.some((alias) => alias.toLowerCase().includes(query)))
			.sort((a, b) => commandScore(b, query) - commandScore(a, query) || a.name.localeCompare(b.name))
			.map(commandItem);
		return { start: lineStart, end: safeCursor, items };
	}

	const parsed = commandLinePattern.exec(line);
	const command = parsed ? findCommand(context.commands, parsed[1]!) : undefined;
	const args = parsed?.[2] ?? "";
	const explicit = explicitReference(value, lineStart, safeCursor, context);
	if (explicit) return explicit;
	if (!command || !line.endsWith(" ")) return null;

	const hint = command.params[Math.min(argumentCount(args), Math.max(0, command.params.length - 1))] ?? command.params[0] ?? "";
	const inferred = inferredProvider(hint, safeCursor, context);
	return inferred?.items.length ? inferred : null;
}

function commandScore(command: QuickActionCommand, query: string) {
	if (!query) return 0;
	if (command.name.toLowerCase().startsWith(query)) return 3;
	if (command.aliases.some((alias) => alias.toLowerCase().startsWith(query))) return 2;
	return 1;
}

function commandItem(command: QuickActionCommand): AutocompleteItem {
	const prefix = parameterPrefix(command.params[0] ?? "");
	const usage = [`/${command.name}`, ...command.params].join(" ");
	return {
		key: `command:${command.name}`,
		kind: "command",
		label: usage,
		detail:
			command.warning ||
			command.description ||
			(command.aliases.length ? `Aliases: ${command.aliases.map((alias) => `/${alias}`).join(", ")}` : "GitLab Quick Action"),
		insertText: `/${command.name}${command.params.length ? ` ${prefix}` : ""}`
	};
}

function explicitReference(value: string, lineStart: number, cursor: number, context: Context): AutocompleteResult | null {
	const line = value.slice(lineStart, cursor);
	const patterns: Array<[RegExp, (query: string) => AutocompleteItem[]]> = [
		[/@([A-Za-z0-9_.-]*)$/, (query) => mergeItems(memberItems(context.bootstrap, query), remoteItems(context, "member"))],
		[/~(?:"([^"]*)|([^\s~]*))$/, (query) => mergeItems(labelItems(context.labels, query), remoteItems(context, "label"))],
		[/#([^\s#]*)$/, (query) => mergeItems(workItemItems(context.bootstrap, query), remoteItems(context, "work_item"))],
		[/%(?:"([^"]*)|([^\s%]*))$/, () => remoteItems(context, "milestone")],
		[/!([^\s!]*)$/, () => remoteItems(context, "merge_request")],
		[/&([^\s&]*)$/, () => remoteItems(context, "epic")],
		[/\$([^\s$]*)$/, () => remoteItems(context, "snippet")],
		[/\*\s*(?:iteration:)?(?:"([^"]*)|([^\s*]*))$/, () => remoteItems(context, "iteration")]
	];
	for (const [pattern, provider] of patterns) {
		const match = pattern.exec(line);
		if (!match) continue;
		const query = match[1] ?? match[2] ?? "";
		return { start: lineStart + match.index, end: cursor, items: provider(query) };
	}
	return null;
}

function inferredProvider(hint: string, cursor: number, context: Context): AutocompleteResult | null {
	const normalized = hint.toLowerCase();
	let items: AutocompleteItem[] = [];
	if (normalized.includes("@")) items = mergeItems(memberItems(context.bootstrap, ""), remoteItems(context, "member"));
	else if (normalized.includes("~")) items = mergeItems(labelItems(context.labels, ""), remoteItems(context, "label"));
	else if (normalized.includes("#")) items = mergeItems(workItemItems(context.bootstrap, ""), remoteItems(context, "work_item"));
	else if (normalized.includes("%")) items = remoteItems(context, "milestone");
	else if (normalized.includes("!")) items = remoteItems(context, "merge_request");
	else if (normalized.includes("&")) items = remoteItems(context, "epic");
	else if (normalized.includes("$")) items = remoteItems(context, "snippet");
	else if (normalized.includes("iteration") || normalized.includes("*")) items = remoteItems(context, "iteration");
	else if (normalized.includes("branch")) items = remoteItems(context, "branch");
	else if (normalized.includes("project") || normalized.includes("namespace")) items = remoteItems(context, "project");
	else if (normalized.includes("status")) items = statusItems(context.bootstrap, "");
	else if (normalized.includes("type")) items = typeItems("");
	return items.length ? { start: cursor, end: cursor, items } : null;
}

function memberItems(bootstrap: Bootstrap, query: string): AutocompleteItem[] {
	const needle = query.toLowerCase();
	return bootstrap.members
		.filter((member) => member.state === "active" && (!needle || `${member.username} ${member.displayName}`.toLowerCase().includes(needle)))
		.slice(0, 20)
		.map((member) => ({
			key: `member:${member.gitLabUserId}`,
			kind: "member",
			label: `@${member.username}`,
			detail: member.displayName,
			insertText: `@${member.username}`
		}));
}

function labelItems(labels: ProjectLabel[], query: string): AutocompleteItem[] {
	const needle = query.toLowerCase();
	return labels
		.filter((label) => !needle || `${label.name} ${label.description ?? ""}`.toLowerCase().includes(needle))
		.slice(0, 20)
		.map((label) => ({
			key: `label:${label.id}`,
			kind: "label",
			label: label.name,
			detail: label.description ?? "GitLab label",
			insertText: `~${quoteReference(label.name)}`
		}));
}

function workItemItems(bootstrap: Bootstrap, query: string): AutocompleteItem[] {
	return bootstrap.board.cards
		.filter((card) => card.issueIid > 0 && (!query || String(card.issueIid).startsWith(query)))
		.slice(0, 20)
		.map((card) => ({
			key: `work-item:${card.issueIid}`,
			kind: "work_item",
			label: `#${card.issueIid}`,
			detail: card.title,
			insertText: `#${card.issueIid}`
		}));
}

function remoteItems(context: Context, kind: QuickActionSuggestionKind): AutocompleteItem[] {
	return (context.suggestions ?? [])
		.filter((item) => item.kind === kind)
		.map((item) => ({
			key: `${item.kind}:${item.id}`,
			kind: item.kind,
			label: item.label,
			detail: item.detail ?? "GitLab suggestion",
			insertText: item.value
		}));
}

function mergeItems(primary: AutocompleteItem[], secondary: AutocompleteItem[]) {
	const seen = new Set(primary.map((item) => item.insertText));
	return [...primary, ...secondary.filter((item) => !seen.has(item.insertText))].slice(0, 20);
}

export function suggestionRequestAt(value: string, cursor: number, commands: QuickActionCommand[]): SuggestionRequest | null {
	const safeCursor = Math.max(0, Math.min(cursor, value.length));
	const lineStart = value.lastIndexOf("\n", safeCursor - 1) + 1;
	const line = value.slice(lineStart, safeCursor);
	const explicit: Array<[RegExp, QuickActionSuggestionKind]> = [
		[/@([A-Za-z0-9_.-]*)$/, "member"],
		[/~(?:"([^"]*)|([^\s~]*))$/, "label"],
		[/#([^\s#]*)$/, "work_item"],
		[/%(?:"([^"]*)|([^\s%]*))$/, "milestone"],
		[/!([^\s!]*)$/, "merge_request"],
		[/&([^\s&]*)$/, "epic"],
		[/\$([^\s$]*)$/, "snippet"],
		[/\*\s*(?:iteration:)?(?:"([^"]*)|([^\s*]*))$/, "iteration"]
	];
	for (const [pattern, kind] of explicit) {
		const match = pattern.exec(line);
		if (match) return { kind, query: match[1] ?? match[2] ?? "" };
	}
	const parsed = commandLinePattern.exec(line);
	const command = parsed ? findCommand(commands, parsed[1]!) : undefined;
	if (!command || !line.endsWith(" ")) return null;
	const args = parsed?.[2] ?? "";
	return providerForHint(command.params[Math.min(argumentCount(args), Math.max(0, command.params.length - 1))] ?? command.params[0] ?? "");
}

function providerForHint(hint: string): SuggestionRequest | null {
	const normalized = hint.toLowerCase();
	if (normalized.includes("@")) return { kind: "member", query: "" };
	if (normalized.includes("~")) return { kind: "label", query: "" };
	if (normalized.includes("#")) return { kind: "work_item", query: "" };
	if (normalized.includes("%")) return { kind: "milestone", query: "" };
	if (normalized.includes("!")) return { kind: "merge_request", query: "" };
	if (normalized.includes("&")) return { kind: "epic", query: "" };
	if (normalized.includes("$")) return { kind: "snippet", query: "" };
	if (normalized.includes("iteration") || normalized.includes("*")) return { kind: "iteration", query: "" };
	if (normalized.includes("branch")) return { kind: "branch", query: "" };
	if (normalized.includes("project") || normalized.includes("namespace")) return { kind: "project", query: "" };
	return null;
}

function statusItems(bootstrap: Bootstrap, query: string): AutocompleteItem[] {
	const needle = query.toLowerCase();
	return bootstrap.board.lists
		.filter((list) => !needle || list.name.toLowerCase().includes(needle))
		.map((list) => ({ key: `status:${list.key}`, kind: "status", label: list.name, detail: "Lifecycle status", insertText: `"${list.name}"` }));
}

function typeItems(query: string): AutocompleteItem[] {
	const needle = query.toLowerCase();
	return ["Issue", "Task", "Incident"]
		.filter((type) => type.toLowerCase().includes(needle))
		.map((type) => ({
			key: `type:${type}`,
			kind: "type",
			label: type,
			detail: "Work item type",
			insertText: `"${type}"`
		}));
}

function findCommand(commands: QuickActionCommand[], token: string) {
	const normalized = token.toLowerCase();
	return commands.find((command) => command.name.toLowerCase() === normalized || command.aliases.some((alias) => alias.toLowerCase() === normalized));
}

function parameterPrefix(param: string) {
	const match = /^([@~%#!&$*]|\[contact:|")/.exec(param);
	return match?.[1] ?? "";
}

function argumentCount(args: string) {
	return args.trim() ? args.trim().split(/\s+/).length : 0;
}

function quoteReference(value: string) {
	return /\s/.test(value) ? `"${value.replaceAll('"', '\\"')}"` : value;
}
