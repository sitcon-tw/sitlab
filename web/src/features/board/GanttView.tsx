import { Badge, Button, EmptyState, SegmentedButton, type SegmentedOption } from "@project-template/ui";
import { CalendarClock, CalendarDays, CalendarRange, CircleAlert } from "lucide-react";
import { useEffect, useRef, type CSSProperties, type RefObject } from "react";
import { type GanttScheduledItem, type GanttTimeline, type GanttViewModel } from "./ganttModel";
import styles from "./GanttView.module.css";
import type { BoardCard, Bootstrap, GanttScale } from "./model";

export interface GanttViewProps {
	bootstrap: Bootstrap;
	viewModel: GanttViewModel;
	filtersActive: boolean;
	scale: GanttScale;
	onScaleChange: (scale: GanttScale) => void;
	onOpen: (issueIid: number) => void;
}

type GanttStyle = CSSProperties & Record<`--gantt-${string}`, string | number>;

const shortDateFormatter = new Intl.DateTimeFormat("zh-TW", { month: "numeric", day: "numeric", timeZone: "UTC" });
const longDateFormatter = new Intl.DateTimeFormat("zh-TW", { year: "numeric", month: "long", day: "numeric", timeZone: "UTC" });
const weekdayFormatter = new Intl.DateTimeFormat("zh-TW", { weekday: "short", timeZone: "UTC" });
const GANTT_SCALE_OPTIONS: Array<SegmentedOption<GanttScale>> = [
	{ value: "day", label: "日", icon: <CalendarDays size="1rem" aria-hidden="true" /> },
	{ value: "week", label: "週", icon: <CalendarRange size="1rem" aria-hidden="true" /> }
];

export default function GanttView({ bootstrap, viewModel, filtersActive, scale, onScaleChange, onOpen }: GanttViewProps) {
	const viewportRef = useRef<HTMLDivElement>(null);
	const cornerRef = useRef<HTMLDivElement>(null);
	const scrolledScale = useRef<GanttScale | null>(null);

	useEffect(() => {
		if (scrolledScale.current === scale || !viewModel.timeline) return;
		const viewport = viewportRef.current;
		const corner = cornerRef.current;
		if (!viewport || !corner) return;
		scrolledScale.current = scale;
		const scheduledDays = viewModel.groups.flatMap((group) =>
			group.scheduled.flatMap((item) => [item.startDay, item.dueDay].filter((day): day is number => day !== null))
		);
		const scheduledMinimum = Math.min(...scheduledDays);
		const scheduledMaximum = Math.max(...scheduledDays);
		const focusDay = Math.min(scheduledMaximum, Math.max(scheduledMinimum, viewModel.timeline.todayDay));
		const timelineWidth = viewport.scrollWidth - corner.offsetWidth;
		const focusPosition = ((focusDay - viewModel.timeline.startDay + 0.5) / viewModel.timeline.totalDays) * timelineWidth;
		const visibleTimelineWidth = Math.max(0, viewport.clientWidth - corner.offsetWidth);
		viewport.scrollLeft = Math.max(0, focusPosition - visibleTimelineWidth / 2);
	}, [scale, viewModel.groups, viewModel.timeline]);

	if (viewModel.orderedCards.length === 0) {
		return (
			<section className={styles.gantt} aria-label="SITCON 2027 甘特圖">
				<EmptyState
					icon={<CalendarClock aria-hidden="true" />}
					title={filtersActive ? "沒有符合篩選的開啟 Issue" : "目前沒有開啟 Issue"}
					description={filtersActive ? "調整搜尋或篩選條件後再試一次。" : "新的開啟 Issue 會顯示在這裡。"}
				/>
			</section>
		);
	}

	const scheduledGroups = viewModel.groups.filter((group) => group.scheduled.length > 0);
	const unscheduledGroups = viewModel.groups.filter((group) => group.unscheduled.length > 0);

	return (
		<section className={styles.gantt} aria-label="SITCON 2027 甘特圖">
			<header className={styles.intro}>
				<div>
					<h2>開啟 Issue 排程</h2>
					<p>{scale === "day" ? "日尺度" : "週尺度"} · 依組別分組 · 點選列即可開啟明細</p>
				</div>
				<div className={styles.introActions}>
					<SegmentedButton label="時間尺度" options={GANTT_SCALE_OPTIONS} value={scale} onChange={onScaleChange} />
					<Badge tone="neutral">{viewModel.orderedCards.length} 個開啟 Issue</Badge>
				</div>
			</header>

			{viewModel.timeline ? (
				<Timeline
					bootstrap={bootstrap}
					groups={scheduledGroups}
					timeline={viewModel.timeline}
					scale={scale}
					onOpen={onOpen}
					viewportRef={viewportRef}
					cornerRef={cornerRef}
				/>
			) : (
				<div className={styles.scheduledEmpty}>
					<CalendarClock aria-hidden="true" />
					<div>
						<h3>還沒有已排程的 Issue</h3>
						<p>在明細中加入 Start date 或 Due date 後，就會出現在時間軸。</p>
					</div>
				</div>
			)}

			{unscheduledGroups.length > 0 ? (
				<section className={styles.unscheduled} aria-labelledby="gantt-unscheduled-title">
					<header>
						<div>
							<h2 id="gantt-unscheduled-title">未排程</h2>
							<p>尚未設定 Start date 與 Due date</p>
						</div>
						<Badge tone="neutral">{unscheduledGroups.reduce((total, group) => total + group.unscheduled.length, 0)}</Badge>
					</header>
					{unscheduledGroups.map((group) => (
						<section className={styles.unscheduledGroup} aria-labelledby={`unscheduled-${group.teamKey}`} key={group.teamKey}>
							<h3 id={`unscheduled-${group.teamKey}`}>{group.teamName}</h3>
							<div className={styles.unscheduledGrid}>
								{group.unscheduled.map((card) => (
									<Button key={card.issueIid} variant="elevated" className={styles.unscheduledCard} onClick={() => onOpen(card.issueIid)}>
										<span>#{card.issueIid}</span>
										<strong>{card.title}</strong>
										<small>{statusName(bootstrap, card)}</small>
									</Button>
								))}
							</div>
						</section>
					))}
				</section>
			) : null}
		</section>
	);
}

interface TimelineProps {
	bootstrap: Bootstrap;
	groups: GanttViewModel["groups"];
	timeline: GanttTimeline;
	scale: GanttScale;
	onOpen: (issueIid: number) => void;
	viewportRef: RefObject<HTMLDivElement | null>;
	cornerRef: RefObject<HTMLDivElement | null>;
}

function Timeline({ bootstrap, groups, timeline, scale, onOpen, viewportRef, cornerRef }: TimelineProps) {
	const units = timelineUnits(timeline, scale);
	const canvasStyle: GanttStyle = { "--gantt-unit-count": units.length };
	const todayStyle = offsetStyle(timeline.todayDay, timeline);
	return (
		<div className={styles.viewport} ref={viewportRef} tabIndex={0} aria-label="甘特圖時間軸，可水平與垂直捲動" data-scale={scale}>
			<div className={styles.canvas} style={canvasStyle}>
				<div className={styles.calendarGrid} aria-hidden="true">
					{units.map((unit) => (
						<span key={unit.startDate} />
					))}
				</div>
				<span className={styles.todayLine} style={todayStyle} aria-hidden="true" />
				<header className={styles.timelineHeader}>
					<div className={styles.corner} ref={cornerRef}>
						<span>Issue</span>
						<small>狀態與日期</small>
					</div>
					<div className={styles.weekHeaders}>
						{units.map((unit) => (
							<div key={unit.startDate} aria-label={unitLabel(unit, scale)}>
								<strong>{shortDate(unit.startDate)}</strong>
								<span>{scale === "day" ? weekday(unit.startDate) : `– ${shortDate(unit.endDate)}`}</span>
							</div>
						))}
						<span className={styles.todayLabel} style={todayStyle} aria-hidden="true">
							今天
						</span>
					</div>
				</header>

				{groups.map((group) => (
					<section className={styles.group} aria-labelledby={`gantt-${group.teamKey}`} key={group.teamKey}>
						<header className={styles.groupHeader}>
							<div className={styles.groupSticky}>
								<h3 id={`gantt-${group.teamKey}`}>{group.teamName}</h3>
								<Badge tone="neutral">{group.scheduled.length}</Badge>
							</div>
							<div aria-hidden="true" />
						</header>
						{group.scheduled.map((item) => (
							<ScheduleRow key={item.card.issueIid} bootstrap={bootstrap} item={item} timeline={timeline} onOpen={onOpen} />
						))}
					</section>
				))}
			</div>
		</div>
	);
}

function ScheduleRow({
	bootstrap,
	item,
	timeline,
	onOpen
}: {
	bootstrap: Bootstrap;
	item: GanttScheduledItem;
	timeline: GanttTimeline;
	onOpen: (issueIid: number) => void;
}) {
	const card = item.card;
	const summary = dateSummary(item);
	return (
		<Button
			variant="text"
			className={styles.issueRow}
			data-list={card.listKey}
			onClick={() => onOpen(card.issueIid)}
			aria-label={`開啟 Issue #${card.issueIid} ${card.title}，${statusName(bootstrap, card)}，${summary}`}
		>
			<span className={styles.issueCell}>
				<span className={styles.issueTitle}>
					<small>#{card.issueIid}</small>
					<strong>{card.title}</strong>
				</span>
				<span className={styles.issueMeta}>
					<span>{statusName(bootstrap, card)}</span>
					<span aria-hidden="true">·</span>
					<span data-error={item.kind === "invalid" || undefined}>{summary}</span>
				</span>
			</span>
			<span className={styles.track} aria-hidden="true">
				<ScheduleMark item={item} timeline={timeline} />
			</span>
		</Button>
	);
}

function ScheduleMark({ item, timeline }: { item: GanttScheduledItem; timeline: GanttTimeline }) {
	if (item.kind === "range") {
		const style: GanttStyle = {
			"--gantt-offset-days": item.startDay! - timeline.startDay,
			"--gantt-span-days": item.dueDay! - item.startDay! + 1
		};
		return (
			<span className={styles.range} style={style}>
				<span>
					{shortDate(item.card.startDate!)} – {shortDate(item.card.dueDate!)}
				</span>
			</span>
		);
	}
	if (item.kind === "invalid") {
		return (
			<>
				<span className={`${styles.marker} ${styles.invalidMarker}`} style={offsetStyle(item.startDay!, timeline)} />
				<span className={`${styles.marker} ${styles.invalidMarker}`} style={offsetStyle(item.dueDay!, timeline)} />
				<span className={styles.invalidLabel} style={offsetStyle(Math.min(item.startDay!, item.dueDay!), timeline)}>
					<CircleAlert size="1rem" /> 日期錯誤
				</span>
			</>
		);
	}
	const day = item.dueDay ?? item.startDay!;
	return (
		<>
			<span className={`${styles.marker} ${item.kind === "due-milestone" ? styles.milestone : styles.startMarker}`} style={offsetStyle(day, timeline)} />
			<span className={styles.markerLabel} style={offsetStyle(day, timeline)}>
				{item.kind === "due-milestone" ? "Due" : "Start"} · {shortDate(item.card.dueDate ?? item.card.startDate!)}
			</span>
		</>
	);
}

function offsetStyle(day: number, timeline: GanttTimeline): GanttStyle {
	return { "--gantt-offset-days": day - timeline.startDay };
}

function dateSummary(item: GanttScheduledItem) {
	switch (item.kind) {
		case "range":
			return `${shortDate(item.card.startDate!)} → ${shortDate(item.card.dueDate!)}`;
		case "due-milestone":
			return `Due ${shortDate(item.card.dueDate!)}`;
		case "start-marker":
			return `Start ${shortDate(item.card.startDate!)}`;
		case "invalid":
			return `日期錯誤：Start ${shortDate(item.card.startDate!)}、Due ${shortDate(item.card.dueDate!)}`;
	}
}

function statusName(bootstrap: Bootstrap, card: BoardCard) {
	return bootstrap.board.lists.find((list) => list.key === card.listKey)?.name ?? card.listKey;
}

function shortDate(value: string) {
	return shortDateFormatter.format(new Date(`${value}T00:00:00Z`));
}

function longDate(value: string) {
	return longDateFormatter.format(new Date(`${value}T00:00:00Z`));
}

function weekday(value: string) {
	return weekdayFormatter.format(new Date(`${value}T00:00:00Z`));
}

function timelineUnits(timeline: GanttTimeline, scale: GanttScale) {
	if (scale === "week") return timeline.weeks;
	return Array.from({ length: timeline.totalDays }, (_, index) => {
		const date = new Date((timeline.startDay + index) * 24 * 60 * 60 * 1000).toISOString().slice(0, 10);
		return { startDate: date, endDate: date };
	});
}

function unitLabel(unit: { startDate: string; endDate: string }, scale: GanttScale) {
	return scale === "day" ? `${longDate(unit.startDate)}，${weekday(unit.startDate)}` : `${longDate(unit.startDate)}至${longDate(unit.endDate)}`;
}
