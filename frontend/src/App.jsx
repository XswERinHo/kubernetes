import { useState, useMemo } from 'react';
import { Routes, Route, Navigate } from 'react-router-dom';
import { ThemeProvider, createTheme, CssBaseline } from '@mui/material';

import MainLayout from './components/MainLayout';
import Dashboard from './views/Dashboard';
import Workloads from './views/Workloads';
import Settings from './views/Settings';
import { ThemeModeContext } from './context/ThemeContext';

function App() {
  const [mode, setMode] = useState('dark');

  // Obiekt z funkcją ORAZ aktualnym trybem
  const themeModeApi = useMemo(
    () => ({
      mode, // Przekazujemy aktualny tryb
      toggleThemeMode: () => {
        setMode((prevMode) => (prevMode === 'light' ? 'dark' : 'light'));
      },
    }),
    [mode], // Zależność od 'mode'
  );

  const theme = useMemo(
    () =>
      createTheme({
        palette: {
          mode: mode,
        },
      }),
    [mode],
  );

  return (
    // Przekazujemy nowy obiekt {mode, toggleThemeMode} do kontekstu
    <ThemeModeContext.Provider value={themeModeApi}>
      <ThemeProvider theme={theme}>
        <CssBaseline />
        <Routes>
          <Route path="/" element={<MainLayout />}>
            <Route path="dashboard" element={<Dashboard />} />
            <Route path="workloads" element={<Workloads />} />
            
            {/* Usunęliśmy prop 'currentMode', Settings pobierze go z kontekstu */}
            <Route path="settings" element={<Settings />} />
            
            <Route index element={<Navigate to="/dashboard" replace />} />
          </Route>
        </Routes>
      </ThemeProvider>
    </ThemeModeContext.Provider>
  );
}

export default App;