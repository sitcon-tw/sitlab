import type { BoardCard, BoardList, DirectoryTeam } from "./model";

const DAY_MS = 24 * 60 * 60 * 1000;

export type GanttScheduleKind = "range" | "due-milestone" | "start-marker" | "invalid";

export interface GanttScheduledItem {
	card: BoardCard;
	kind: GanttScheduleKind;
	startDay: number | null;
	dueDay: number | null;
	anchorDay: number;
}

export interface GanttGroup {
	teamKey: string;
	teamName: string;
	sortOrder: number;
	scheduled: GanttScheduledItem[];
	unscheduled: BoardCard[];
}

export interface GanttWeek {
	startDate: string;
	endDate: string;
}

export interface GanttTimeline {
	startDay: number;
	endDay: number;
	totalDays: number;
	weeks: GanttWeek[];
	todayDay: number;
}

export interface GanttViewModel {
	groups: GanttGroup[];
	timeline: GanttTimeline | null;
	orderedCards: BoardCard[];
}

export function openBoardCards(cards: BoardCard[], lists: BoardList[]) {
	const openListKeys = new Set(lists.filter((list) => !list.closed).map((list) => list.key));
	return cards.filter((card) => openListKeys.has(card.listKey));
}

export function createGanttViewModel(cards: BoardCard[], teams: DirectoryTeam[], today: string): GanttViewModel {
	const groupsByKey = new Map<string, GanttGroup>();
	const teamsByKey = new Map(teams.map((team) => [team.key, team]));

	for (const card of cards) {
		const team = teamsByKey.get(card.teamKey);
		let group = groupsByKey.get(card.teamKey);
		if (!group) {
			group = {
				teamKey: card.teamKey,
				teamName: team?.name ?? "未分類",
				sortOrder: team?.sortOrder ?? Number.MAX_SAFE_INTEGER,
				scheduled: [],
				unscheduled: []
			};
			groupsByKey.set(card.teamKey, group);
		}

		const item = scheduleItem(card);
		if (item) group.scheduled.push(item);
		else group.unscheduled.push(card);
	}

	const groups = [...groupsByKey.values()].sort(
		(a, b) => a.sortOrder - b.sortOrder || a.teamName.localeCompare(b.teamName, "zh-Hant") || a.teamKey.localeCompare(b.teamKey)
	);
	for (const group of groups) {
		group.scheduled.sort(compareScheduledItems);
		group.unscheduled.sort((a, b) => a.position - b.position || a.issueIid - b.issueIid);
	}

	const scheduledItems = groups.flatMap((group) => group.scheduled);
	return {
		groups,
		timeline: createTimeline(scheduledItems, today),
		orderedCards: [...groups.flatMap((group) => group.scheduled.map((item) => item.card)), ...groups.flatMap((group) => group.unscheduled)]
	};
}

export function plainDateToDay(value: string) {
	const [year, month, day] = value.split("-").map(Number);
	return Math.floor(Date.UTC(year!, month! - 1, day!) / DAY_MS);
}

export function dayToPlainDate(value: number) {
	return new Date(value * DAY_MS).toISOString().slice(0, 10);
}

function scheduleItem(card: BoardCard): GanttScheduledItem | null {
	const startDay = card.startDate ? plainDateToDay(card.startDate) : null;
	const dueDay = card.dueDate ? plainDateToDay(card.dueDate) : null;
	if (startDay === null && dueDay === null) return null;
	if (startDay !== null && dueDay !== null) {
		return {
			card,
			kind: startDay <= dueDay ? "range" : "invalid",
			startDay,
			dueDay,
			anchorDay: Math.min(startDay, dueDay)
		};
	}
	if (dueDay !== null) return { card, kind: "due-milestone", startDay, dueDay, anchorDay: dueDay };
	return { card, kind: "start-marker", startDay, dueDay, anchorDay: startDay! };
}

function compareScheduledItems(a: GanttScheduledItem, b: GanttScheduledItem) {
	return a.anchorDay - b.anchorDay || (a.dueDay ?? a.startDay ?? 0) - (b.dueDay ?? b.startDay ?? 0) || a.card.issueIid - b.card.issueIid;
}

function createTimeline(items: GanttScheduledItem[], today: string): GanttTimeline | null {
	if (items.length === 0) return null;
	const todayDay = plainDateToDay(today);
	let minimum = todayDay;
	let maximum = todayDay;
	for (const item of items) {
		if (item.startDay !== null) {
			minimum = Math.min(minimum, item.startDay);
			maximum = Math.max(maximum, item.startDay);
		}
		if (item.dueDay !== null) {
			minimum = Math.min(minimum, item.dueDay);
			maximum = Math.max(maximum, item.dueDay);
		}
	}

	const startDay = startOfWeek(minimum) - 7;
	const endDay = startOfWeek(maximum) + 14;
	const weeks: GanttWeek[] = [];
	for (let weekStart = startDay; weekStart < endDay; weekStart += 7) {
		weeks.push({ startDate: dayToPlainDate(weekStart), endDate: dayToPlainDate(weekStart + 6) });
	}
	return { startDay, endDay, totalDays: endDay - startDay, weeks, todayDay };
}

function startOfWeek(day: number) {
	const weekday = new Date(day * DAY_MS).getUTCDay();
	return day - ((weekday + 6) % 7);
}
