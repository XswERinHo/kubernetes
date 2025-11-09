import { useState, useMemo } from 'react';
import { Routes, Route, Navigate } from 'react-router-dom';
import { ThemeProvider, createTheme, CssBaseline } from '@mui/material';

import MainLayout from './components/MainLayout';
import Dashboard from './views/Dashboard';
import Workloads from './views/Workloads';
import Settings from './views/Settings';
import Nodes from './views/Nodes'; // <-- NOWY IMPORT
import { ThemeModeContext } from './context/ThemeContext';
import { ClusterProvider } from './context/ClusterContext';
import { AuthProvider } from './context/AuthContext'; 
import Login from './views/Login'; 
import ProtectedRoute from './components/ProtectedRoute';

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
        <AuthProvider>
          <ClusterProvider>
            <Routes>
              
              <Route path="/login" element={<Login />} />

              <Route element={<ProtectedRoute />}>
                <Route path="/" element={<MainLayout />}>
                  <Route path="dashboard" element={<Dashboard />} />
                  <Route path="workloads" element={<Workloads />} />
                  <Route path="nodes" element={<Nodes />} /> {/* <-- NOWA ŚCIEŻKA */}
                  <Route path="settings" element={<Settings />} />
                  
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