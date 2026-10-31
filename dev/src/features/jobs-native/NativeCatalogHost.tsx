import {useEffect,useRef} from 'react';
import {createApp,reactive,h,type App} from 'vue';
import {createI18n} from 'vue-i18n';
import NativeCatalog from './NativeCatalog.vue';
import './native-theme.scss';
export type NativeCatalogProps={rows:unknown[];executionMode:boolean;loading:boolean;error:string;canCreate:boolean;canExecute:boolean;hasMore:boolean;query:string;stateFilter:string;onFilter:(key:string,value:string)=>void;onRefresh:()=>void;onQuickRun:()=>void;onCreate:()=>void;onOpen:(row:any)=>void;onRun:(row:any)=>void;onEdit:(row:any)=>void;onLoadMore:()=>void};
export function NativeCatalogHost(props:NativeCatalogProps){
 const host=useRef<HTMLDivElement>(null),instance=useRef<App|null>(null),state=useRef(reactive({...props}));
 useEffect(()=>{const app=createApp({setup:()=>()=>h(NativeCatalog,state.current)});app.use(createI18n({legacy:false,locale:'en',fallbackLocale:'en',messages:{en:{ks_no_data:{no_results:'暂无记录',nothing_here:'当前没有匹配结果',will_appear:'调整筛选条件或创建作业后查看'},ks_bulk_select:{selected:'已选择 {count} 项',all:'全部 {count} 项'}}},missingWarn:false,fallbackWarn:false}));app.mount(host.current!);instance.current=app;return()=>{app.unmount();instance.current=null}},[]);
 useEffect(()=>{Object.assign(state.current,props)},[props]);
 return <div className="keel-kestra" ref={host}/>;
}
