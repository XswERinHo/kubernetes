import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Box,
  Typography,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Chip,
  Button,
  CircularProgress,
  Alert,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  TextField,
  Snackbar,
  Tooltip,
} from '@mui/material';
import MuiAlert from '@mui/material/Alert';
import CheckIcon from '@mui/icons-material/CheckCircleOutline';
import CloseIcon from '@mui/icons-material/HighlightOff';
import PendingIcon from '@mui/icons-material/Schedule';
import { useTranslation } from 'react-i18next';

import { useCluster } from '../context/ClusterContext';
import { useAuth } from '../context/AuthContext';

const statusIconMap = {
  pending: <PendingIcon fontSize="small" />, 
  approved: <CheckIcon fontSize="small" color="success" />, 
  rejected: <CloseIcon fontSize="small" color="error" />,
};

const statusColorMap = {
  pending: 'warning',
  approved: 'success',
  rejected: 'error',
};

function formatDate(value) {
  if (!value) return '-';
  return new Date(value).toLocaleString();
}

export default function Approvals() {
  const { selectedCluster } = useCluster();
  const { getAuthHeader, userRole } = useAuth();
  const { t } = useTranslation();

  const canDecide = userRole === 'Admin';

  const [records, setRecords] = useState([]);
  const [statusFilter, setStatusFilter] = useState('pending');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [dialogMode, setDialogMode] = useState('approve');
  const [dialogTarget, setDialogTarget] = useState(null);
  const [rejectReason, setRejectReason] = useState('');
  const [snackbar, setSnackbar] = useState({ open: false, message: '', severity: 'success' });

  const statusOptions = useMemo(() => (
    [
      { value: 'pending', label: t('approvals.filter_pending') },
      { value: 'approved', label: t('approvals.filter_approved') },
      { value: 'rejected', label: t('approvals.filter_rejected') },
      { value: 'all', label: t('approvals.filter_all') },
    ]
  ), [t]);

  const fetchApprovals = useCallback(() => {
    if (!selectedCluster) {
      setRecords([]);
      return;
    }

    setLoading(true);
    setError(null);

    const params = new URLSearchParams();
    if (statusFilter !== 'all') {
      params.append('status', statusFilter);
    }

    fetch(`/api/clusters/${selectedCluster}/approvals?${params.toString()}`, {
      headers: getAuthHeader(),
    })
      .then(async (response) => {
        if (!response.ok) {
          const text = await response.text();
          throw new Error(text || `HTTP error ${response.status}`);
        }
        return response.json();
      })
      .then((data) => {
        setRecords(Array.isArray(data) ? data : []);
      })
      .catch((err) => {
        console.error('Approvals fetch error', err);
        setError(err.message);
      })
      .finally(() => setLoading(false));
  }, [selectedCluster, statusFilter, getAuthHeader]);

  useEffect(() => {
    fetchApprovals();
  }, [fetchApprovals]);

  const handleDecision = (mode, change) => {
    setDialogMode(mode);
    setDialogTarget(change);
    setRejectReason('');
    setDialogOpen(true);
  };

  const handleDialogClose = () => {
    setDialogOpen(false);
    setDialogTarget(null);
    setRejectReason('');
  };

  const submitDecision = () => {
    if (!dialogTarget) return;
    const action = dialogMode === 'approve' ? 'approve' : 'reject';
    const url = `/api/approvals/${dialogTarget.id}/${action}`;

    fetch(url, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        ...getAuthHeader(),
      },
      body: dialogMode === 'reject' ? JSON.stringify({ reason: rejectReason }) : undefined,
    })
      .then(async (response) => {
        const text = await response.text();
        if (!response.ok) {
          throw new Error(text || `HTTP error ${response.status}`);
        }
        return text;
      })
      .then((message) => {
        setSnackbar({ open: true, severity: 'success', message: message || t('approvals.success_generic') });
        handleDialogClose();
        fetchApprovals();
      })
      .catch((err) => {
        console.error('Approval decision error', err);
        setSnackbar({ open: true, severity: 'error', message: err.message });
      });
  };

  const renderPayload = (payload = {}) => {
    const entries = Object.entries(payload);
    if (entries.length === 0) {
      return <Typography variant="body2" color="text.secondary">{t('approvals.no_changes')}</Typography>;
    }
    return (
      <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.5, maxWidth: 300 }}>
        {entries.map(([key, value]) => {
          let displayValue = value;
          // Truncate long string values
          if (typeof value === 'string' && value.length > 40) {
            displayValue = value.substring(0, 37) + '...';
          }
          return (
            <Tooltip key={key} title={`${key}: ${value}`}>
              <Chip
                label={`${key}: ${displayValue}`}
                size="small"
              />
            </Tooltip>
          );
        })}
      </Box>
    );
  };

  const statusLabel = (status) => {
    switch (status) {
      case 'approved':
        return t('approvals.status_approved');
      case 'rejected':
        return t('approvals.status_rejected');
      default:
        return t('approvals.status_pending');
    }
  };

  const renderContent = () => {
    if (!selectedCluster) {
      return (
        <Paper sx={{ p: 3 }}>
          <Typography>{t('approvals.no_cluster')}</Typography>
        </Paper>
      );
    }

    if (loading) {
      return (
        <Box sx={{ display: 'flex', justifyContent: 'center', py: 6 }}>
          <CircularProgress />
        </Box>
      );
    }

    if (error) {
      return (
        <Alert severity="error" sx={{ mb: 2 }}>
          {error}
        </Alert>
      );
    }

    if (records.length === 0) {
      return (
        <Paper sx={{ p: 3 }}>
          <Typography>{t('approvals.empty_state')}</Typography>
        </Paper>
      );
    }

    return (
      <TableContainer component={Paper}>
        <Table>
          <TableHead>
            <TableRow>
              <TableCell>{t('approvals.col_workload')}</TableCell>
              <TableCell>{t('approvals.col_requested_by')}</TableCell>
              <TableCell>{t('approvals.col_payload')}</TableCell>
              <TableCell>{t('approvals.col_requested_at')}</TableCell>
              <TableCell>{t('approvals.col_status')}</TableCell>
              <TableCell>{t('approvals.col_decision')}</TableCell>
              {canDecide && <TableCell align="right">{t('approvals.col_actions')}</TableCell>}
            </TableRow>
          </TableHead>
          <TableBody>
            {records.map((change) => (
              <TableRow key={change.id} hover>
                <TableCell>
                  <Typography fontWeight="bold">{change.namespace}/{change.name}</Typography>
                  <Typography variant="caption" color="text.secondary">{change.kind}</Typography>
                </TableCell>
                <TableCell>
                  <Typography>{change.requestedBy}</Typography>
                  <Typography variant="caption" color="text.secondary">{change.role}</Typography>
                </TableCell>
                <TableCell>
                  <Box sx={{ display: 'flex', flexWrap: 'wrap' }}>
                    {renderPayload(change.payload)}
                  </Box>
                </TableCell>
                <TableCell>{formatDate(change.requestedAt)}</TableCell>
                <TableCell>
                  <Chip
                    icon={statusIconMap[change.status] || undefined}
                    color={statusColorMap[change.status] || 'default'}
                    label={statusLabel(change.status)}
                    size="small"
                  />
                </TableCell>
                <TableCell>
                  {change.status === 'pending' ? '—' : (
                    <>
                      <Typography>{change.decisionBy || '-'}</Typography>
                      <Typography variant="caption" color="text.secondary">
                        {formatDate(change.decisionAt)}
                      </Typography>
                      {change.reason && (
                        <Typography variant="caption" color="text.secondary" display="block">
                          {t('approvals.reason_label')}: {change.reason}
                        </Typography>
                      )}
                    </>
                  )}
                </TableCell>
                {canDecide && (
                  <TableCell align="right">
                    {change.status === 'pending' ? (
                      <Box sx={{ display: 'flex', gap: 1 }}>
                        <Button variant="contained" size="small" onClick={() => handleDecision('approve', change)}>
                          {t('approvals.btn_approve')}
                        </Button>
                        <Button variant="outlined" color="error" size="small" onClick={() => handleDecision('reject', change)}>
                          {t('approvals.btn_reject')}
                        </Button>
                      </Box>
                    ) : (
                      <Typography variant="body2" color="text.secondary">{t('approvals.no_actions')}</Typography>
                    )}
                  </TableCell>
                )}
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </TableContainer>
    );
  };

  return (
    <Box>
      <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 3 }}>
        <Box>
          <Typography variant="h4" gutterBottom>{t('approvals.title')}</Typography>
          <Typography variant="body1" color="text.secondary">
            {t('approvals.subtitle', { cluster: selectedCluster || '-' })}
          </Typography>
        </Box>
        <FormControl sx={{ minWidth: 200 }} size="small">
          <InputLabel>{t('approvals.filter_label')}</InputLabel>
          <Select
            value={statusFilter}
            label={t('approvals.filter_label')}
            onChange={(e) => setStatusFilter(e.target.value)}
          >
            {statusOptions.map((option) => (
              <MenuItem key={option.value} value={option.value}>{option.label}</MenuItem>
            ))}
          </Select>
        </FormControl>
      </Box>

      {renderContent()}

      <Dialog open={dialogOpen} onClose={handleDialogClose} maxWidth="sm" fullWidth>
        <DialogTitle>
          {dialogMode === 'approve' ? t('approvals.dialog_approve_title') : t('approvals.dialog_reject_title')}
        </DialogTitle>
        <DialogContent>
          <Typography gutterBottom>
            {dialogTarget ? `${dialogTarget.namespace}/${dialogTarget.name} (${dialogTarget.kind})` : ''}
          </Typography>
          {dialogMode === 'reject' && (
            <TextField
              autoFocus
              fullWidth
              multiline
              minRows={3}
              label={t('approvals.dialog_reason_label')}
              value={rejectReason}
              onChange={(e) => setRejectReason(e.target.value)}
            />
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={handleDialogClose}>{t('approvals.dialog_cancel')}</Button>
          <Button variant="contained" color={dialogMode === 'approve' ? 'primary' : 'error'} onClick={submitDecision}>
            {dialogMode === 'approve' ? t('approvals.btn_confirm_approve') : t('approvals.btn_confirm_reject')}
          </Button>
        </DialogActions>
      </Dialog>

      <Snackbar
        open={snackbar.open}
        autoHideDuration={6000}
        onClose={() => setSnackbar((prev) => ({ ...prev, open: false }))}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
      >
        <MuiAlert elevation={6} variant="filled" severity={snackbar.severity} onClose={() => setSnackbar((prev) => ({ ...prev, open: false }))}>
          {snackbar.message}
        </MuiAlert>
      </Snackbar>
    </Box>
  );
}
