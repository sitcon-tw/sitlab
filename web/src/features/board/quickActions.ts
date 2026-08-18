import type { BoardCard, Bootstrap } from "./model";

export const quickActionCommands = [
	{ command: "/assign", usage: "/assign @username", label: "指派成員" },
	{ command: "/unassign", usage: "/unassign [@username]", label: "取消指派" },
	{ command: "/due", usage: "/due YYYY-MM-DD", label: "設定期限" },
	{ command: "/remove_due_date", usage: "/remove_due_date", label: "清除期限" },
	{ command: "/start_date", usage: "/start_date YYYY-MM-DD", label: "設定開始日" },
	{ command: "/remove_start_date", usage: "/remove_start_date", label: "清除開始日" },
	{ command: "/close", usage: "/close", label: "關閉卡片" },
	{ command: "/reopen", usage: "/reopen", label: "重新開啟" }
] as const;

export type QuickAction =
	{ kind: "assign"; memberIds: number[] } | { kind: "due"; value: string | null } | { kind: "start"; value: string | null } | { kind: "move"; listKey: string };

export type QuickActionResult = { action: QuickAction; error?: never } | { action?: never; error: string };

export function parseQuickAction(raw: string, bootstrap: Bootstrap, card: BoardCard): QuickActionResult {
	const [command = "", ...args] = raw.trim().split(/\s+/);
	switch (command.toLowerCase()) {
		case "/assign": {
			const resolved = resolveMemberIds(args, bootstrap);
			if (!resolved.ok) return { error: resolved.error };
			return { action: { kind: "assign", memberIds: [...new Set([...card.assigneeGitLabUserIds, ...resolved.memberIds])] } };
		}
		case "/unassign": {
			if (!args.length) return { action: { kind: "assign", memberIds: [] } };
			const resolved = resolveMemberIds(args, bootstrap);
			if (!resolved.ok) return { error: resolved.error };
			return { action: { kind: "assign", memberIds: card.assigneeGitLabUserIds.filter((id) => !resolved.memberIds.includes(id)) } };
		}
		case "/due":
			return plainDateAction("due", args[0]);
		case "/remove_due_date":
			return { action: { kind: "due", value: null } };
		case "/start":
		case "/start_date":
			return plainDateAction("start", args[0]);
		case "/remove_start_date":
			return { action: { kind: "start", value: null } };
		case "/close": {
			const closed = bootstrap.board.lists.find((list) => list.closed);
			return closed ? { action: { kind: "move", listKey: closed.key } } : { error: "看板沒有 Done 欄位" };
		}
		case "/reopen": {
			const fallback = bootstrap.board.lists.find((list) => !list.closed);
			const current = bootstrap.board.lists.find((list) => list.key === card.listKey && !list.closed);
			const target = current ?? fallback;
			return target ? { action: { kind: "move", listKey: target.key } } : { error: "看板沒有可重新開啟的欄位" };
		}
		default:
			return { error: "不支援這個 Quick Action" };
	}
}

function resolveMemberIds(args: string[], bootstrap: Bootstrap): { ok: true; memberIds: number[] } | { ok: false; error: string } {
	if (!args.length) return { ok: false, error: "請輸入至少一個 @username" };
	const usernames = args.map((value) => value.replace(/^@/, "").toLowerCase());
	const members = usernames.map((username) => bootstrap.members.find((member) => member.state === "active" && member.username.toLowerCase() === username));
	const missingIndex = members.findIndex((member) => !member);
	if (missingIndex >= 0) return { ok: false, error: `找不到 @${usernames[missingIndex]}` };
	return { ok: true, memberIds: members.map((member) => member!.gitLabUserId) };
}

function plainDateAction(kind: "due" | "start", value: string | undefined): QuickActionResult {
	if (!value || !/^\d{4}-\d{2}-\d{2}$/.test(value)) return { error: "日期格式必須是 YYYY-MM-DD" };
	const parsed = new Date(`${value}T00:00:00Z`);
	if (Number.isNaN(parsed.getTime()) || parsed.toISOString().slice(0, 10) !== value) return { error: "日期不存在" };
	return { action: { kind, value } };
}
