import React from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import './index.css'
const queryClient = new QueryClient()
createRoot(document.getElementById('root')!).render(<React.StrictMode><QueryClientProvider client={queryClient}><BrowserRouter><main className="mx-auto max-w-4xl p-10"><p className="text-sm font-semibold text-slate-500">KEEL · 开发环境</p><h1 className="mt-3 text-2xl font-semibold">前端基础环境已就绪</h1><p className="mt-3 text-slate-600">按设计文档从 P0 全局壳层和 mock API 开始开发。</p></main></BrowserRouter></QueryClientProvider></React.StrictMode>)
