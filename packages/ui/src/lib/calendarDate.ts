export interface CalendarDate {
	year: number;
	month: number;
	day: number;
}

export interface CalendarMonth {
	year: number;
	month: number;
}

const ISO_DATE = /^(\d{4})-(\d{2})-(\d{2})$/;
const COMPACT_DATE = /^(\d{4})(\d{2})(\d{2})$/;
const SEPARATED_DATE = /^(\d{4})[/-](\d{2})[/-](\d{2})$/;

function validDate(year: number, month: number, day: number): CalendarDate | null {
	if (!Number.isInteger(year) || year < 1 || year > 9999 || month < 1 || month > 12 || day < 1 || day > 31) return null;
	const date = new Date(Date.UTC(year, month - 1, day));
	if (date.getUTCFullYear() !== year || date.getUTCMonth() !== month - 1 || date.getUTCDate() !== day) return null;
	return { year, month, day };
}

function matchDate(value: string, pattern: RegExp): CalendarDate | null {
	const match = pattern.exec(value);
	if (!match) return null;
	return validDate(Number(match[1]), Number(match[2]), Number(match[3]));
}

export function parseISODate(value: string): CalendarDate | null {
	return matchDate(value, ISO_DATE);
}

export function parseDateInput(value: string): CalendarDate | null {
	const normalized = value.trim();
	return matchDate(normalized, COMPACT_DATE) ?? matchDate(normalized, SEPARATED_DATE);
}

export function isCompleteDateInput(value: string) {
	const normalized = value.trim();
	return COMPACT_DATE.test(normalized) || SEPARATED_DATE.test(normalized);
}

export function toISODate(value: CalendarDate) {
	return `${String(value.year).padStart(4, "0")}-${String(value.month).padStart(2, "0")}-${String(value.day).padStart(2, "0")}`;
}

export function formatDateInput(value: string | null) {
	if (!value) return "";
	const parsed = parseISODate(value);
	return parsed ? `${String(parsed.year).padStart(4, "0")}/${String(parsed.month).padStart(2, "0")}/${String(parsed.day).padStart(2, "0")}` : value;
}

export function monthOf(value: string): CalendarMonth {
	const parsed = parseISODate(value);
	if (!parsed) throw new Error(`Invalid ISO date: ${value}`);
	return { year: parsed.year, month: parsed.month };
}

export function monthKey(value: CalendarMonth) {
	return `${String(value.year).padStart(4, "0")}-${String(value.month).padStart(2, "0")}`;
}

export function compareMonths(a: CalendarMonth, b: CalendarMonth) {
	return a.year === b.year ? a.month - b.month : a.year - b.year;
}

export function addDays(value: string, days: number) {
	const parsed = parseISODate(value);
	if (!parsed) throw new Error(`Invalid ISO date: ${value}`);
	const date = new Date(Date.UTC(parsed.year, parsed.month - 1, parsed.day + days));
	return toISODate({ year: date.getUTCFullYear(), month: date.getUTCMonth() + 1, day: date.getUTCDate() });
}

export function addMonths(value: string, months: number) {
	const parsed = parseISODate(value);
	if (!parsed) throw new Error(`Invalid ISO date: ${value}`);
	const first = new Date(Date.UTC(parsed.year, parsed.month - 1 + months, 1));
	const lastDay = new Date(Date.UTC(first.getUTCFullYear(), first.getUTCMonth() + 1, 0)).getUTCDate();
	return toISODate({ year: first.getUTCFullYear(), month: first.getUTCMonth() + 1, day: Math.min(parsed.day, lastDay) });
}

export function startOfWeek(value: string) {
	const parsed = parseISODate(value);
	if (!parsed) throw new Error(`Invalid ISO date: ${value}`);
	const weekday = new Date(Date.UTC(parsed.year, parsed.month - 1, parsed.day)).getUTCDay();
	return addDays(value, -weekday);
}

export function endOfWeek(value: string) {
	return addDays(startOfWeek(value), 6);
}

export function calendarGrid(month: CalendarMonth) {
	const first = `${monthKey(month)}-01`;
	const start = startOfWeek(first);
	return Array.from({ length: 42 }, (_, index) => addDays(start, index));
}

export function calendarRange(start: string, end: string) {
	const first = startOfWeek(start);
	const last = endOfWeek(end);
	const dates: string[] = [];
	for (let date = first; date <= last; date = addDays(date, 1)) dates.push(date);
	return dates;
}

export function clampDate(value: string, start: string, end: string) {
	if (value < start) return start;
	if (value > end) return end;
	return value;
}

export function localToday() {
	const date = new Date();
	return toISODate({ year: date.getFullYear(), month: date.getMonth() + 1, day: date.getDate() });
}
