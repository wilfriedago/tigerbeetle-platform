import { type ColumnDef, createColumnHelper, metaHelper, type RowData, tableFeatures, useTable } from '@tanstack/react-table'

import { Skeleton } from '~/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '~/components/ui/table'

import { cn } from '~/lib/utils'

// 15 Data display, Data Table: the table parts on TanStack Table. Columns declare their alignment
// in `meta` (numbers right, text left). Rows can open a record: the whole row is the click target
// for the pointer, and the first column carries a real link for the keyboard.

interface ColumnMeta {
  align?: 'left' | 'right' | 'center'
  /** Applied to the header and the cells of the column. */
  className?: string
}

/** Core features only: sorting and filtering happen in the API, never on a page of rows. */
export const dataTableFeatures = tableFeatures({ columnMeta: metaHelper<ColumnMeta>() })

export type DataTableFeatures = typeof dataTableFeatures

// Cell values differ per column, so the column list cannot name one value type.
export type DataTableColumn<TData extends RowData> = ColumnDef<DataTableFeatures, TData, any>

export function columnHelper<TData extends RowData>() {
  return createColumnHelper<DataTableFeatures, TData>()
}

/** Interactive elements inside a row keep their own click. */
const INTERACTIVE = 'a, button, input, select, textarea, [role="menuitem"], [role="checkbox"]'

interface DataTableProps<TData extends RowData> {
  columns: readonly DataTableColumn<TData>[]
  data: readonly TData[] | undefined
  getRowId: (row: TData) => string
  /** The accessible name of the table. */
  label: string
  onRowClick?: (row: TData) => void
  /** Marks rows, such as a stuck request, with `data-state`. */
  rowState?: (row: TData) => 'selected' | 'warning' | undefined
  loading?: boolean
  empty?: React.ReactNode
  className?: string
}

export function DataTable<TData extends RowData>({
  columns,
  data,
  getRowId,
  label,
  onRowClick,
  rowState,
  loading = false,
  empty,
  className
}: DataTableProps<TData>) {
  const table = useTable({
    columns: columns as DataTableColumn<TData>[],
    data: data ?? [],
    features: dataTableFeatures,
    getRowId: (row) => getRowId(row)
  })
  const { rows } = table.getRowModel()
  const columnCount = table.getAllLeafColumns().length
  return (
    <Table aria-label={label} aria-busy={loading || undefined} className={className}>
      <TableHeader>
        {table.getHeaderGroups().map((group) => (
          <TableRow key={group.id}>
            {group.headers.map((header) => (
              <TableHead key={header.id} align={header.column.columnDef.meta?.align} className={header.column.columnDef.meta?.className}>
                {header.isPlaceholder ? null : <table.FlexRender header={header} />}
              </TableHead>
            ))}
          </TableRow>
        ))}
      </TableHeader>
      <TableBody>
        {rows.map((row) => (
          <TableRow
            key={row.id}
            data-state={rowState?.(row.original)}
            className={cn(onRowClick && 'cursor-pointer', 'data-[state=warning]:bg-warning-bg/60')}
            onClick={
              onRowClick
                ? (event) => {
                    if (!(event.target as HTMLElement).closest(INTERACTIVE)) {
                      onRowClick(row.original)
                    }
                  }
                : undefined
            }
          >
            {row.getAllCells().map((cell) => (
              <TableCell key={cell.id} align={cell.column.columnDef.meta?.align} className={cell.column.columnDef.meta?.className}>
                <table.FlexRender cell={cell} />
              </TableCell>
            ))}
          </TableRow>
        ))}
        {loading && rows.length === 0
          ? Array.from({ length: 5 }, (_, index) => (
              <TableRow key={index}>
                <TableCell colSpan={columnCount}>
                  <Skeleton className="w-full" />
                </TableCell>
              </TableRow>
            ))
          : null}
        {!loading && rows.length === 0 && empty ? (
          <TableRow className="hover:bg-transparent">
            <TableCell colSpan={columnCount} className="h-auto py-10 text-center whitespace-normal text-muted-foreground">
              {empty}
            </TableCell>
          </TableRow>
        ) : null}
      </TableBody>
    </Table>
  )
}
