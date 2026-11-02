import {defineConfig} from 'vite';
import vue from '@vitejs/plugin-vue';
import prefix from 'postcss-prefix-selector';
import {fileURLToPath,URL} from 'node:url';
export default defineConfig({root:fileURLToPath(new URL('.',import.meta.url)),base:'./',plugins:[vue()],resolve:{dedupe:['vue']},css:{postcss:{plugins:[prefix({prefix:'.keel-kestra',includeFiles:[/vendor\/kestra/,/jobs-native/],transform:(p:string,s:string,prefixed:string)=>s===':root'?'html.jp-preview-document':s==='html.dark'?'html.jp-preview-document.dark':s.startsWith('.keel-kestra')?s:prefixed}),prefix({prefix:'.jp-preview-document',includeFiles:[/prototype\/native-foundation/],transform:(p:string,s:string,prefixed:string)=>s===':root'?'html.jp-preview-document':s.startsWith('html.jp-preview-document')?s:prefixed})]}},build:{target:'esnext',outDir:'dist',emptyOutDir:true}});
