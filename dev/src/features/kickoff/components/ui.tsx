import { useState, type ReactNode } from 'react';
import * as Dialog from '@radix-ui/react-dialog';
import { ArrowDown, ArrowUp, ChevronLeft, ChevronRight, Inbox, Search, X } from 'lucide-react';
import { Button as SharedButton } from '../../../components/shadcn/button';
import { useReactTable, getCoreRowModel, getFilteredRowModel, getSortedRowModel, getPaginationRowModel, flexRender, type ColumnDef, type SortingState } from '@tanstack/react-table';

export function Button({children,kind='secondary',...props}:React.ButtonHTMLAttributes<HTMLButtonElement>&{kind?:'primary'|'secondary'|'ghost'|'danger'}) {
  return <SharedButton type="button" {...props} className={`button ${kind} ${props.className??''}`}>{children}</SharedButton>;
}
export function Badge({state,children}:{state:string;children?:ReactNode}) {
  const labels:Record<string,string>={ready:'可用',partial:'部分就绪',needs_input:'待补充',blocked:'待确认',published:'已入库',recognized:'已识别',awaiting_analysis:'待分析',extracted:'已提取',unmapped:'待映射',source:'原始资料',mapping:'关联提取',manual:'人工确认',calculation:'计算结果',stale:'待重新计算'};
  return <span className={`badge ${state}`}><i/>{children??labels[state]??state}</span>;
}
export function Heading({eyebrow,title,description,action}:{eyebrow:string;title:string;description:string;action?:ReactNode}) {
  return <div className="heading"><div><div className="eyebrow">{eyebrow}</div><h1>{title}</h1><p>{description}</p></div><div className="actions">{action}</div></div>;
}
export function Section({title,caption,action,children,className=''}:{title:string;caption?:string;action?:ReactNode;children:ReactNode;className?:string}) {
  return <section className={`section ${className}`}><div className="section-heading"><div><h2>{title}</h2>{caption&&<p>{caption}</p>}</div>{action&&<div className="actions">{action}</div>}</div>{children}</section>;
}
export function Modal({title,description,children,close,wide=false,drawer=false}:{title:string;description:string;children:ReactNode;close:()=>void;wide?:boolean;drawer?:boolean}) {
  return <Dialog.Root open onOpenChange={open=>!open&&close()}><Dialog.Portal container={document.getElementById("kickoff-portals")}><Dialog.Overlay className="modal-overlay"/><Dialog.Content className={`${drawer?'drawer':'modal'} ${wide?'wide':''}`}><div className="modal-heading"><div><Dialog.Title>{title}</Dialog.Title><Dialog.Description>{description}</Dialog.Description></div><Dialog.Close className="icon-button" aria-label="关闭"><X size={18}/></Dialog.Close></div><div className="modal-body">{children}</div></Dialog.Content></Dialog.Portal></Dialog.Root>;
}
export function Empty({title,description,action}:{title:string;description:string;action?:ReactNode}) {return <div className="empty"><Inbox size={27}/><h3>{title}</h3><p>{description}</p>{action}</div>;}
export function Field({label,hint,children}:{label:string;hint?:string;children:ReactNode}) {return <label className="field"><span>{label}</span>{children}{hint&&<small>{hint}</small>}</label>;}
export function Notice({children,tone='info'}:{children:ReactNode;tone?:'info'|'warning'|'error'|'success'}){return <div className={`notice ${tone}`} role={tone==='error'?'alert':undefined}>{children}</div>;}
export function Stat({label,value,detail}:{label:string;value:ReactNode;detail:string}){return <div className="stat"><span>{label}</span><strong>{value}</strong><small>{detail}</small></div>;}
export function Tabs({items,value,onChange}:{items:{id:string;label:string;count?:number}[];value:string;onChange:(value:string)=>void}) {return <div className="tabs" role="tablist">{items.map(t=><button role="tab" aria-selected={t.id===value} className={t.id===value?'active':''} key={t.id} onClick={()=>onChange(t.id)}>{t.label}{t.count!==undefined&&<span>{t.count}</span>}</button>)}</div>;}
export function KV({values}:{values:[string,ReactNode][]}){return <dl className="kv">{values.map(([k,v])=><div key={k}><dt>{k}</dt><dd>{v||<span className="muted">待补充</span>}</dd></div>)}</dl>;}
export function DataTable<T>({data,columns,onRow,searchPlaceholder='搜索名称、编号或属性…',pageSize=10,toolbar}:{data:T[];columns:ColumnDef<T,any>[];onRow?:(row:T)=>void;searchPlaceholder?:string;pageSize?:number;toolbar?:ReactNode}) {
  const [search,setSearch]=useState(''),[sorting,setSorting]=useState<SortingState>([]);
  const table=useReactTable({data,columns,state:{globalFilter:search,sorting},onSortingChange:setSorting,onGlobalFilterChange:setSearch,
    getCoreRowModel:getCoreRowModel(),getFilteredRowModel:getFilteredRowModel(),getSortedRowModel:getSortedRowModel(),getPaginationRowModel:getPaginationRowModel(),initialState:{pagination:{pageSize}}});
  return <div className="data-table"><div className="table-toolbar"><label className="search-input"><Search size={15}/><input aria-label="搜索表格" placeholder={searchPlaceholder} value={search} onChange={e=>setSearch(e.target.value)}/>{search&&<button aria-label="清除搜索" onClick={()=>setSearch('')}><X size={13}/></button>}</label>{toolbar}</div><div className="table-scroll"><table><thead>{table.getHeaderGroups().map(g=><tr key={g.id}>{g.headers.map(h=><th key={h.id} style={{width:h.column.columnDef.size}}>{h.column.getCanSort()?<button onClick={h.column.getToggleSortingHandler()}>{flexRender(h.column.columnDef.header,h.getContext())}{h.column.getIsSorted()==='asc'?<ArrowUp size={12}/>:h.column.getIsSorted()==='desc'?<ArrowDown size={12}/>:null}</button>:flexRender(h.column.columnDef.header,h.getContext())}</th>)}</tr>)}</thead><tbody>{table.getRowModel().rows.map(row=><tr key={row.id} className={onRow?'clickable':''} onClick={()=>onRow?.(row.original)}>{row.getVisibleCells().map(c=><td key={c.id}>{flexRender(c.column.columnDef.cell,c.getContext())}</td>)}</tr>)}</tbody></table>{!table.getFilteredRowModel().rows.length&&<Empty title="没有匹配的记录" description={search?'调整搜索条件后重试。':'可以导入资料，或在工程建模中补充。'}/>}</div><div className="pagination"><span>共 {table.getFilteredRowModel().rows.length.toLocaleString()} 条记录</span><div><button aria-label="上一页" disabled={!table.getCanPreviousPage()} onClick={()=>table.previousPage()}><ChevronLeft size={15}/></button><span>{table.getState().pagination.pageIndex+1} / {Math.max(1,table.getPageCount())}</span><button aria-label="下一页" disabled={!table.getCanNextPage()} onClick={()=>table.nextPage()}><ChevronRight size={15}/></button></div></div></div>;
}
export function formatDate(value:string){return new Date(value).toLocaleString('zh-CN',{month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit',hour12:false});}
