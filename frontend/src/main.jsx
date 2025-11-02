import { StrictMode, Suspense } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import './index.css';
import App from './App.jsx';
import './i18n'; // Importujemy i inicjujemy i18n

// --- ZMIANA TUTAJ ---
// Importujemy komponent zamiast go definiować
import LoaderFallback from './components/LoaderFallback'; 
// --- KONIEC ZMIANY ---


// Usunęliśmy definicję komponentu LoaderFallback stąd

createRoot(document.getElementById('root')).render(
  <StrictMode>
    <Suspense fallback={<LoaderFallback />}>
      <BrowserRouter>
        <App />
      </BrowserRouter>
    </Suspense>
  </StrictMode>,
);