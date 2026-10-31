import {useEffect,useRef} from 'react';
import {createApp,h,reactive} from 'vue';
import {createI18n} from 'vue-i18n';
import NativeOutputs from './NativeOutputs.vue';
import './native-theme.scss';
export function NativeOutputsHost({outputs}:{outputs:Record<string,unknown>}){
 const element=useRef<HTMLDivElement>(null),state=useRef(reactive({outputs}));
 useEffect(()=>{const app=createApp({setup:()=>()=>h(NativeOutputs,state.current)});app.use(createI18n({legacy:false,locale:'zh',messages:{zh:{variable_explorer:{search_placeholder:'搜索输出名称或内容',empty:'没有匹配的输出'},ks_no_data:{no_results:'暂无记录',nothing_here:'当前尚无输出',will_appear:'任务生成输出后可在此查看'}}}}));app.mount(element.current!);return()=>app.unmount()},[]);
 useEffect(()=>{state.current.outputs=outputs},[outputs]);
 return <div className="keel-kestra" ref={element}/>;
}
