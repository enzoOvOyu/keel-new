import {useEffect,useRef,useState} from 'react';
import * as T from 'three';
import {OrbitControls} from 'three/addons/controls/OrbitControls.js';
import {Box,Expand,Focus,Minus,Plus,ScanLine} from 'lucide-react';
import {Button} from '../../shared';
import type {RoomRack} from '../room-model';
import {shortPod} from '../model';

type Props={racks:RoomRack[];visible:string[];selected:string;angle:string;onAngle:(angle:string)=>void;onSelect:(id:string)=>void;onOpen:(id:string)=>void};
type RackObject={group:T.Group;outline:T.LineSegments;cap:T.Mesh;label:HTMLButtonElement;point:T.Vector3};
type Runtime={render:()=>void;focus:()=>void;home:(top:boolean)=>void;zoom:(delta:number)=>void;objects:Map<string,RackObject>};
export default function RoomScene(props:Props){
  const host=useRef<HTMLDivElement>(null),labels=useRef<HTMLDivElement>(null),runtime=useRef<Runtime|null>(null),latest=useRef(props);
  latest.current=props;const [error,setError]=useState('');
  useEffect(()=>{
    const target=host.current!,labelRoot=labels.current!;
    let renderer:T.WebGLRenderer;
    try{renderer=new T.WebGLRenderer({antialias:true,alpha:true});}catch{setError('当前浏览器无法创建 WebGL2 场景。可返回资产阵列查看相同机柜。');return;}
    renderer.setPixelRatio(Math.min(devicePixelRatio,2));renderer.setClearColor('#edf1f6');
    renderer.shadowMap.enabled=true;renderer.shadowMap.type=T.PCFSoftShadowMap;
    renderer.domElement.setAttribute('aria-label','机房三维机柜场景');renderer.domElement.tabIndex=0;
    target.prepend(renderer.domElement);
    const scene=new T.Scene(),camera=new T.OrthographicCamera(-7,7,7,-7,.1,100);
    const controls=new OrbitControls(camera,renderer.domElement);controls.minZoom=.7;controls.maxZoom=3.8;
    controls.minPolarAngle=.02;controls.maxPolarAngle=Math.PI*.45;controls.enableDamping=false;
    controls.listenToKeyEvents(renderer.domElement);
    const ambient=new T.HemisphereLight('#eff6ff','#6f819b',2.5);scene.add(ambient);
    const light=new T.DirectionalLight('#ffffff',3.2);light.position.set(1,12,8);light.castShadow=true;
    light.shadow.mapSize.set(2048,2048);Object.assign(light.shadow.camera,{left:-10,right:10,top:10,bottom:-10,near:1,far:35});light.shadow.bias=-.001;scene.add(light);
    const geometries:T.BufferGeometry[]=[],materials:T.Material[]=[];
    const box=new T.BoxGeometry(1,1,1);geometries.push(box);
    const material=(color:string)=>{const m=new T.MeshStandardMaterial({color,roughness:.8,metalness:.12});materials.push(m);return m};
    const floorMat=material('#dce3ec'),frame=material('#27384d'),side=material('#405167'),blank=material('#1a2a3d'),server=material('#8296b3'),network=material('#7caaa6');
    const cube=(parent:T.Object3D,w:number,h:number,d:number,x:number,y:number,z:number,m:T.Material)=>{const mesh=new T.Mesh(box,m);mesh.scale.set(w,h,d);mesh.position.set(x,y,z);mesh.castShadow=true;mesh.receiveShadow=true;parent.add(mesh);return mesh};
    const floorWidth=Math.max(10,...props.racks.map(r=>Math.abs(r.x)*2+3)),floorDepth=Math.max(6,...props.racks.map(r=>Math.abs(r.z)*2+3.2));
    cube(scene,floorWidth,.12,floorDepth,0,-.08,0,floorMat);
    const grid=new T.GridHelper(Math.max(floorWidth,floorDepth),24,'#b6c2d1','#c9d2de');grid.position.y=-.01;grid.scale.set(floorWidth/Math.max(floorWidth,floorDepth),1,floorDepth/Math.max(floorWidth,floorDepth));scene.add(grid);geometries.push(grid.geometry);materials.push(...(Array.isArray(grid.material)?grid.material:[grid.material]));
    const objects=new Map<string,RackObject>(),pickable:T.Object3D[]=[];
    const podLabels:{element:HTMLDivElement;point:T.Vector3;pod:string}[]=[];
    for(const pod of [...new Set(props.racks.map(r=>r.pod))]){
      const members=props.racks.filter(r=>r.pod===pod),z=members[0].z;
      const band=material('#cdd8e8');cube(scene,7.1,.018,1.45,0,.006,z,band);
      const element=document.createElement('div');element.className='room-pod-label';element.textContent=shortPod(pod);labelRoot.append(element);
      podLabels.push({element,point:new T.Vector3(-4.3,.02,z),pod});
    }
    for(const item of props.racks){
      const s=item.snapshot,h=item.height,group=new T.Group();group.position.set(item.x,.05,item.z);group.userData.rack=s.rack.id;scene.add(group);
      cube(group,.88,.08,1.12,0,.04,0,frame);cube(group,.88,.075,1.12,0,h+.12,0,side);
      for(const x of [-.42,.42])for(const z of [-.52,.52])cube(group,.045,h,.045,x,h/2+.08,z,frame);
      for(const x of [-.43,.43])cube(group,.025,h-.03,1.04,x,h/2+.08,0,side);
      for(const z of [-.53,.53]){
        cube(group,.78,h,.025,0,h/2+.08,z,blank);
        // Known U/height become face strips only. Device depth and mounting face remain unknown.
        for(const p of s.layout?.placed||[]){
          const d=s.items.find(d=>d.id===p.id)!;
          cube(group,.75,p.height/20-.012,.018,0,(p.u-1+p.height/2)/20+.08,z*1.035,d.type==='switch'?network:server);
        }
      }
      const ratio=s.height&&s.layout?s.layout.occupied/s.height:0;
      const tint=new T.Color('#8dbec5').lerp(new T.Color('#766cc2'),ratio);
      const capMat=material('#'+tint.getHexString());
      const cap=cube(group,.78,.025,1.01,0,h+.17,0,capMat);
      // Roof capacity bar, sourced from the shared U interval union.
      cube(group,.7,.026,.12,0,h+.19,.29,frame);
      if(ratio>0)cube(group,.7*ratio,.027,.12,-.35+.35*ratio,h+.194,.29,server);
      const selectionBox=new T.BoxGeometry(.98,h+.25,1.22);const edges=new T.EdgesGeometry(selectionBox);selectionBox.dispose();geometries.push(edges);
      const edgeMat=new T.LineBasicMaterial({color:'#7b70df',depthTest:false});materials.push(edgeMat);
      const outline=new T.LineSegments(edges,edgeMat);outline.position.y=h/2+.11;outline.renderOrder=3;outline.visible=false;group.add(outline);
      const label=document.createElement('button');label.className='room-rack-label';label.dataset.rack=s.rack.id;
      label.innerHTML=`<b>${s.rack.name}</b><span>${s.layout?.occupied??'—'} / ${s.height??'—'}U</span>`;
      label.setAttribute('aria-label',`选择 ${s.rack.name}`);label.addEventListener('click',()=>latest.current.onSelect(s.rack.id));label.addEventListener('dblclick',()=>latest.current.onOpen(s.rack.id));labelRoot.append(label);
      objects.set(s.rack.id,{group,outline,cap,label,point:new T.Vector3(item.x,h+.44,item.z)});
      group.traverse(o=>{if(o instanceof T.Mesh){o.userData.rack=s.rack.id;pickable.push(o)}});
    }
    let width=1,height=1;
    const project=(node:HTMLElement,p:T.Vector3,visible:boolean)=>{
      const v=p.clone().project(camera);const inFrame=visible&&v.z>=-1&&v.z<=1&&Math.abs(v.x)<1.05&&Math.abs(v.y)<1.05;
      node.style.display=inFrame?'':'none';node.style.transform=`translate(${(v.x*.5+.5)*width}px,${(-v.y*.5+.5)*height}px) translate(-50%,-100%)`;
    };
    function render(){
      const visible=new Set(latest.current.visible);
      for(const [id,o] of objects){o.group.visible=visible.has(id);o.outline.visible=id===latest.current.selected;o.label.classList.toggle('selected',id===latest.current.selected);o.label.setAttribute('aria-pressed',String(id===latest.current.selected));project(o.label,o.point,o.group.visible);}
      for(const p of podLabels)project(p.element,p.point,props.racks.some(r=>r.pod===p.pod&&visible.has(r.snapshot.rack.id)));
      renderer.render(scene,camera);
      target.dataset.rendered='true';target.dataset.camera=latest.current.angle;
      target.dataset.visibleRacks=String(visible.size);
    }
    function home(top:boolean){controls.target.set(0,.4,0);camera.zoom=1;camera.position.set(top?0:8,top?22:16,top?.001:18);camera.up.set(0,1,0);camera.updateProjectionMatrix();controls.update();render();}
    function focus(){const o=objects.get(latest.current.selected);if(!o)return;const offset=camera.position.clone().sub(controls.target);controls.target.copy(o.group.position).add(new T.Vector3(0,.7,0));camera.position.copy(controls.target).add(offset);camera.zoom=1.9;camera.updateProjectionMatrix();controls.update();render();}
    function zoom(delta:number){camera.zoom=T.MathUtils.clamp(camera.zoom+delta,.7,3.8);camera.updateProjectionMatrix();render();}
    runtime.current={render,home,focus,zoom,objects};
    const resize=new ResizeObserver(()=>{width=target.clientWidth;height=target.clientHeight;if(!width||!height)return;const aspect=width/height,half=6.7;camera.left=-half*aspect;camera.right=half*aspect;camera.top=half;camera.bottom=-half;camera.updateProjectionMatrix();renderer.setSize(width,height);render();});resize.observe(target);
    controls.addEventListener('change',render);home(latest.current.angle==='top');
    const ray=new T.Raycaster();let down={x:0,y:0};
    const pointerDown=(e:PointerEvent)=>{down={x:e.clientX,y:e.clientY}};
    const hit=(e:MouseEvent)=>{const r=renderer.domElement.getBoundingClientRect();ray.setFromCamera(new T.Vector2((e.clientX-r.left)/r.width*2-1,-(e.clientY-r.top)/r.height*2+1),camera);return ray.intersectObjects(pickable.filter(o=>o.parent?.visible),false)[0]?.object.userData.rack as string|undefined};
    const pointerUp=(e:PointerEvent)=>{if(Math.hypot(e.clientX-down.x,e.clientY-down.y)>4)return;const id=hit(e);if(id)latest.current.onSelect(id)};
    const doubleClick=(e:MouseEvent)=>{const id=hit(e);if(id)latest.current.onOpen(id)};
    renderer.domElement.addEventListener('pointerdown',pointerDown);renderer.domElement.addEventListener('pointerup',pointerUp);renderer.domElement.addEventListener('dblclick',doubleClick);
    return()=>{runtime.current=null;resize.disconnect();controls.dispose();renderer.domElement.removeEventListener('pointerdown',pointerDown);renderer.domElement.removeEventListener('pointerup',pointerUp);renderer.domElement.removeEventListener('dblclick',doubleClick);for(const g of geometries)g.dispose();for(const m of materials)m.dispose();renderer.dispose();renderer.forceContextLoss();renderer.domElement.remove();labelRoot.replaceChildren();};
  },[props.racks]);
  useEffect(()=>{runtime.current?.render()},[props.visible,props.selected]);
  useEffect(()=>{runtime.current?.home(props.angle==='top')},[props.angle]);
  return <section className="room-scene-shell">
    <div className="room-scene-toolbar"><div className="rv-segment" role="tablist" aria-label="机房视角"><button role="tab" aria-selected={props.angle!=='top'} onClick={()=>props.onAngle('oblique')}><Box size={13}/>3D 俯视</button><button role="tab" aria-selected={props.angle==='top'} onClick={()=>props.onAngle('top')}><ScanLine size={13}/>垂直俯视</button></div><span className="spacer"/><Button onClick={()=>runtime.current?.focus()} disabled={!props.selected}><Focus size={13}/>聚焦机柜</Button><Button aria-label="适配机房视图" onClick={()=>runtime.current?.home(props.angle==='top')}><Expand size={14}/></Button></div>
    <div className="room-scene" ref={host}>
      <div className="room-label-layer" ref={labels}/>
      {error&&<div className="room-webgl-error" role="alert">{error}</div>}
      <div className="room-view-badge">示意排布 <span>机房坐标尚未采集</span></div>
      <div className="room-controls"><Button aria-label="缩小机房" onClick={()=>runtime.current?.zoom(-.2)}><Minus size={14}/></Button><Button aria-label="放大机房" onClick={()=>runtime.current?.zoom(.2)}><Plus size={14}/></Button></div>
      {!props.visible.length&&<div className="room-no-match">当前筛选没有匹配机柜</div>}
    </div>
    <div className="room-scene-footer"><span>拖动旋转 · Shift + 拖动平移 · 滚轮缩放 · 双击进入机柜</span><span>布局与宽深为示意，不用于安装测量</span></div>
  </section>;
}
