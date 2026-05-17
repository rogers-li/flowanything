import type { ReactNode } from "react";

type Column<T> = {
  key: string;
  header: string;
  render: (item: T) => ReactNode;
};

type DataTableProps<T> = {
  columns: Column<T>[];
  rows: T[];
  getRowKey: (item: T) => string;
  emptyMessage?: string;
  onRowClick?: (item: T) => void;
};

export function DataTable<T>({ columns, rows, getRowKey, emptyMessage = "No data.", onRowClick }: DataTableProps<T>) {
  return (
    <div className="table-shell" role="region" aria-label="Data table" tabIndex={0}>
      <table>
        <thead>
          <tr>
            {columns.map((column) => (
              <th key={column.key} scope="col">
                {column.header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.length === 0 ? (
            <tr>
              <td colSpan={columns.length}>{emptyMessage}</td>
            </tr>
          ) : null}
          {rows.map((row) => (
            <tr
              key={getRowKey(row)}
              className={onRowClick ? "clickable-table-row" : undefined}
              onClick={() => onRowClick?.(row)}
            >
              {columns.map((column) => (
                <td key={column.key}>{column.render(row)}</td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
