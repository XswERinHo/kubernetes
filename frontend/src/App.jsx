import { useState, useMemo } from 'react';
import { Routes, Route, Navigate } from 'react-router-dom';
import { ThemeProvider, createTheme, CssBaseline } from '@mui/material';

import MainLayout from './components/MainLayout';
import Dashboard from './views/Dashboard';
import Workloads from './views/Workloads';
import Settings from './views/Settings';
import Nodes from './views/Nodes';
import Approvals from './views/Approvals';
import Alerts from './views/Alerts';
import { ThemeModeContext } from './context/ThemeContext';
import { ClusterProvider } from './context/ClusterContext';
import { AuthProvider } from './context/AuthContext'; 
import Login from './views/Login'; 
import ProtectedRoute from './components/ProtectedRoute';
import AdminRoute from './components/AdminRoute';

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
          ...(mode === 'dark' ? {
            background: {
              default: '#0f172a',
              paper: 'rgba(30, 41, 59, 0.7)',
            },
            primary: {
              main: '#38bdf8',
            },
            secondary: {
              main: '#818cf8',
            },
            text: {
              primary: '#f1f5f9',
              secondary: '#94a3b8',
            },
          } : {
            background: {
              default: '#f0f2f5',
              paper: 'rgba(255, 255, 255, 0.8)',
            },
          }),
        },
        shape: {
          borderRadius: 16,
        },
        typography: {
          fontFamily: '"Inter", "Roboto", "Helvetica", "Arial", sans-serif',
          h4: {
            fontWeight: 700,
            letterSpacing: '-0.02em',
          },
          h6: {
            fontWeight: 600,
          },
        },
        components: {
          MuiCssBaseline: {
            styleOverrides: {
              body: {
                backgroundImage: mode === 'dark' 
                  ? 'radial-gradient(circle at 50% 0%, #1e293b 0%, #0f172a 100%)' 
                  : 'linear-gradient(180deg, #f0f2f5 0%, #e2e8f0 100%)',
                backgroundAttachment: 'fixed',
              },
            },
          },
          MuiPaper: {
            styleOverrides: {
              root: {
                backdropFilter: 'blur(12px)',
                border: mode === 'dark' ? '1px solid rgba(255, 255, 255, 0.08)' : '1px solid rgba(255, 255, 255, 0.4)',
                boxShadow: mode === 'dark' ? '0 4px 30px rgba(0, 0, 0, 0.1)' : '0 4px 30px rgba(0, 0, 0, 0.05)',
                backgroundImage: 'none',
              },
            },
          },
          MuiButton: {
            styleOverrides: {
              root: {
                textTransform: 'none',
                fontWeight: 600,
                borderRadius: 12,
              },
              contained: {
                boxShadow: 'none',
                '&:hover': {
                  boxShadow: '0 4px 12px rgba(0, 0, 0, 0.15)',
                },
              },
            },
          },
          MuiTableCell: {
            styleOverrides: {
              root: {
                borderBottom: mode === 'dark' ? '1px solid rgba(255, 255, 255, 0.05)' : '1px solid rgba(0, 0, 0, 0.05)',
              },
              head: {
                fontWeight: 600,
                color: mode === 'dark' ? '#94a3b8' : '#64748b',
              },
            },
          },
          MuiChip: {
            styleOverrides: {
              root: {
                fontWeight: 500,
              },
            },
          },
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
                  <Route path="nodes" element={<Nodes />} />
                  <Route path="alerts" element={<Alerts />} />
                  <Route
                    path="approvals"
                    element={(
                      <AdminRoute>
                        <Approvals />
                      </AdminRoute>
                    )}
                  />
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