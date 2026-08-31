import { describe, expect, it } from "vitest";
import { addDays, addMonths, calendarGrid, calendarRange, formatDateInput, parseDateInput, parseISODate, toISODate } from "./calendarDate";

describe("calendar date helpers", () => {
	it("parses supported input formats and rejects impossible dates", () => {
		expect(parseDateInput("20260829")).toEqual({ year: 2026, month: 8, day: 29 });
		expect(parseDateInput("2026/08/29")).toEqual({ year: 2026, month: 8, day: 29 });
		expect(parseDateInput("2026-08-29")).toEqual({ year: 2026, month: 8, day: 29 });
		expect(parseDateInput("2026/02/29")).toBeNull();
		expect(parseISODate("2026/08/29")).toBeNull();
	});

	it("formats canonical values without changing their date", () => {
		expect(formatDateInput("2026-08-29")).toBe("2026/08/29");
		expect(formatDateInput(null)).toBe("");
		expect(toISODate({ year: 2027, month: 4, day: 3 })).toBe("2027-04-03");
	});

	it("moves across month and leap-year boundaries in UTC", () => {
		expect(addDays("2027-01-01", -1)).toBe("2026-12-31");
		expect(addMonths("2026-08-31", 1)).toBe("2026-09-30");
		expect(addDays("2028-02-28", 1)).toBe("2028-02-29");
	});

	it("builds a stable six-week Sunday-first grid", () => {
		const dates = calendarGrid({ year: 2026, month: 8 });
		expect(dates).toHaveLength(42);
		expect(dates[0]).toBe("2026-07-26");
		expect(dates[41]).toBe("2026-09-05");
	});

	it("builds one continuous week-aligned range", () => {
		const dates = calendarRange("2026-07-01", "2027-04-30");
		expect(dates[0]).toBe("2026-06-28");
		expect(dates.at(-1)).toBe("2027-05-01");
		expect(dates).toHaveLength(308);
	});
});
