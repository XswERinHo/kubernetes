// frontend/src/components/MainLayout.jsx

import { useState, useEffect, useCallback } from 'react'; // <-- IMPORT useCallback
import { Outlet } from 'react-router-dom';
import { Box, Drawer, AppBar, Toolbar, Typography, CssBaseline, CircularProgress, Alert } from '@mui/material';
import NavList from './NavList';
import { useTranslation } from 'react-i18next';
import { useCluster } from '../context/ClusterContext'; 
import ClusterSelector from './ClusterSelector'; 

const drawerWidth = 240;

export default function MainLayout() {
  const { t } = useTranslation();
  
  const { selectedCluster, error: clusterError } = useCluster();

  const [workloads, setWorkloads] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  // --- ZMIANA: Opakowujemy fetchData w useCallback ---
  const fetchData = useCallback(() => {
    if (!selectedCluster) {
      setLoading(false); // Upewnij się, że ładowanie jest wyłączone, jeśli nie ma klastra
      return;
    }

    setLoading(true);
    setError(null);
    
    fetch(`/api/clusters/${selectedCluster}/workloads`)
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
  }, [selectedCluster]); // <-- Deklarujemy zależność od selectedCluster
  
  // --- ZMIANA: Dodajemy fetchData do tablicy zależności ---
  useEffect(() => {
    setWorkloads([]);
    setError(null);
    
    if (selectedCluster) {
      fetchData();
    } else if (clusterError) {
      setError(clusterError);
      setLoading(false);
    }
  }, [selectedCluster, clusterError, fetchData]); // <-- DODANO fetchData
  // --- KONIEC ZMIAN ---


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
            <Typography variant="h6">{t('main_layout.error_title')}</Typography>
            <pre>{error}</pre>
          </Alert>
        </Box>
      );
    }

    return <Outlet context={{ workloads, fetchData, selectedCluster }} />;
  };

  return (
    <Box sx={{ display: 'flex' }}>
      <CssBaseline />
      <AppBar
        position="fixed"
        sx={{ width: `calc(100% - ${drawerWidth}px)`, ml: `${drawerWidth}px` }}
      >
        <Toolbar>
          <Typography variant="h6" noWrap component="div" sx={{ flexGrow: 1 }}>
            K8s Resource Manager
          </Typography>
          
          <ClusterSelector />
          
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
        
        {renderContent()}
      </Box>
    </Box>
  );
}