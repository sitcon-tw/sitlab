import * as DialogPrimitive from "@radix-ui/react-dialog";
import * as PopoverPrimitive from "@radix-ui/react-popover";
import { CalendarDays, ChevronLeft, ChevronRight } from "lucide-react";
import { useId, useLayoutEffect, useRef, useState, type FocusEvent, type KeyboardEvent, type PointerEvent, type Ref, type UIEvent } from "react";
import {
	addDays,
	addMonths,
	calendarRange,
	clampDate,
	compareMonths,
	endOfWeek,
	formatDateInput,
	isCompleteDateInput,
	localToday,
	monthKey,
	monthOf,
	parseDateInput,
	parseISODate,
	startOfWeek,
	toISODate,
	type CalendarMonth
} from "../lib/calendarDate";
import { classNames } from "../lib/classNames";
import { useRipple } from "../lib/useRipple";
import { Button } from "./Button";
import { describedBy, FieldFrame } from "./FieldFrame";
import { IconButton } from "./IconButton";

const MOBILE_CALENDAR_QUERY = "(max-width: 38rem)";
const WEEKDAYS = [
	{ short: "日", long: "星期日" },
	{ short: "一", long: "星期一" },
	{ short: "二", long: "星期二" },
	{ short: "三", long: "星期三" },
	{ short: "四", long: "星期四" },
	{ short: "五", long: "星期五" },
	{ short: "六", long: "星期六" }
] as const;
const MONTH_FORMATTER = new Intl.DateTimeFormat("zh-TW", { timeZone: "UTC", year: "numeric", month: "long" });
const DATE_FORMATTER = new Intl.DateTimeFormat("zh-TW", { timeZone: "UTC", dateStyle: "full" });

interface EditingDate {
	draft: string;
	lastEmitted: string | null | undefined;
}

export interface DateFieldProps {
	label: string;
	value: string | null;
	onValueChange: (value: string | null) => void;
	/** First day shown by the calendar navigator. This is not a validation boundary. */
	calendarStart: string;
	/** Last day shown by the calendar navigator. This is not a validation boundary. */
	calendarEnd: string;
	/** Canonical YYYY-MM-DD date used by the Today action. */
	today?: string | undefined;
	id?: string | undefined;
	name?: string | undefined;
	className?: string | undefined;
	description?: string | undefined;
	optional?: boolean | undefined;
	dense?: boolean | undefined;
	variant?: "field" | "compact" | undefined;
	disabled?: boolean | undefined;
	required?: boolean | undefined;
}

function utcDate(value: string) {
	return new Date(`${value}T00:00:00Z`);
}

function monthLabel(month: CalendarMonth) {
	return MONTH_FORMATTER.format(new Date(Date.UTC(month.year, month.month - 1, 1)));
}

function initialCalendarDate(today: string, start: string, end: string) {
	if (parseISODate(today)) return clampDate(today, start, end);
	return start;
}

function CalendarDayButton({
	date,
	label,
	selected,
	today,
	outsideMonth,
	disabled,
	tabIndex,
	buttonRef,
	onSelect,
	onKeyDown
}: {
	date: string;
	label: string;
	selected: boolean;
	today: boolean;
	outsideMonth: boolean;
	disabled: boolean;
	tabIndex: number;
	buttonRef: Ref<HTMLButtonElement>;
	onSelect: () => void;
	onKeyDown: (event: KeyboardEvent<HTMLButtonElement>) => void;
}) {
	const ripple = useRipple();
	return (
		<button
			ref={buttonRef}
			type="button"
			className="md-date-picker__day md-state-layer"
			aria-label={label}
			aria-selected={selected}
			data-date={date}
			data-selected={selected || undefined}
			data-today={today || undefined}
			data-outside-month={outsideMonth || undefined}
			disabled={disabled}
			tabIndex={tabIndex}
			onClick={onSelect}
			onKeyDown={onKeyDown}
			onPointerDown={(event: PointerEvent<HTMLButtonElement>) => ripple.onPointerDown(event)}
		>
			{Number(date.slice(-2))}
			{ripple.rippleNodes}
		</button>
	);
}

interface CalendarPanelProps {
	label: string;
	monthLabelId: string;
	visibleMonth: CalendarMonth;
	focusedDate: string;
	selectedDate: string | null;
	today: string;
	calendarStart: string;
	calendarEnd: string;
	dayButtons: React.MutableRefObject<Map<string, HTMLButtonElement>>;
	onFocusedDateChange: (date: string, focus: boolean) => void;
	onMonthChange: (date: string) => void;
	onSelect: (date: string | null) => void;
}

function CalendarPanel({
	label,
	monthLabelId,
	visibleMonth,
	focusedDate,
	selectedDate,
	today,
	calendarStart,
	calendarEnd,
	dayButtons,
	onFocusedDateChange,
	onMonthChange,
	onSelect
}: CalendarPanelProps) {
	const startMonth = monthOf(calendarStart);
	const endMonth = monthOf(calendarEnd);
	const dates = calendarRange(calendarStart, calendarEnd);
	const rows = Array.from({ length: dates.length / 7 }, (_, index) => dates.slice(index * 7, index * 7 + 7));
	const previousDisabled = compareMonths(visibleMonth, startMonth) <= 0;
	const nextDisabled = compareMonths(visibleMonth, endMonth) >= 0;
	const scrollViewport = useRef<HTMLDivElement>(null);
	const initiallyPositioned = useRef(false);
	const rowMetrics = useRef({ height: 44, bodyOffset: 0 });

	const moveByMonths = (baseDate: string, months: number, focus: boolean) => {
		const next = clampDate(addMonths(baseDate, months), calendarStart, calendarEnd);
		onMonthChange(next);
		onFocusedDateChange(next, focus);
	};
	const moveVisibleMonth = (months: number) => moveByMonths(`${monthKey(visibleMonth)}-15`, months, true);

	const moveDayFocus = (date: string, event: KeyboardEvent<HTMLButtonElement>) => {
		let next: string;
		switch (event.key) {
			case "ArrowLeft":
				next = addDays(date, -1);
				break;
			case "ArrowRight":
				next = addDays(date, 1);
				break;
			case "ArrowUp":
				next = addDays(date, -7);
				break;
			case "ArrowDown":
				next = addDays(date, 7);
				break;
			case "Home":
				next = startOfWeek(date);
				break;
			case "End":
				next = endOfWeek(date);
				break;
			case "PageUp":
				event.preventDefault();
				moveByMonths(date, event.shiftKey ? -12 : -1, true);
				return;
			case "PageDown":
				event.preventDefault();
				moveByMonths(date, event.shiftKey ? 12 : 1, true);
				return;
			default:
				return;
		}
		event.preventDefault();
		onFocusedDateChange(clampDate(next, calendarStart, calendarEnd), true);
	};
	useLayoutEffect(() => {
		if (initiallyPositioned.current) return;
		const viewport = scrollViewport.current;
		const rowIndex = rows.findIndex((row) => row.includes(focusedDate));
		const body = viewport?.querySelector("tbody");
		const firstRow = body?.querySelector("tr");
		if (!viewport || !body || rowIndex < 0) return;

		const rowHeight = firstRow?.getBoundingClientRect().height || 44;
		rowMetrics.current = { height: rowHeight, bodyOffset: body.offsetTop };
		viewport.scrollTop = body.offsetTop + rowIndex * rowHeight - (viewport.clientHeight - rowHeight) / 2;
		initiallyPositioned.current = true;
	}, [focusedDate, rows]);
	const updateVisibleMonth = (event: UIEvent<HTMLDivElement>) => {
		const viewport = event.currentTarget;
		const { height, bodyOffset } = rowMetrics.current;
		const center = viewport.scrollTop + viewport.clientHeight / 2 - bodyOffset;
		const rowIndex = Math.max(0, Math.min(rows.length - 1, Math.floor(center / height)));
		const centerDate = rows[rowIndex]?.[0];
		if (centerDate) onMonthChange(centerDate);
	};

	return (
		<div className="md-date-picker__panel">
			<header className="md-date-picker__header">
				<h2 id={monthLabelId} aria-live="polite">
					{monthLabel(visibleMonth)}
				</h2>
				<div className="md-date-picker__navigation">
					<IconButton
						size="sm"
						label="上一個月"
						icon={<ChevronLeft size="1.25rem" aria-hidden="true" />}
						disabled={previousDisabled}
						onClick={() => moveVisibleMonth(-1)}
					/>
					<IconButton
						size="sm"
						label="下一個月"
						icon={<ChevronRight size="1.25rem" aria-hidden="true" />}
						disabled={nextDisabled}
						onClick={() => moveVisibleMonth(1)}
					/>
				</div>
			</header>
			<div ref={scrollViewport} className="md-date-picker__scroll" onScroll={updateVisibleMonth}>
				<table className="md-date-picker__grid" role="grid" aria-labelledby={monthLabelId}>
					<thead>
						<tr>
							{WEEKDAYS.map((weekday) => (
								<th key={weekday.long} scope="col" abbr={weekday.long}>
									{weekday.short}
								</th>
							))}
						</tr>
					</thead>
					<tbody>
						{rows.map((row, rowIndex) => (
							<tr key={row[0] ?? rowIndex}>
								{row.map((date) => {
									const dateMonth = monthOf(date);
									const disabled = date < calendarStart || date > calendarEnd;
									return (
										<td key={date}>
											<CalendarDayButton
												date={date}
												label={DATE_FORMATTER.format(utcDate(date))}
												selected={date === selectedDate}
												today={date === today}
												outsideMonth={compareMonths(dateMonth, visibleMonth) !== 0}
												disabled={disabled}
												tabIndex={!disabled && date === focusedDate ? 0 : -1}
												buttonRef={(node) => {
													if (node) dayButtons.current.set(date, node);
													else dayButtons.current.delete(date);
												}}
												onSelect={() => onSelect(date)}
												onKeyDown={(event) => moveDayFocus(date, event)}
											/>
										</td>
									);
								})}
							</tr>
						))}
					</tbody>
				</table>
			</div>
			<footer className="md-date-picker__footer">
				<Button variant="text" size="sm" onClick={() => onSelect(null)}>
					清除
				</Button>
				<Button variant="text" size="sm" onClick={() => onSelect(today)}>
					今天
				</Button>
			</footer>
			<p className="md-sr-only">{label}月曆可連續上下捲動，也可使用方向鍵逐日或逐週移動。</p>
		</div>
	);
}

export function DateField({
	label,
	value,
	onValueChange,
	calendarStart,
	calendarEnd,
	today: explicitToday,
	id: explicitId,
	name,
	className,
	description,
	optional,
	dense,
	variant = "field",
	disabled = false,
	required = false
}: DateFieldProps) {
	const generatedId = useId();
	const id = explicitId ?? generatedId;
	const monthLabelId = `${id}-calendar-month`;
	const formatId = `${id}-format`;
	const today = explicitToday && parseISODate(explicitToday) ? explicitToday : localToday();
	const [open, setOpen] = useState(false);
	const [mobile, setMobile] = useState(false);
	const [editing, setEditing] = useState<EditingDate | null>(null);
	const [inputError, setInputError] = useState<string>();
	const initialFocus = initialCalendarDate(today, calendarStart, calendarEnd);
	const [visibleMonth, setVisibleMonth] = useState(() => monthOf(initialFocus));
	const [focusedDate, setFocusedDate] = useState(initialFocus);
	const [focusRequest, setFocusRequest] = useState(0);
	const inputRef = useRef<HTMLInputElement>(null);
	const dayButtons = useRef(new Map<string, HTMLButtonElement>());

	useLayoutEffect(() => {
		if (!open || focusRequest === 0) return;
		dayButtons.current.get(focusedDate)?.focus({ preventScroll: true });
	}, [focusRequest, focusedDate, open]);

	const displayedValue = editing?.draft ?? formatDateInput(value);
	const emit = (next: string | null) => {
		if (next !== value && next !== editing?.lastEmitted) onValueChange(next);
	};
	const commitDraft = () => {
		if (!editing) return;
		const normalized = editing.draft.trim();
		if (!normalized) {
			emit(null);
			setEditing(null);
			setInputError(undefined);
			return;
		}
		const parsed = parseDateInput(normalized);
		if (parsed) {
			emit(toISODate(parsed));
			setEditing(null);
			setInputError(undefined);
			return;
		}
		setEditing(null);
		setInputError("請輸入有效日期（YYYY/MM/DD）；已還原原日期。");
	};
	const changeDraft = (draft: string) => {
		const parsed = parseDateInput(draft);
		if (parsed) {
			const next = toISODate(parsed);
			emit(next);
			setEditing({ draft: formatDateInput(next), lastEmitted: next });
			setInputError(undefined);
			return;
		}
		setEditing({ draft, lastEmitted: editing?.lastEmitted });
		setInputError(isCompleteDateInput(draft) ? "請輸入有效日期（YYYY/MM/DD）。" : undefined);
	};
	const changeOpen = (next: boolean) => {
		if (next) {
			setMobile(window.matchMedia(MOBILE_CALENDAR_QUERY).matches);
			const nextFocus = initialCalendarDate(today, calendarStart, calendarEnd);
			setFocusedDate(nextFocus);
			setVisibleMonth(monthOf(nextFocus));
			setFocusRequest((current) => current + 1);
		}
		setOpen(next);
	};
	const selectDate = (next: string | null) => {
		emit(next);
		setEditing(null);
		setInputError(undefined);
		setOpen(false);
	};
	const changeFocusedDate = (next: string, focus: boolean) => {
		setFocusedDate(next);
		setVisibleMonth(monthOf(next));
		if (focus) setFocusRequest((current) => current + 1);
	};
	const changeMonth = (date: string) => {
		const next = monthOf(date);
		setVisibleMonth((current) => (compareMonths(current, next) === 0 ? current : next));
	};
	const focusCalendarOnOpen = (event: Event) => {
		event.preventDefault();
		dayButtons.current.get(focusedDate)?.focus({ preventScroll: true });
	};
	const handleControlBlur = (event: FocusEvent<HTMLDivElement>) => {
		if (event.currentTarget.contains(event.relatedTarget as Node | null) || open) return;
		commitDraft();
	};
	const handleInputKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
		if (event.key === "Enter") {
			event.preventDefault();
			commitDraft();
		} else if (event.key === "Escape" && editing) {
			event.preventDefault();
			event.stopPropagation();
			setEditing(null);
			setInputError(undefined);
		} else if (event.key === "ArrowDown" && event.altKey) {
			event.preventDefault();
			changeOpen(true);
		}
	};

	const trigger = (
		<IconButton
			size="sm"
			className="md-date-field__trigger"
			label={`開啟${label}日曆`}
			icon={<CalendarDays size="1.125rem" aria-hidden="true" />}
			disabled={disabled}
			aria-expanded={open}
		/>
	);
	const calendar = (
		<CalendarPanel
			label={label}
			monthLabelId={monthLabelId}
			visibleMonth={visibleMonth}
			focusedDate={focusedDate}
			selectedDate={value}
			today={today}
			calendarStart={calendarStart}
			calendarEnd={calendarEnd}
			dayButtons={dayButtons}
			onFocusedDateChange={changeFocusedDate}
			onMonthChange={changeMonth}
			onSelect={selectDate}
		/>
	);
	const picker = mobile ? (
		<DialogPrimitive.Root open={open} onOpenChange={changeOpen}>
			<DialogPrimitive.Trigger asChild>{trigger}</DialogPrimitive.Trigger>
			<DialogPrimitive.Portal>
				<DialogPrimitive.Overlay className="md-overlay md-date-picker__overlay" />
				<DialogPrimitive.Content className="md-date-picker md-date-picker--dialog" onOpenAutoFocus={focusCalendarOnOpen}>
					<DialogPrimitive.Title className="md-sr-only">{label}日期選擇器</DialogPrimitive.Title>
					<DialogPrimitive.Description className="md-sr-only">可使用方向鍵選擇日期，或使用下方按鈕清除日期與選擇今天。</DialogPrimitive.Description>
					{calendar}
				</DialogPrimitive.Content>
			</DialogPrimitive.Portal>
		</DialogPrimitive.Root>
	) : (
		<PopoverPrimitive.Root open={open} onOpenChange={changeOpen}>
			<PopoverPrimitive.Trigger asChild>{trigger}</PopoverPrimitive.Trigger>
			<PopoverPrimitive.Portal>
				<PopoverPrimitive.Content
					className="md-date-picker md-date-picker--popover"
					align="end"
					sideOffset={4}
					collisionPadding={8}
					aria-label={`${label}日期選擇器`}
					onOpenAutoFocus={focusCalendarOnOpen}
				>
					{calendar}
				</PopoverPrimitive.Content>
			</PopoverPrimitive.Portal>
		</PopoverPrimitive.Root>
	);
	const input = (
		<>
			<input
				ref={inputRef}
				id={id}
				type="text"
				className={variant === "compact" ? "md-date-field__compact-input" : "md-field__input md-date-field__input"}
				value={displayedValue}
				placeholder="YYYY/MM/DD"
				inputMode="numeric"
				maxLength={10}
				autoComplete="off"
				disabled={disabled}
				required={required}
				aria-label={variant === "compact" ? label : undefined}
				aria-invalid={inputError ? true : undefined}
				aria-describedby={describedBy(id, description, inputError, formatId)}
				onFocus={() => {
					setEditing((current) => current ?? { draft: formatDateInput(value), lastEmitted: undefined });
					setInputError(undefined);
				}}
				onChange={(event) => changeDraft(event.target.value)}
				onKeyDown={handleInputKeyDown}
			/>
			{picker}
			{name ? <input type="hidden" name={name} value={value ?? ""} disabled={disabled} /> : null}
			<span id={formatId} className="md-sr-only">
				日期格式：YYYY/MM/DD
			</span>
		</>
	);

	if (variant === "compact") {
		return (
			<div className={classNames("md-date-field md-date-field--compact", className)} data-invalid={inputError ? true : undefined} onBlur={handleControlBlur}>
				{input}
				{inputError ? (
					<span id={`${id}-error`} className="md-sr-only" role="alert">
						{inputError}
					</span>
				) : null}
			</div>
		);
	}

	return (
		<div className="md-date-field" onBlur={handleControlBlur}>
			<FieldFrame
				id={id}
				label={label}
				description={description}
				error={inputError}
				optional={optional}
				dense={dense}
				floating
				className={classNames("md-field--date", className)}
			>
				{input}
			</FieldFrame>
		</div>
	);
}
