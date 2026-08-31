import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { axe } from "vitest-axe";
import { DateField, type DateFieldProps } from "./DateField";
import { Drawer } from "./Dialog";
import { SelectField } from "./Field";

const range = { calendarStart: "2026-07-01", calendarEnd: "2027-04-30", today: "2026-08-31" } as const;

function ControlledDateField({ onChange = () => undefined, ...props }: Partial<DateFieldProps> & { onChange?: (value: string | null) => void }) {
	const [value, setValue] = useState<string | null>(props.value ?? "2026-08-29");
	return (
		<>
			<DateField
				label="期限"
				value={value}
				onValueChange={(next) => {
					setValue(next);
					onChange(next);
				}}
				{...range}
				{...props}
			/>
			<button type="button">下一個控制項</button>
		</>
	);
}

describe("DateField", () => {
	it("accepts compact, slash, and ISO typing while emitting canonical dates", async () => {
		const user = userEvent.setup();
		const onChange = vi.fn();
		render(<ControlledDateField onChange={onChange} />);
		const input = screen.getByRole("textbox", { name: "期限" });

		await user.clear(input);
		await user.type(input, "20270430");
		expect(input).toHaveValue("2027/04/30");
		expect(onChange).toHaveBeenLastCalledWith("2027-04-30");

		await user.clear(input);
		await user.type(input, "2028-05-01");
		expect(onChange).toHaveBeenLastCalledWith("2028-05-01");
	});

	it("rejects impossible typed dates and restores the committed value", async () => {
		const user = userEvent.setup();
		const onChange = vi.fn();
		render(<ControlledDateField onChange={onChange} />);
		const input = screen.getByRole("textbox", { name: "期限" });
		await user.clear(input);
		await user.type(input, "20260230");
		expect(input).toHaveAttribute("aria-invalid", "true");
		await user.keyboard("{Enter}");
		expect(input).toHaveValue("2026/08/29");
		expect(screen.getByRole("alert")).toHaveTextContent("已還原原日期");
		expect(onChange).not.toHaveBeenCalled();
	});

	it("opens on today while preserving the selected date, then navigates with the keyboard", async () => {
		const user = userEvent.setup();
		const onChange = vi.fn();
		render(<ControlledDateField onChange={onChange} />);
		await user.click(screen.getByRole("button", { name: "開啟期限日曆" }));
		const today = await screen.findByRole("button", { name: /2026年8月31日/ });
		expect(today).toHaveFocus();
		expect(screen.getByRole("button", { name: /2026年8月29日/ })).toHaveAttribute("aria-selected", "true");
		await user.keyboard("{ArrowRight}");
		const next = screen.getByRole("button", { name: /2026年9月1日/ });
		expect(next).toHaveFocus();
		await user.keyboard("{Enter}");
		expect(onChange).toHaveBeenLastCalledWith("2026-09-01");
		expect(screen.queryByRole("dialog", { name: "期限日期選擇器" })).not.toBeInTheDocument();
		expect(screen.getByRole("button", { name: "開啟期限日曆" })).toHaveFocus();
	});

	it("limits month navigation without treating the range as validation", async () => {
		const user = userEvent.setup();
		render(<ControlledDateField value="2026-07-01" today="2026-07-01" />);
		await user.click(screen.getByRole("button", { name: "開啟期限日曆" }));
		expect(screen.getByRole("button", { name: "上一個月" })).toBeDisabled();
		expect(screen.getByRole("heading", { name: "2026年7月" })).toBeVisible();
		await user.click(screen.getByRole("button", { name: "下一個月" }));
		expect(screen.getByRole("heading", { name: "2026年8月" })).toBeVisible();
	});

	it("renders the whole range as one continuous scroll and updates the centered month", async () => {
		const user = userEvent.setup();
		render(<ControlledDateField />);
		await user.click(screen.getByRole("button", { name: "開啟期限日曆" }));
		const scroller = document.querySelector<HTMLElement>(".md-date-picker__scroll");
		expect(scroller).not.toBeNull();
		expect(document.querySelectorAll(".md-date-picker__day")).toHaveLength(308);

		Object.defineProperty(scroller, "clientHeight", { configurable: true, value: 296 });
		(scroller as HTMLElement).scrollTop = 440;
		fireEvent.scroll(scroller as HTMLElement);
		expect(screen.getByRole("heading", { name: "2026年9月" })).toBeVisible();
	});

	it("clears and selects today from the footer actions", async () => {
		const user = userEvent.setup();
		const onChange = vi.fn();
		render(<ControlledDateField onChange={onChange} />);
		await user.click(screen.getByRole("button", { name: "開啟期限日曆" }));
		await user.click(screen.getByRole("button", { name: "清除" }));
		expect(onChange).toHaveBeenLastCalledWith(null);
		await user.click(screen.getByRole("button", { name: "開啟期限日曆" }));
		await user.click(screen.getByRole("button", { name: "今天" }));
		expect(onChange).toHaveBeenLastCalledWith("2026-08-31");
	});

	it("uses the centered dialog presentation on narrow screens", async () => {
		vi.mocked(window.matchMedia).mockImplementation(() => ({
			matches: true,
			media: "(max-width: 38rem)",
			onchange: null,
			addListener: vi.fn(),
			removeListener: vi.fn(),
			addEventListener: vi.fn(),
			removeEventListener: vi.fn(),
			dispatchEvent: vi.fn()
		}));
		const user = userEvent.setup();
		render(<ControlledDateField />);
		await user.click(screen.getByRole("button", { name: "開啟期限日曆" }));
		expect(await screen.findByRole("dialog", { name: "期限日期選擇器" })).toHaveClass("md-date-picker--dialog");
	});

	it("has no accessibility violations in field and compact variants", async () => {
		const { container } = render(
			<>
				<DateField label="Start" value="2026-08-29" onValueChange={() => undefined} {...range} />
				<DateField label="Due" value={null} onValueChange={() => undefined} variant="compact" {...range} />
			</>
		);
		expect((await axe(container)).violations).toEqual([]);
	});

	it("coexists with a portalled select inside a drawer", async () => {
		const user = userEvent.setup();
		render(
			<Drawer open onOpenChange={() => undefined} title="Details">
				<SelectField
					label="Status"
					value="todo"
					options={[
						{ value: "todo", label: "To do" },
						{ value: "doing", label: "Doing" }
					]}
				/>
				<DateField label="Due" value="2026-08-29" onValueChange={() => undefined} {...range} />
			</Drawer>
		);
		await user.click(screen.getByRole("button", { name: "Status" }));
		expect(await screen.findByRole("menu", { name: "Status選項" })).toBeVisible();
		await user.click(screen.getByRole("menuitemcheckbox", { name: "Doing" }));
		expect(screen.queryByRole("menu", { name: "Status選項" })).not.toBeInTheDocument();
		expect(screen.getByRole("dialog", { name: "Details" })).toBeVisible();
	});
});
