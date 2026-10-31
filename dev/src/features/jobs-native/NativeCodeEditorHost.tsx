import {useEffect,useRef} from 'react';
import {createApp,h,reactive} from 'vue';
import {createI18n} from 'vue-i18n';
import './monaco-workers';
import NativeCodeEditor from './NativeWorkbench.vue'
import {registerNativePanels} from './native-registration';
import './native-theme.scss';
type Props={scopeKey?:string;source:string;readOnly:boolean;onChange:(source:string)=>void;onSave:()=>void};
export function NativeCodeEditorHost(props:Props){
 const element=useRef<HTMLDivElement>(null),state=useRef(reactive({...props,path:'flow.yaml',theme:document.documentElement.classList.contains('dark')?'dark':'light'}));
 useEffect(()=>{const app=createApp({setup:()=>()=>h(NativeCodeEditor,state.current)});registerNativePanels(app);app.use(createI18n({legacy:false,locale:'zh',missingWarn:false,fallbackWarn:false,messages:{zh:{'Fold content lines':'折叠代码','Unfold content lines':'展开代码',close:'关闭','select view':'选择视图','panel actions':'面板操作',editor_shortcuts:{label:'快捷键'},empty:{panels:{title:'选择编辑视图',content:'从顶部打开 YAML 或 Flow 结构面板'}},multi_panel_editor:{move_left:'移至左侧面板',move_right:'移至右侧面板',close_all_tabs:'关闭此面板标签',close_all_panels:'关闭全部面板',fullscreen:'最大化面板',exit_fullscreen:'恢复面板布局'}}}}));app.mount(element.current!);const observer=new MutationObserver(()=>{state.current.theme=document.documentElement.classList.contains('dark')?'dark':'light'});observer.observe(document.documentElement,{attributes:true,attributeFilter:['class']});return()=>{observer.disconnect();app.unmount()}},[]);
 useEffect(()=>{Object.assign(state.current,props)},[props]);
 return <div className="keel-kestra" ref={element}/>;
}
