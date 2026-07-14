import { Button, SelectField, TextField } from "@project-template/ui";
import { Search, X } from "lucide-react";
import { useState, type FormEvent } from "react";
import type { TaskFilters as TaskFilterValue, TaskStatus } from "./model";
import styles from "./TasksPage.module.css";

export interface TaskFiltersProps {
	filters: TaskFilterValue;
	onChange: (filters: TaskFilterValue) => void;
}

export function TaskFilters({ filters, onChange }: TaskFiltersProps) {
	const [query, setQuery] = useState(filters.q ?? "");

	function submit(event: FormEvent) {
		event.preventDefault();
		const trimmed = query.trim();
		const next = { ...filters };
		if (trimmed) next.q = trimmed;
		else delete next.q;
		onChange(next);
	}

	function updateStatus(value: string) {
		const status = value as TaskStatus | "all";
		const next = { ...filters };
		if (status === "all") delete next.status;
		else next.status = status;
		onChange(next);
	}

	function clear() {
		setQuery("");
		onChange({});
	}

	const active = Boolean(filters.q || filters.status);
	return (
		<form className={styles.filters} aria-label="Task filters" onSubmit={submit}>
			<TextField label="Search" value={query} placeholder="Search task titles" onChange={(event) => setQuery(event.target.value)} />
			<SelectField
				label="Status"
				value={filters.status ?? "all"}
				onChange={(event) => updateStatus(event.target.value)}
				options={[
					{ value: "all", label: "All statuses" },
					{ value: "todo", label: "To do" },
					{ value: "in_progress", label: "In progress" },
					{ value: "done", label: "Done" }
				]}
			/>
			<div className={styles.filterActions}>
				<Button type="submit" variant="secondary" leadingIcon={<Search size="1rem" aria-hidden="true" />}>
					Search
				</Button>
				{active ? (
					<Button variant="ghost" leadingIcon={<X size="1rem" aria-hidden="true" />} onClick={clear}>
						Clear
					</Button>
				) : null}
			</div>
		</form>
	);
}
