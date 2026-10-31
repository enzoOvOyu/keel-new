declare module '*.vue' { const component: import('vue').DefineComponent<any,any,any>; export default component; }
declare module '*?worker' { const WorkerConstructor: {new():Worker}; export default WorkerConstructor; }
