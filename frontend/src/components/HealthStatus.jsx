// frontend/src/components/HealthStatus.jsx

import { useState, useEffect, useCallback } from 'react'; // <-- IMPORT useCallback
import { Paper, Typography, Box, Chip, Tooltip, CircularProgress } from '@mui/material';
import CheckCircleIcon from '@mui/icons-material/CheckCircle';
import ErrorIcon from '@mui/icons-material/Error';
import { useTranslation } from 'react-i18next';
import { useCluster } from '../context/ClusterContext'; 

const STATUS_OK = 'ok';
const STATUS_ERROR = 'error';
const REFRESH_INTERVAL = 30000; // 30 sekund

function StatusChip({ status, label, errorMsg }) {
  // ... (bez zmian)
  const { t } = useTranslation();
  const isOk = status === STATUS_OK;
  const color = isOk ? 'success' : 'error';
  const icon = isOk ? <CheckCircleIcon /> : <ErrorIcon />;
  const tooltipTitle = isOk ? t('dashboard.health_ok') : `${t('dashboard.health_error')}: ${errorMsg}`;

  return (
    <Tooltip title={tooltipTitle} placement="top">
      <Chip
        icon={icon}
        label={label}
        color={color}
        variant="outlined"
        sx={{ mr: 1 }}
      />
    </Tooltip>
  );
}

export default function HealthStatus() {
  const { t } = useTranslation();
  const [health, setHealth] = useState(null);
  const [loading, setLoading] = useState(true);
  
  const { selectedCluster } = useCluster();

  // --- ZMIANA: Opakowujemy fetchHealth w useCallback ---
  const fetchHealth = useCallback(() => {
    if (!selectedCluster) return;

    setLoading(true);
    fetch(`/api/clusters/${selectedCluster}/health`)
      .then(res => {
        if (!res.ok) {
          throw new Error(`HTTP error ${res.status}`);
        }
        return res.json();
      })
      .then(data => {
        setHealth(data);
      })
      .catch(err => {
        console.error("Fetch health error:", err);
        setHealth({
          kubernetesStatus: STATUS_ERROR,
          prometheusStatus: STATUS_ERROR,
          errorMessage: err.message,
        });
      })
      .finally(() => {
        setLoading(false);
      });
  }, [selectedCluster]); // <-- Deklarujemy zależność

  // --- ZMIANA: useEffect zależy teraz od stabilnej funkcji fetchHealth ---
  useEffect(() => {
    fetchHealth(); // Pobierz od razu
    const intervalId = setInterval(fetchHealth, REFRESH_INTERVAL); // Ustaw interwał
    return () => clearInterval(intervalId); // Wyczyść interwał przy odmontowaniu
  }, [fetchHealth]); // <-- ZALEŻNOŚĆ

  return (
    <Paper sx={{ p: 2, height: '100%' }}>
      <Typography variant="h6" gutterBottom>
        {t('dashboard.health_title')}
      </Typography>
      <Box sx={{ display: 'flex', alignItems: 'center', minHeight: 40 }}>
        {loading && <CircularProgress size={24} />}
        {!loading && health && (
          <>
            <StatusChip
              status={health.kubernetesStatus}
              label={t('dashboard.health_kubernetes')}
              errorMsg={health.errorMessage}
            />
            <StatusChip
              status={health.prometheusStatus}
              label={t('dashboard.health_prometheus')}
              errorMsg={health.errorMessage}
            />
          </>
        )}
      </Box>
    </Paper>
  );
}