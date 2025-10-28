import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
// --- NOWE IMPORTY Z MUI ---
import { ThemeProvider, createTheme, CssBaseline } from '@mui/material';
// --- KONIEC NOWYCH IMPORTÓW ---
import './index.css';
import App from './App.jsx';

// --- DEFINICJA CIEMNEGO MOTYWU ---
const darkTheme = createTheme({
  palette: {
    mode: 'dark', // Włącza ciemny tryb
  },
});
// --- KONIEC DEFINICJI ---

createRoot(document.getElementById('root')).render(
  <StrictMode>
    {/* Otaczamy aplikację dostawcą motywu */}
    <ThemeProvider theme={darkTheme}>
      {/* CssBaseline resetuje domyślne style przeglądarki i stosuje tło z motywu */}
      <CssBaseline />
      <App />
    </ThemeProvider>
  </StrictMode>,
);