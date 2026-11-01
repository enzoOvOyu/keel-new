import {defineConfig} from 'vite';
import vue from '@vitejs/plugin-vue';
import prefix from 'postcss-prefix-selector';
import {fileURLToPath,URL} from 'node:url';
export default defineConfig({root:fileURLToPath(new URL('.',import.meta.url)),base:'/jobs-preview/',plugins:[vue()],resolve:{dedupe:['vue']},css:{postcss:{plugins:[prefix({prefix:'.keel-kestra',includeFiles:[/vendor\/kestra/,/jobs-native/],transform:(p:string,s:string,prefixed:string)=>s===':root'?p:s==='html.dark'?'html.dark '+p:s.startsWith('.keel-kestra')?s:prefixed})]}},build:{target:'esnext',outDir:'dist',emptyOutDir:true}});
