import {useEffect,useRef,useState,useCallback,type ReactNode} from 'react';import {createPortal} from 'react-dom';
import {createApp,h,reactive} from 'vue';import {createI18n} from 'vue-i18n';
import './monaco-workers';import NativeWorkbench from './NativeWorkbench.vue';import {registerNativePanels} from './native-registration';import './native-theme.scss';
type Props={scopeKey?:string;source:string;readOnly:boolean;busy?:boolean;selected:string;panels:Record<string,ReactNode>;onSelect:(id:string)=>void;onFiles:(step:string)=>void;onChange:(source:string)=>void;onSave:()=>void;onPublish:()=>void;onValidate:()=>void};
export function NativeCodeEditorHost(props:Props){
 const element=useRef<HTMLDivElement>(null),[slots,setSlots]=useState<Record<string,HTMLElement>>({});
 const attach=useCallback((name:string,el:HTMLElement|null)=>setSlots(previous=>{if(previous[name]===el||!el&&!previous[name])return previous;const next={...previous};if(el)next[name]=el;else delete next[name];return next}),[]);
 const state=useRef(reactive({...props,panels:undefined,attach,path:'flow.yaml',theme:document.documentElement.classList.contains('dark')?'dark':'light'}));
 useEffect(()=>{const app=createApp({setup:()=>()=>h(NativeWorkbench,state.current)});registerNativePanels(app);app.use(createI18n({legacy:false,locale:'zh',missingWarn:false,fallbackWarn:false,messages:{zh:{close:'关闭','select view':'选择视图','panel actions':'面板操作',editor_shortcuts:{label:'快捷键'},empty:{panels:{title:'选择编辑视图',content:'从顶部打开 Visual、YAML 或其他面板'}},multi_panel_editor:{move_left:'移至左侧面板',move_right:'移至右侧面板',close_all_tabs:'关闭此面板标签',close_all_panels:'关闭全部面板',fullscreen:'最大化面板',exit_fullscreen:'恢复面板布局'}}}}));app.mount(element.current!);const observer=new MutationObserver(()=>{state.current.theme=document.documentElement.classList.contains('dark')?'dark':'light'});observer.observe(document.documentElement,{attributes:true,attributeFilter:['class']});return()=>{observer.disconnect();app.unmount()}},[]);
 useEffect(()=>{const {panels,...rest}=props;Object.assign(state.current,rest)},[props]);
 return <><div className="keel-kestra" ref={element}/>{Object.entries(slots).map(([name,target])=>createPortal(props.panels[name],target,name))}</>;
}
