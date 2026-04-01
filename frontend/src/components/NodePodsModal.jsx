// frontend/src/components/NodePodsModal.jsx

import { useState, useEffect, useCallback, Fragment } from 'react';
import {
  Modal, Box, Typography, CircularProgress, Alert, Button,
  Table, TableBody, TableCell, TableContainer, TableHead, TableRow, Paper,
  IconButton, Chip, Dialog, DialogTitle, DialogContent, DialogActions
} from '@mui/material';
import { useTranslation } from 'react-i18next';
import { useCluster } from '../context/ClusterContext';
import { useAuth } from '../context/AuthContext';
import ExpandMoreIcon from '@mui/icons-material/ExpandMore';
import ExpandLessIcon from '@mui/icons-material/ExpandLess';
import ArticleIcon from '@mui/icons-material/Article';

const modalStyle = {
  position: 'absolute', top: '50%', left: '50%', transform: 'translate(-50%, -50%)',
  width: '80%', maxWidth: 900, bgcolor: 'background.paper', border: '2px solid #000',
  boxShadow: 24, p: 4, maxHeight: '90vh', display: 'flex', flexDirection: 'column'
};

export default function NodePodsModal({ open, onClose, nodeName }) {
  const { t } = useTranslation();
  const { selectedCluster } = useCluster();
  const { getAuthHeader, logout } = useAuth();

  const [pods, setPods] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const [expandedPods, setExpandedPods] = useState({});

  // Log Viewer State
  const [logModalOpen, setLogModalOpen] = useState(false);
  const [logContent, setLogContent] = useState('');
  const [logLoading, setLogLoading] = useState(false);
  const [currentPod, setCurrentPod] = useState(null);

  const fetchPods = useCallback(() => {
    if (!open || !nodeName || !selectedCluster) return;

    setLoading(true);
    setError(null);

    fetch(`/api/clusters/${selectedCluster}/nodes/${nodeName}/pods`, {
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
      setPods(data || []);
      setExpandedPods({});
    })
    .catch(err => {
      console.error("Fetch pods error:", err);
      setError(err.message);
    })
    .finally(() => {
      setLoading(false);
    });
  }, [open, nodeName, selectedCluster, getAuthHeader, logout]);

  useEffect(() => {
    fetchPods();
  }, [fetchPods]);
  
  // Czyść stan przy zamknięciu
  useEffect(() => {
    if (!open) {
      setPods([]);
      setError(null);
      setLoading(false);
      setExpandedPods({});
    }
  }, [open]);

  const togglePodRow = (podKey) => {
    setExpandedPods(prev => ({
      ...prev,
      [podKey]: !prev[podKey]
    }));
  };

  const handleShowLogs = (pod) => {
    setCurrentPod(pod);
    setLogModalOpen(true);
    setLogLoading(true);
    setLogContent('');

    fetch(`/api/clusters/${selectedCluster}/namespaces/${pod.namespace}/pods/${pod.name}/logs`, {
      headers: getAuthHeader(),
    })
    .then(async res => {
      if (!res.ok) {
        const text = await res.text();
        throw new Error(text || `HTTP error ${res.status}`);
      }
      return res.text();
    })
    .then(text => setLogContent(text))
    .catch(err => setLogContent(`Error fetching logs: ${err.message}`))
    .finally(() => setLogLoading(false));
  };

  const handleCloseLogs = () => {
    setLogModalOpen(false);
    setCurrentPod(null);
  };

  return (
    <>
      <Modal open={open} onClose={onClose}>
        <Box sx={modalStyle}>
          <Typography variant="h6" component="h2" gutterBottom>
            {t('nodes.modal_pods_title', { nodeName: nodeName })}
          </Typography>

          <Box sx={{ flexGrow: 1, overflowY: 'auto', mt: 2 }}>
            {loading && (
              <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: 200 }}>
                <CircularProgress />
              </Box>
            )}
            {error && (
              <Alert severity="error" variant="filled" sx={{ mt: 2 }}>
                {error}
              </Alert>
            )}
            {!loading && !error && (
              <TableContainer component={Paper}>
                <Table stickyHeader size="small">
                  <TableHead>
                    <TableRow>
                      <TableCell />
                      <TableCell>{t('nodes.modal_pods_ns')}</TableCell>
                      <TableCell>{t('nodes.modal_pods_name')}</TableCell>
                      <TableCell>{t('nodes.modal_pods_status')}</TableCell>
                      <TableCell>{t('nodes.modal_pods_reason')}</TableCell>
                      <TableCell align="center">{t('nodes.modal_pods_containers')}</TableCell>
                      <TableCell align="center">{t('nodes.modal_pods_restarts')}</TableCell>
                      <TableCell align="center">Logi</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {pods.map((pod) => {
                      const podKey = `${pod.namespace}-${pod.name}`;
                      const isExpanded = !!expandedPods[podKey];
                      const containers = pod.containers || [];
                      return (
                        <Fragment key={podKey}>
                          <TableRow>
                            <TableCell>
                              {containers.length > 0 && (
                                <IconButton size="small" onClick={() => togglePodRow(podKey)}>
                                  {isExpanded ? <ExpandLessIcon fontSize="small" /> : <ExpandMoreIcon fontSize="small" />}
                                </IconButton>
                              )}
                            </TableCell>
                            <TableCell>{pod.namespace}</TableCell>
                            <TableCell>{pod.name}</TableCell>
                            <TableCell>{pod.status}</TableCell>
                            <TableCell>{pod.reason}</TableCell>
                            <TableCell align="center">{pod.containerCount ?? containers.length}</TableCell>
                            <TableCell align="center">{pod.restartCount ?? 0}</TableCell>
                            <TableCell align="center">
                              <IconButton size="small" onClick={() => handleShowLogs(pod)} title="Pokaż logi">
                                <ArticleIcon fontSize="small" />
                              </IconButton>
                            </TableCell>
                          </TableRow>
                          {isExpanded && containers.length > 0 && (
                            <TableRow>
                              <TableCell colSpan={8}>
                                <Table size="small">
                                  <TableHead>
                                    <TableRow>
                                      <TableCell>{t('nodes.containers_col_name')}</TableCell>
                                      <TableCell>{t('nodes.containers_col_image')}</TableCell>
                                      <TableCell>{t('nodes.containers_col_state')}</TableCell>
                                      <TableCell>{t('nodes.containers_col_ready')}</TableCell>
                                      <TableCell>{t('nodes.containers_col_restarts')}</TableCell>
                                      <TableCell>{t('nodes.containers_col_cpu')}</TableCell>
                                      <TableCell>{t('nodes.containers_col_mem')}</TableCell>
                                      <TableCell>{t('nodes.containers_col_message')}</TableCell>
                                    </TableRow>
                                  </TableHead>
                                  <TableBody>
                                    {containers.map((container) => (
                                      <TableRow key={`${podKey}-${container.name}`}>
                                        <TableCell>{container.name}</TableCell>
                                        <TableCell>{container.image}</TableCell>
                                        <TableCell>
                                          <Typography variant="body2" color="text.secondary">
                                            {container.state}
                                            {container.reason ? ` (${container.reason})` : ''}
                                          </Typography>
                                        </TableCell>
                                        <TableCell>
                                          <Chip
                                            size="small"
                                            color={container.ready ? 'success' : 'warning'}
                                            label={container.ready ? t('nodes.status_ready') : t('nodes.status_not_ready')}
                                          />
                                        </TableCell>
                                        <TableCell>{container.restartCount}</TableCell>
                                        <TableCell>
                                          {container.cpuRequests || '—'} / {container.cpuLimits || '—'}
                                        </TableCell>
                                        <TableCell>
                                          {container.memoryRequests || '—'} / {container.memoryLimits || '—'}
                                        </TableCell>
                                        <TableCell>
                                          {container.message || container.reason || '—'}
                                        </TableCell>
                                      </TableRow>
                                    ))}
                                  </TableBody>
                                </Table>
                              </TableCell>
                            </TableRow>
                          )}
                        </Fragment>
                      );
                    })}
                  </TableBody>
                </Table>
              </TableContainer>
            )}
          </Box>

          <Button onClick={onClose} variant="outlined" sx={{ mt: 2, flexShrink: 0 }}>
            {t('chart_modal.close_btn')}
          </Button>
        </Box>
      </Modal>

      {/* Log Viewer Dialog */}
      <Dialog open={logModalOpen} onClose={handleCloseLogs} maxWidth="lg" fullWidth>
        <DialogTitle>
          Logi: {currentPod?.name} ({currentPod?.namespace})
        </DialogTitle>
        <DialogContent dividers>
          {logLoading ? (
            <Box sx={{ display: 'flex', justifyContent: 'center', p: 3 }}>
              <CircularProgress />
            </Box>
          ) : (
            <Box
              component="pre"
              sx={{
                backgroundColor: '#1e1e1e',
                color: '#d4d4d4',
                p: 2,
                borderRadius: 1,
                overflowX: 'auto',
                fontFamily: 'monospace',
                fontSize: '0.875rem',
                maxHeight: '60vh'
              }}
            >
              {logContent || 'Brak logów lub pusty strumień.'}
            </Box>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={handleCloseLogs}>Zamknij</Button>
        </DialogActions>
      </Dialog>
    </>
  );
}