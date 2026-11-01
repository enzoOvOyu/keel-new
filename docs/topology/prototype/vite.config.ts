import {defineConfig,type Plugin} from 'vite';
import react from '@vitejs/plugin-react';
import tailwind from '@tailwindcss/vite';
import {fileURLToPath,URL} from 'node:url';
// The independent jobs HTML must resolve before this prototype's SPA fallback.
const jobsPreviewRoute:Plugin={name:'keel-jobs-preview-route',configureServer(server){server.middlewares.use((req,res,next)=>{const path=req.url?.split('?')[0];if(path==='/jobs-preview'||path==='/jobs-preview/'){res.statusCode=302;res.setHeader('Cache-Control','no-store');res.setHeader('Location',(path.endsWith('/')?'index.html':'jobs-preview/index.html')+(req.url?.slice(path.length)||''));res.end();return}next()})}};
export default defineConfig({plugins:[react(),tailwind(),jobsPreviewRoute],resolve:{alias:{'@':fileURLToPath(new URL('../../../dev/src',import.meta.url))},dedupe:['react','react-dom']},server:{host:'0.0.0.0',port:5181,strictPort:true,fs:{allow:[fileURLToPath(new URL('../../../',import.meta.url)),process.env.KEEL_PROTOTYPE_SOURCE_ROOT||'/home/ubuntu/keel']}},build:{target:'esnext'},});
