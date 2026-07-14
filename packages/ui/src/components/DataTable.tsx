import type { ReactNode } from "react";

export interface DataColumn<Row> {
	key: string;
	header: ReactNode;
	cell: (row: Row) => ReactNode;
	width?: string;
}

export interface DataTableProps<Row> {
	rows: Row[];
	columns: Array<DataColumn<Row>>;
	getRowKey: (row: Row) => string;
	label: string;
	empty?: ReactNode;
}

export function DataTable<Row>({ rows, columns, getRowKey, label, empty }: DataTableProps<Row>) {
	if (rows.length === 0 && empty) return <>{empty}</>;
	return (
		<div className="pt-data-table-wrap">
			<table className="pt-data-table">
				<caption className="pt-sr-only">{label}</caption>
				<thead>
					<tr>
						{columns.map((column) => (
							<th key={column.key} scope="col" style={column.width ? { width: column.width } : undefined}>
								{column.header}
							</th>
						))}
					</tr>
				</thead>
				<tbody>
					{rows.map((row) => (
						<tr key={getRowKey(row)}>
							{columns.map((column) => (
								<td key={column.key}>{column.cell(row)}</td>
							))}
						</tr>
					))}
				</tbody>
			</table>
		</div>
	);
}
