import { useState, type ReactNode } from 'react';
import { useReactTable, getCoreRowModel, getSortedRowModel, getPaginationRowModel, flexRender, type ColumnDef } from '@tanstack/react-table';
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '@/components/shadcn/dialog';
import { Button } from '@/components/shadcn/button';
export { Button };
export function State({ value }: {
    value: unknown;
}) { const text = String(value ?? 'unknown'); return <span className={'fl-state fl-state-' + text.toLowerCase()}>{text}</span>; }
export function Notice({ error }: {
    error: unknown;
}) { return error ? <div role="alert" className="fl-error">{error instanceof Error ? error.message : String(error)}</div> : null; }
export function Empty({ loading, error, children }: {
    loading?: boolean;
    error?: unknown;
    children?: ReactNode;
}) { return <div className="fl-empty">{loading ? '正在加载真实数据…' : error ? <Notice error={error}/> : children || '暂无记录'}</div>; }
export function Panel({ title, description, open, onClose, children }: {
    title: string;
    description: string;
    open: boolean;
    onClose: () => void;
    children: ReactNode;
}) {
    return <Dialog open={open} onOpenChange={v => {
            if (!v)
                onClose();
        }}><DialogContent className="fl-dialog"><DialogTitle>{title}</DialogTitle><DialogDescription>{description}</DialogDescription>{children}</DialogContent></Dialog>;
}
export function Table<T extends {
    id: string;
}>({ data, columns, onSelect }: {
    data: T[];
    columns: ColumnDef<T, any>[];
    onSelect?: (row: T) => void;
}) {
    const table = useReactTable({ data, columns, getCoreRowModel: getCoreRowModel(), getSortedRowModel: getSortedRowModel(), getPaginationRowModel: getPaginationRowModel(), initialState: { pagination: { pageSize: 15 } } });
    return <div className="fl-table"><div className="fl-table-scroll"><table><thead>{table.getHeaderGroups().map(g => <tr key={g.id}>{g.headers.map(h => <th key={h.id}>{h.column.getCanSort()?<button onClick={h.column.getToggleSortingHandler()}>{flexRender(h.column.columnDef.header, h.getContext())}{h.column.getIsSorted() === 'asc' ? ' ↑' : h.column.getIsSorted() === 'desc' ? ' ↓' : ''}</button>:flexRender(h.column.columnDef.header,h.getContext())}</th>)}</tr>)}</thead><tbody>{table.getRowModel().rows.map(r => <tr key={r.id} tabIndex={onSelect ? 0 : undefined} onKeyDown={e => {
                if (e.key === 'Enter' && e.target === e.currentTarget)
                    onSelect?.(r.original);
            }} onClick={e => {
                if (!(e.target as HTMLElement).closest('button,input,a,select,[role=menuitem]'))
                    onSelect?.(r.original);
            }}>{r.getVisibleCells().map(c => <td key={c.id}>{flexRender(c.column.columnDef.cell, c.getContext())}</td>)}</tr>)}</tbody></table>{!data.length && <Empty />}</div><div className="fl-pagination"><span>{data.length} 条记录</span><Button variant="outline" size="sm" disabled={!table.getCanPreviousPage()} onClick={() => table.previousPage()}>上一页</Button><span>{table.getState().pagination.pageIndex + 1} / {Math.max(1, table.getPageCount())}</span><Button variant="outline" size="sm" disabled={!table.getCanNextPage()} onClick={() => table.nextPage()}>下一页</Button></div></div>;
}
export function Json({ value }: {
    value: unknown;
}) { return <pre className="fl-code">{JSON.stringify(value, null, 2)}</pre>; }
export function Tabs({ items, current, onChange }: {
    items: string[];
    current: string;
    onChange: (v: string) => void;
}) { return <div className="fl-tabs" role="tablist">{items.map(v => <button key={v} role="tab" aria-selected={current === v} onClick={() => onChange(v)}>{v}</button>)}</div>; }
export function Search({ value, onChange, placeholder }: {
    value: string;
    onChange: (v: string) => void;
    placeholder: string;
}) { return <input className="fl-search" type="search" placeholder={placeholder} aria-label={placeholder} value={value} onChange={e => onChange(e.target.value)}/>; }
