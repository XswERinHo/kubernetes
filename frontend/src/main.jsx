import { StrictMode, Suspense } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import './index.css';
import App from './App.jsx';
import './i18n';

import LoaderFallback from './components/LoaderFallback';

createRoot(document.getElementById('root')).render(
  <StrictMode>
    <Suspense fallback={<LoaderFallback />}>
      <BrowserRouter>
        <App />
      </BrowserRouter>
    </Suspense>
  </StrictMode>,
);