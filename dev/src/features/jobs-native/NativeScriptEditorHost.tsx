import {useEffect,useRef} from 'react';
import {createApp,h,reactive} from 'vue';
import {createI18n} from 'vue-i18n';
import NativeCodeEditor from './NativeCodeEditor.vue';
import './monaco-workers';
import './native-theme.scss';
type Props={source:string;path:string;readOnly:boolean;onChange:(source:string)=>void;onSave:()=>void};
export function NativeScriptEditorHost(props:Props){
 const root=useRef<HTMLDivElement>(null);
 const state=useRef(reactive({...props,language:language(props.path),theme:document.documentElement.classList.contains('dark')?'dark':'light'}));
 useEffect(()=>{const app=createApp({setup:()=>()=>h(NativeCodeEditor,state.current)});app.use(createI18n({legacy:false,locale:'zh',missingWarn:false,fallbackWarn:false,messages:{zh:{editor_shortcuts:{label:'快捷键'}}}}));app.mount(root.current!);const observer=new MutationObserver(()=>{state.current.theme=document.documentElement.classList.contains('dark')?'dark':'light'});observer.observe(document.documentElement,{attributes:true,attributeFilter:['class']});return()=>{observer.disconnect();app.unmount()}},[]);
 useEffect(()=>{Object.assign(state.current,props,{language:language(props.path)})},[props]);
 return <div className="keel-kestra" ref={root}/>;
}
function language(path:string){return path.endsWith('.py')?'python':/\.(sh|bash)$/.test(path)?'shell':'plaintext'}
