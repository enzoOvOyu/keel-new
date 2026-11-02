import React from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom';
import '../../src/style.css';
import KickoffStudio from './KickoffStudio';

createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <BrowserRouter>
      <Routes>
        <Route path="/kickoff-studio" element={<KickoffStudio />} />
        <Route path="*" element={<Navigate to="/kickoff-studio" replace />} />
      </Routes>
    </BrowserRouter>
  </React.StrictMode>,
);
