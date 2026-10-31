// Flow editing currently uses YAML. Load only its generic editor worker;
// language-specific workers are added with the corresponding editor capability.
import EditorWorker from 'monaco-editor/editor/editor.worker?worker';
(globalThis as any).MonacoEnvironment={getWorker(_:unknown,label:string){
 if(['json','typescript','javascript'].includes(label))throw Error('该语言的语义 Worker 尚未启用');
 return new EditorWorker();
}};
