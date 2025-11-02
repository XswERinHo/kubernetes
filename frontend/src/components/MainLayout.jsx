// Pełna zawartość pliku:
// frontend/src/components/MainLayout.jsx

import { useState, useEffect } from 'react';
import { Outlet } from 'react-router-dom';
import { Box, Drawer, AppBar, Toolbar, Typography, CssBaseline, CircularProgress, Alert } from '@mui/material';
import NavList from './NavList';

const drawerWidth = 240;

export default function MainLayout() {
  // --- POCZĄTEK NOWEJ LOGIKI ---
  // Przenosimy stany z Workloads.jsx do layoutu nadrzędnego
  const [workloads, setWorkloads] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  // Przenosimy funkcję pobierania danych
  const fetchData = () => {
    setLoading(true);
    setError(null);
    fetch('/api/workloads')
      .then(response => {
        if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);
        return response.json();
      })
      .then(data => {
        const workloadsWithRecs = (data || []).map(dep => ({ ...dep, recommendations: dep.recommendations || [] }));
        setWorkloads(workloadsWithRecs);
      })
      .catch(error => {
        console.error('Error fetching data:', error);
        setError(`Failed to load workloads: ${error.message}`);
      })
      .finally(() => setLoading(false));
  };
  
  // Pobieramy dane przy pierwszym załadowaniu layoutu
  useEffect(() => {
    fetchData();
  }, []);
  // --- KONIEC NOWEJ LOGIKI ---

  // Funkcja pomocnicza do renderowania treści
  const renderContent = () => {
    if (loading) {
      return (
        <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '80vh' }}>
          <CircularProgress />
        </Box>
      );
    }

    if (error) {
      return (
        <Box sx={{ p: 3 }}>
          <Alert severity="error" variant="filled">
            <Typography variant="h6">Failed to connect to backend</Typography>
            <pre>{error.message}</pre>
          </Alert>
        </Box>
      );
    }

    // Jeśli dane są gotowe, przekazujemy je do pod-widoków (Dashboard/Workloads)
    // Używamy "Outlet context" do przekazania danych i funkcji odświeżania
    return <Outlet context={{ workloads, fetchData }} />;
  };

  return (
    <Box sx={{ display: 'flex' }}>
      <CssBaseline />
      <AppBar
        position="fixed"
        sx={{ width: `calc(100% - ${drawerWidth}px)`, ml: `${drawerWidth}px` }}
      >
        <Toolbar>
          <Typography variant="h6" noWrap component="div">
            K8s Resource Manager
          </Typography>
        </Toolbar>
      </AppBar>
      
      <Drawer
        sx={{
          width: drawerWidth,
          flexShrink: 0,
          '& .MuiDrawer-paper': {
            width: drawerWidth,
            boxSizing: 'border-box',
          },
        }}
        variant="permanent"
        anchor="left"
      >
        <Toolbar /> 
        <NavList />
      </Drawer>
      
      <Box
        component="main"
        sx={{ flexGrow: 1, bgcolor: 'background.default', p: 3 }}
      >
        <Toolbar />
        
        {/* Renderujemy treść (Spinner, Błąd lub Outlet) */}
        {renderContent()}
      </Box>
    </Box>
  );
}