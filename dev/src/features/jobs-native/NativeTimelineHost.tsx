import {useEffect,useRef} from 'react';
import {createApp,h,reactive} from 'vue';
import {createI18n} from 'vue-i18n';
import NativeTimeline from './NativeTimeline.vue';
import './native-theme.scss';
export function NativeTimelineHost({histories}:{histories:{date:string;state:string}[]}) {
 const element=useRef<HTMLDivElement>(null),state=useRef(reactive({histories}));
 useEffect(()=>{const app=createApp({setup:()=>()=>h(NativeTimeline,state.current)});app.use(createI18n({legacy:false,locale:'zh',messages:{zh:{'see timeline':'状态时间线','run timeline':'执行状态历史',close:'关闭'}}}));app.mount(element.current!);return()=>app.unmount()},[]);
 useEffect(()=>{state.current.histories=histories},[histories]);
 return <div className="keel-kestra" ref={element}/>;
}
