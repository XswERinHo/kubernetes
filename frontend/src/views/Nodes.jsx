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
      <Chip icon={<InfoIcon />} label={count} size="small" />
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

  const renderContent = () => {
    if (loading) {
      return (
        <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '60vh' }}>
          <CircularProgress />
        </Box>
      );
    }

    if (error) {
      return (
        <Alert severity="error" variant="filled" sx={{ mt: 2 }}>
          {error}
        </Alert>
      );
    }

    return (
      <TableContainer component={Paper}>
        <Table sx={{ minWidth: 650 }}>
          <TableHead>
            <TableRow>
              <TableCell>{t('nodes.col_name')}</TableCell>
              <TableCell>{t('nodes.col_status')}</TableCell>
              <TableCell align="center">{t('nodes.col_pods')}</TableCell>
              <TableCell>{t('nodes.col_cpu')}</TableCell>
              <TableCell>{t('nodes.col_mem')}</TableCell>
              <TableCell align="center">{t('nodes.col_labels')}</TableCell>
              <TableCell align="center">{t('nodes.col_taints')}</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {nodes.map((node) => {
              const cpuPercent = getUsagePercent(node.cpuUsage, node.cpuAllocatableMilli);
              const memPercent = getUsagePercent(node.memoryUsage, node.memoryAllocatableBytes);
              
              return (
                <TableRow key={node.name} sx={{ '&:last-child td, &:last-child th': { border: 0 } }}>
                  <TableCell component="th" scope="row">
                    <Typography variant="body1" fontWeight="bold">{node.name}</Typography>
                  </TableCell>
                  <TableCell>
                    <Chip
                      icon={node.status === 'Ready' ? <CheckCircleIcon /> : <ErrorIcon />}
                      label={node.status === 'Ready' ? t('nodes.status_ready') : t('nodes.status_not_ready')}
                      color={node.status === 'Ready' ? 'success' : 'error'}
                      size="small"
                      variant="outlined"
                    />
                  </TableCell>
                  <TableCell align="center">
                    <Button 
                      size="small" 
                      onClick={() => setSelectedNodeForPods(node.name)}
                    >
                      {node.podCount}
                    </Button>
                  </TableCell>
                  <TableCell>
                    <PercentageBar value={cpuPercent} />
                    <Tooltip title={`${t('nodes.usage_vs_allocatable')}: ${formatMilliCpu(node.cpuUsage)} / ${node.cpuAllocatable}`}>
                      <Typography variant="body2" color="text.secondary">
                        {`${cpuPercent}%`}
                      </Typography>
                    </Tooltip>
                  </TableCell>
                  <TableCell>
                    <PercentageBar value={memPercent} />
                    <Tooltip title={`${t('nodes.usage_vs_allocatable')}: ${formatBytes(node.memoryUsage, 1)} / ${node.memoryAllocatable}`}>
                      <Typography variant="body2" color="text.secondary">
                        {`${memPercent}%`}
                      </Typography>
                    </Tooltip>
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
    );
  };


  return (
    <Box>
      <Typography variant="h4" gutterBottom>
        {t('nodes.title')}
      </Typography>
      <Typography variant="body1" color="text.secondary" gutterBottom>
        {t('nodes.subtitle', { cluster: selectedCluster })}
      </Typography>
      <Box sx={{ mt: 3 }}>
        {renderContent()}
      </Box>

      <NodePodsModal 
        open={Boolean(selectedNodeForPods)}
        nodeName={selectedNodeForPods}
        onClose={() => setSelectedNodeForPods(null)}
      />
    </Box>
  );
}