import {defineConfig} from 'vite';
import react from '@vitejs/plugin-react';
import tailwind from '@tailwindcss/vite';
import {fileURLToPath,URL} from 'node:url';
export default defineConfig({plugins:[react(),tailwind()],resolve:{alias:{'@':fileURLToPath(new URL('../../../dev/src',import.meta.url))},dedupe:['react','react-dom']},server:{host:'0.0.0.0',port:5181,strictPort:true,fs:{allow:[fileURLToPath(new URL('../../../',import.meta.url))]}},build:{target:'esnext'},});
