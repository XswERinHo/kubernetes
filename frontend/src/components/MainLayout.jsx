// frontend/src/components/MainLayout.jsx

import { useState, useEffect, useCallback } from 'react';
import { Outlet, useNavigate } from 'react-router-dom'; // <-- IMPORT useNavigate
import { 
  Box, Drawer, AppBar, Toolbar, Typography, CssBaseline, 
  CircularProgress, Alert, Button, Tooltip, IconButton
} from '@mui/material';
import LogoutIcon from '@mui/icons-material/Logout'; // <-- NOWY IMPORT
import NavList from './NavList';
import { useTranslation } from 'react-i18next';
import { useCluster } from '../context/ClusterContext'; 
import ClusterSelector from './ClusterSelector'; 
import { useAuth } from '../context/AuthContext'; // <-- NOWY IMPORT

const drawerWidth = 240;

export default function MainLayout() {
  const { t } = useTranslation();
  const navigate = useNavigate(); // <-- NOWY HOOK
  
  const { selectedCluster, error: clusterError } = useCluster();
  // --- NOWA LOGIKA: Pobieramy dane autoryzacji ---
  const { userRole, logout, getAuthHeader } = useAuth(); 

  const [workloads, setWorkloads] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  const fetchData = useCallback(() => {
    if (!selectedCluster) {
      setLoading(false); 
      return;
    }

    setLoading(true);
    setError(null);
    
    // --- ZMIANA: Dodajemy nagłówek autoryzacji ---
    fetch(`/api/clusters/${selectedCluster}/workloads`, {
      headers: getAuthHeader() // <-- DODANY NAGŁÓWEK
    })
      .then(response => {
        if (response.status === 401) { // Obsługa wygaśnięcia sesji
          logout();
          navigate('/login');
          throw new Error('Sesja wygasła');
        }
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
  // --- ZMIANA: Dodajemy getAuthHeader i logout do zależności ---
  }, [selectedCluster, getAuthHeader, logout, navigate]); 
  
  useEffect(() => {
    setWorkloads([]);
    setError(null);
    
    if (selectedCluster) {
      fetchData();
    } else if (clusterError) {
      setError(clusterError);
      setLoading(false);
    }
  }, [selectedCluster, clusterError, fetchData]); 
  
  // --- NOWA FUNKCJA: Obsługa wylogowania ---
  const handleLogout = () => {
    logout();
    navigate('/login'); // Przekieruj na logowanie po wylogowaniu
  };

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
            <pre>{error.toString()}</pre> {/* Używamy toString() dla bezpieczeństwa */}
          </Alert>
        </Box>
      );
    }

    // --- ZMIANA: Przekazujemy także userRole ---
    return <Outlet context={{ workloads, fetchData, selectedCluster, userRole }} />;
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
          
          {/* --- NOWY PRZYCISK WYLOGOWANIA --- */}
          <Tooltip title={t('app_bar.logout_btn')}>
            <IconButton color="inherit" onClick={handleLogout}>
              <LogoutIcon />
            </IconButton>
          </Tooltip>
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