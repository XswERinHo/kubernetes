import { useState, useMemo } from 'react';
import { Routes, Route, Navigate } from 'react-router-dom';
import { ThemeProvider, createTheme, CssBaseline } from '@mui/material';

import MainLayout from './components/MainLayout';
import Dashboard from './views/Dashboard';
import Workloads from './views/Workloads';
import Settings from './views/Settings';
import { ThemeModeContext } from './context/ThemeContext';
import { ClusterProvider } from './context/ClusterContext';
import { AuthProvider } from './context/AuthContext'; // <-- NOWY IMPORT
import Login from './views/Login'; // <-- NOWY IMPORT
import ProtectedRoute from './components/ProtectedRoute'; // <-- NOWY IMPORT

function App() {
  const [mode, setMode] = useState('dark');

  const themeModeApi = useMemo(
    () => ({
      mode,
      toggleThemeMode: () => {
        setMode((prevMode) => (prevMode === 'light' ? 'dark' : 'light'));
      },
    }),
    [mode],
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
    <ThemeModeContext.Provider value={themeModeApi}>
      <ThemeProvider theme={theme}>
        <CssBaseline />
        {/* AuthProvider musi być na zewnątrz ClusterProvider, 
            ponieważ ClusterProvider będzie potrzebował tokenu do wysłania zapytania */}
        <AuthProvider>
          <ClusterProvider>
            <Routes>
              {/* --- NOWA STRUKTURA ROUTINGU --- */}
              
              {/* Ścieżka publiczna do logowania */}
              <Route path="/login" element={<Login />} />

              {/* Ścieżki chronione */}
              <Route element={<ProtectedRoute />}>
                <Route path="/" element={<MainLayout />}>
                  <Route path="dashboard" element={<Dashboard />} />
                  <Route path="workloads" element={<Workloads />} />
                  <Route path="settings" element={<Settings />} />
                  
                  {/* Przekierowanie z / na /dashboard */}
                  <Route index element={<Navigate to="/dashboard" replace />} />
                </Route>
              </Route>

            </Routes>
          </ClusterProvider>
        </AuthProvider>
      </ThemeProvider>
    </ThemeModeContext.Provider>
  );
}

export default App;