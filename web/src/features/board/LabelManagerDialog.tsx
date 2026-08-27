import { Button, ConfirmDialog, Dialog, IconButton } from "@project-template/ui";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Pencil, Trash2 } from "lucide-react";
import { useState } from "react";
import { bootstrapQueryKey } from "./bootstrap";
import { LabelColorPicker } from "./LabelColorPicker";
import styles from "./LabelManagerDialog.module.css";
import { defaultLabelColor, normalizeLabelColor } from "./labelPalette";
import { isReservedLabel } from "./labels";
import { createProjectLabel, deleteProjectLabel, updateProjectLabel, type ProjectLabelWrite } from "./labelsApi";
import type { Bootstrap, ProjectLabel } from "./model";
import { TagSwatch } from "./TagSwatch";
import { projectLabelsQueryKey, useProjectLabels } from "./useProjectLabels";

export interface LabelManagerDialogProps {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	bootstrap: Bootstrap;
	/** Pre-seeds create mode, for the "create <query>" row in the pickers. */
	initialName?: string;
	/** Called after a successful create, so the caller can apply it to a card. */
	onCreated?: (label: ProjectLabel) => void;
}

/**
 * Project-wide label administration.
 *
 * Deliberately its own dialog rather than affordances inside the card's tag
 * picker: that picker is a <details> popover inside a form inside a drawer, and
 * nesting a color picker plus an AlertDialog there is a focus-management
 * minefield. It also keeps an irreversible delete away from routine tagging.
 */
export function LabelManagerDialog({ open, onOpenChange, bootstrap, initialName, onCreated }: LabelManagerDialogProps) {
	const client = useQueryClient();
	const labelsQuery = useProjectLabels();
	const [query, setQuery] = useState("");
	const [editing, setEditing] = useState<ProjectLabel | null>(null);
	const [pendingDelete, setPendingDelete] = useState<ProjectLabel | null>(null);
	const [error, setError] = useState<string | null>(null);

	// A renamed or deleted label leaves every card's label array stale and can
	// strand a URL filter that now matches nothing, so both are rewritten here.
	// The server reconciles within one board poll; this only avoids the gap.
	const rewriteCards = (map: (labels: string[]) => string[]) => {
		client.setQueryData<Bootstrap>(bootstrapQueryKey, (current) =>
			current ? { ...current, board: { ...current.board, cards: current.board.cards.map((card) => ({ ...card, labels: map(card.labels) })) } } : current
		);
	};
	const settle = () => {
		void client.invalidateQueries({ queryKey: projectLabelsQueryKey });
		void client.invalidateQueries({ queryKey: bootstrapQueryKey });
	};

	const create = useMutation({
		mutationFn: (input: ProjectLabelWrite) => createProjectLabel(input),
		onSuccess: (label) => {
			setError(null);
			settle();
			onCreated?.(label);
		},
		onError: (cause: Error) => setError(cause.message)
	});

	const update = useMutation({
		mutationFn: ({ label, next }: { label: ProjectLabel; next: { name: string; color: string; description: string | null } }) =>
			updateProjectLabel(label.id, next).then((result) => ({ previous: label.name, result })),
		onSuccess: ({ previous, result }) => {
			rewriteCards((labels) => labels.map((entry) => (entry === previous ? result.name : entry)));
			setEditing(null);
			setError(null);
			settle();
		},
		onError: (cause: Error) => setError(cause.message)
	});

	const remove = useMutation({
		mutationFn: (label: ProjectLabel) => deleteProjectLabel(label.id).then(() => label.name),
		onSuccess: (removed) => {
			rewriteCards((labels) => labels.filter((entry) => entry !== removed));
			setPendingDelete(null);
			setError(null);
			settle();
		},
		onError: (cause: Error) => setError(cause.message)
	});

	const all = labelsQuery.data ?? [];
	const normalizedQuery = query.trim().toLocaleLowerCase("zh-Hant");
	const matches = (label: ProjectLabel) =>
		!normalizedQuery || `${label.name} ${label.description ?? ""}`.toLocaleLowerCase("zh-Hant").includes(normalizedQuery);
	const editable = all.filter((label) => !isReservedLabel(bootstrap, label.name)).filter(matches);
	// Shown rather than hidden: otherwise people hunt for Team::開發組 and try to
	// create it themselves.
	const reserved = all.filter((label) => isReservedLabel(bootstrap, label.name)).filter(matches);

	const affectedCards = (label: ProjectLabel) => bootstrap.board.cards.filter((card) => card.labels.includes(label.name)).length;

	return (
		<>
			<Dialog open={open} onOpenChange={onOpenChange} title="管理 Labels" description="建立、更名或刪除這個 GitLab 專案的 Labels">
				<div className={styles.manager}>
					<LabelCreateForm
						key={`${String(open)}:${initialName ?? ""}:${String(create.isSuccess)}`}
						initialName={initialName ?? ""}
						busy={create.isPending}
						onCreate={(next) => create.mutate(next)}
					/>

					<label className={styles.search}>
						<span className={styles.srOnly}>搜尋 Label</span>
						<input type="search" value={query} placeholder="搜尋 Label 名稱或說明" onChange={(event) => setQuery(event.target.value)} />
					</label>

					{error ? (
						<p className={styles.error} role="alert">
							{error}
						</p>
					) : null}

					<ul className={styles.list} aria-label="可管理的 Labels">
						{editable.map((label) =>
							editing?.id === label.id ? (
								<li key={label.id}>
									<LabelEditRow label={label} busy={update.isPending} onCancel={() => setEditing(null)} onSave={(next) => update.mutate({ label, next })} />
								</li>
							) : (
								<li key={label.id} className={styles.row}>
									<TagSwatch label={label} />
									<span className={styles.name}>{label.name}</span>
									<span className={styles.description}>{label.description}</span>
									<IconButton label={`編輯 ${label.name}`} size="sm" icon={<Pencil size="0.875rem" aria-hidden="true" />} onClick={() => setEditing(label)} />
									<IconButton
										label={`刪除 ${label.name}`}
										size="sm"
										tone="error"
										icon={<Trash2 size="0.875rem" aria-hidden="true" />}
										onClick={() => setPendingDelete(label)}
									/>
								</li>
							)
						)}
						{labelsQuery.isSuccess && editable.length === 0 ? <li className={styles.empty}>沒有可管理的 Label</li> : null}
						{labelsQuery.isLoading ? <li role="status">載入中...</li> : null}
					</ul>

					{reserved.length ? (
						<section className={styles.reserved} aria-labelledby="reserved-labels-heading">
							<h3 id="reserved-labels-heading">由 board-directory.yml 管理</h3>
							<ul aria-label="保留的 Labels">
								{reserved.map((label) => (
									<li key={label.id} className={styles.row}>
										<TagSwatch label={label} />
										<span className={styles.name}>{label.name}</span>
										<span className={styles.description}>{label.description}</span>
									</li>
								))}
							</ul>
						</section>
					) : null}
				</div>
			</Dialog>

			<ConfirmDialog
				open={pendingDelete !== null}
				onOpenChange={(next) => !next && setPendingDelete(null)}
				title={pendingDelete ? `刪除 ${pendingDelete.name}？` : "刪除 Label？"}
				description={pendingDelete ? `刪除後，${affectedCards(pendingDelete)} 張卡片會失去這個 Label，且無法復原。` : ""}
				confirmLabel="刪除"
				cancelLabel="取消"
				destructive
				busy={remove.isPending}
				onConfirm={() => pendingDelete && remove.mutate(pendingDelete)}
			/>
		</>
	);
}

function LabelCreateForm({
	initialName,
	busy,
	onCreate
}: {
	initialName: string;
	busy: boolean;
	onCreate: (next: { name: string; color: string; description: string | null }) => void;
}) {
	const [name, setName] = useState(initialName);
	const [color, setColor] = useState(defaultLabelColor);
	const [description, setDescription] = useState("");
	const normalizedColor = normalizeLabelColor(color);
	const canCreate = name.trim().length > 0 && normalizedColor !== null && !busy;
	return (
		<form
			className={styles.create}
			onSubmit={(event) => {
				event.preventDefault();
				if (!canCreate || !normalizedColor) return;
				onCreate({ name: name.trim(), color: normalizedColor, description: description.trim() || null });
			}}
		>
			<label className={styles.field}>
				<span>新 Label 名稱</span>
				<input value={name} maxLength={255} placeholder="例如 Priority::High" onChange={(event) => setName(event.target.value)} />
			</label>
			<label className={styles.field}>
				<span>新 Label 描述</span>
				<input value={description} placeholder="選填" onChange={(event) => setDescription(event.target.value)} />
			</label>
			<LabelColorPicker value={color} onChange={setColor} label="新 Label 顏色" />
			<Button type="submit" variant="filled" disabled={!canCreate} loading={busy} loadingLabel="建立中">
				建立 Label
			</Button>
		</form>
	);
}

function LabelEditRow({
	label,
	busy,
	onCancel,
	onSave
}: {
	label: ProjectLabel;
	busy: boolean;
	onCancel: () => void;
	onSave: (next: { name: string; color: string; description: string | null }) => void;
}) {
	const [name, setName] = useState(label.name);
	const [color, setColor] = useState(label.color);
	const [description, setDescription] = useState(label.description ?? "");
	const normalizedColor = normalizeLabelColor(color);
	const canSave = name.trim().length > 0 && normalizedColor !== null && !busy;
	return (
		<form
			className={styles.edit}
			onSubmit={(event) => {
				event.preventDefault();
				if (!canSave || !normalizedColor) return;
				onSave({ name: name.trim(), color: normalizedColor, description: description.trim() || null });
			}}
		>
			<label className={styles.field}>
				<span>編輯名稱</span>
				<input autoFocus value={name} maxLength={255} onChange={(event) => setName(event.target.value)} />
			</label>
			<label className={styles.field}>
				<span>編輯描述</span>
				<input value={description} placeholder="選填" onChange={(event) => setDescription(event.target.value)} />
			</label>
			<LabelColorPicker value={color} onChange={setColor} label={`編輯 ${label.name} 顏色`} />
			<div className={styles.editActions}>
				<Button type="button" variant="text" onClick={onCancel}>
					取消
				</Button>
				<Button type="submit" variant="filled" disabled={!canSave} loading={busy} loadingLabel="儲存中">
					儲存
				</Button>
			</div>
		</form>
	);
}
