// frontend/src/views/Nodes.jsx

import { useState, useEffect, useCallback } from 'react';
import {
  Typography, Paper, Box, CircularProgress, Alert,
  Table, TableBody, TableCell, TableContainer, TableHead, TableRow, Tooltip, Chip,
  Button, IconButton
} from '@mui/material';
import CheckCircleIcon from '@mui/icons-material/CheckCircle';
import ErrorIcon from '@mui/icons-material/Error';
import InfoIcon from '@mui/icons-material/Info';
import { useTranslation } from 'react-i18next';
import { useCluster } from '../context/ClusterContext';
import { useAuth } from '../context/AuthContext';
import { formatBytes } from '../utils/formatters'; // <-- Poprawiony import
import PercentageBar from '../components/PercentageBar';
import NodePodsModal from '../components/NodePodsModal';

// Nowa funkcja formatująca
const formatMilliCpu = (milliCpu) => {
  if (milliCpu < 1000) {
    return `${milliCpu}m`;
  }
  return `${(milliCpu / 1000).toFixed(1)} cores`;
};

const getUsagePercent = (usage, total) => {
  if (!total || total === 0) return 0;
  if (!usage || usage === 0) return 0;
  const percent = (usage / total) * 100;
  return Math.round(percent);
};

// --- KOMPONENT POMOCNICZY DLA TOOLTIPÓW (POPRAWIONY) ---
const KeyValueTooltip = ({ data, type }) => {
  const { t } = useTranslation();
  let content;
  let count = 0;

  if (type === 'labels') {
    // --- POPRAWKA: Zabezpieczenie przed 'null' ---
    const safeData = data || {}; // Traktuj null jak pusty obiekt
    count = Object.keys(safeData).length;
    content = count === 0 ? 
      t('nodes.no_labels') : 
      Object.entries(safeData).map(([k, v]) => <div key={k}>{k}: {v}</div>);
  } else { // taints
    // --- POPRAWKA: Zabezpieczenie przed 'null' ---
    const safeData = data || []; // Traktuj null jak pustą tablicę
    count = safeData.length;
    content = count === 0 ? 
      t('nodes.no_taints') :
      safeData.map((taint, i) => <div key={i}>{taint}</div>);
  }
  // --- KONIEC POPRAWKI ---

  return (
    <Tooltip title={<Box sx={{ p: 1, maxWidth: 600 }}>{content}</Box>} placement="top">
      <Chip 
        icon={<InfoIcon sx={{ color: 'inherit !important' }} />} 
        label={count} 
        size="small" 
        sx={{ 
          backgroundColor: 'rgba(255, 255, 255, 0.05)',
          border: '1px solid rgba(255, 255, 255, 0.1)',
          color: 'text.secondary',
          '&:hover': { backgroundColor: 'rgba(255, 255, 255, 0.1)' }
        }}
      />
    </Tooltip>
  );
}
// ------------------------------------------

export default function Nodes() {
  const { t } = useTranslation();
  const { selectedCluster } = useCluster();
  const { getAuthHeader, logout } = useAuth();

  const [nodes, setNodes] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  
  const [selectedNodeForPods, setSelectedNodeForPods] = useState(null);

  const fetchNodes = useCallback(() => {
    if (!selectedCluster) return;

    setLoading(true);
    setError(null);

    fetch(`/api/clusters/${selectedCluster}/nodes`, {
      headers: getAuthHeader(),
    })
    .then(res => {
      if (res.status === 401) {
        logout();
        throw new Error('Sesja wygasła');
      }
      if (!res.ok) {
        throw new Error(`HTTP error ${res.status}`);
      }
      return res.json();
    })
    .then(data => {
      setNodes(data || []);
    })
    .catch(err => {
      console.error("Fetch nodes error:", err);
      setError(err.message);
    })
    .finally(() => {
      setLoading(false);
    });
  }, [selectedCluster, getAuthHeader, logout]);

  useEffect(() => {
    fetchNodes();
  }, [fetchNodes]);

  return (
    <Box>
      <Box sx={{ mb: 4 }}>
        <Typography variant="h4" component="h1" gutterBottom sx={{ 
          background: 'linear-gradient(45deg, #38bdf8 30%, #818cf8 90%)',
          WebkitBackgroundClip: 'text',
          WebkitTextFillColor: 'transparent',
          fontWeight: 'bold'
        }}>
          {t('nodes.title')}
        </Typography>
        <Typography variant="subtitle1" color="text.secondary">
          {t('nodes.subtitle', { cluster: selectedCluster })}
        </Typography>
      </Box>

      {loading && <CircularProgress />}
      {error && <Alert severity="error" sx={{ mb: 2 }}>{error}</Alert>}

      {!loading && !error && (
        <TableContainer component={Paper} sx={{ 
          background: 'transparent', // Przezroczystość dla efektu szkła z App.jsx
          boxShadow: 'none'
        }}>
          <Table>
            <TableHead>
              <TableRow>
                <TableCell>{t('nodes.col_name')}</TableCell>
                <TableCell>{t('nodes.col_status')}</TableCell>
                <TableCell align="center">{t('nodes.col_pods')}</TableCell>
                <TableCell>{t('nodes.col_cpu')}</TableCell>
                <TableCell>{t('nodes.col_mem')}</TableCell>
                <TableCell>{t('nodes.col_disk')}</TableCell>
                <TableCell align="center">{t('nodes.col_labels')}</TableCell>
                <TableCell align="center">{t('nodes.col_taints')}</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {nodes.map((node) => {
                const cpuPercent = getUsagePercent(node.cpuUsage, node.cpuAllocatableMilli);
                const memPercent = getUsagePercent(node.memoryUsage, node.memoryAllocatableBytes);
                const diskPercent = getUsagePercent(node.ephemeralStorageUsageBytes, node.ephemeralStorageAllocatableBytes);

                return (
                  <TableRow 
                    key={node.name}
                    hover
                    sx={{ 
                      '&:hover': { backgroundColor: 'rgba(255, 255, 255, 0.03) !important' },
                      transition: 'background-color 0.2s'
                    }}
                  >
                    <TableCell component="th" scope="row" sx={{ fontWeight: 'bold' }}>
                      {node.name}
                    </TableCell>
                    <TableCell>
                      <Chip 
                        icon={node.status === 'Ready' ? <CheckCircleIcon /> : <ErrorIcon />} 
                        label={node.status} 
                        color={node.status === 'Ready' ? 'success' : 'error'} 
                        variant="outlined"
                        size="small"
                        sx={{ 
                          fontWeight: 600,
                          borderColor: node.status === 'Ready' ? '#4ade80' : '#f87171',
                          color: node.status === 'Ready' ? '#4ade80' : '#f87171',
                          '& .MuiChip-icon': { color: 'inherit' }
                        }}
                      />
                    </TableCell>
                    <TableCell align="center">
                      <Button 
                        variant="text" 
                        size="small" 
                        onClick={() => setSelectedNodeForPods(node.name)}
                        sx={{ minWidth: 'auto', fontWeight: 'bold', color: '#38bdf8' }}
                      >
                        {node.podCount}
                      </Button>
                    </TableCell>
                    <TableCell sx={{ width: '20%' }}>
                      <PercentageBar 
                        value={cpuPercent} 
                        label={`${cpuPercent}%`} 
                        colorStart="#38bdf8" 
                        colorEnd="#818cf8" 
                      />
                    </TableCell>
                    <TableCell sx={{ width: '20%' }}>
                      <PercentageBar 
                        value={memPercent} 
                        label={`${memPercent}%`} 
                        colorStart="#818cf8" 
                        colorEnd="#c084fc" 
                      />
                    </TableCell>
                    <TableCell sx={{ width: '15%' }}>
                      <PercentageBar 
                        value={diskPercent} 
                        label={`${diskPercent}%`} 
                        colorStart="#2dd4bf" 
                        colorEnd="#38bdf8" 
                      />
                    </TableCell>
                    <TableCell align="center">
                      <KeyValueTooltip data={node.labels} type="labels" />
                    </TableCell>
                    <TableCell align="center">
                      <KeyValueTooltip data={node.taints} type="taints" />
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        </TableContainer>
      )}

      <NodePodsModal 
        open={!!selectedNodeForPods} 
        nodeName={selectedNodeForPods} 
        onClose={() => setSelectedNodeForPods(null)} 
      />
    </Box>
  );
}
