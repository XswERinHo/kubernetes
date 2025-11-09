// frontend/src/components/NodePodsModal.jsx

import { useState, useEffect, useCallback } from 'react';
import {
  Modal, Box, Typography, CircularProgress, Alert, Button,
  Table, TableBody, TableCell, TableContainer, TableHead, TableRow, Paper
} from '@mui/material';
import { useTranslation } from 'react-i18next';
import { useCluster } from '../context/ClusterContext';
import { useAuth } from '../context/AuthContext';

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
    }
  }, [open]);

  return (
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
                    <TableCell>{t('nodes.modal_pods_ns')}</TableCell>
                    <TableCell>{t('nodes.modal_pods_name')}</TableCell>
                    <TableCell>{t('nodes.modal_pods_status')}</TableCell>
                    <TableCell>{t('nodes.modal_pods_reason')}</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {pods.map((pod) => (
                    <TableRow key={`${pod.namespace}-${pod.name}`}>
                      <TableCell>{pod.namespace}</TableCell>
                      <TableCell>{pod.name}</TableCell>
                      <TableCell>{pod.status}</TableCell>
                      <TableCell>{pod.reason}</TableCell>
                    </TableRow>
                  ))}
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
  );
}