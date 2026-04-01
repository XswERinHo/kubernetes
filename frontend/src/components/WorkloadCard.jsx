// frontend/src/components/WorkloadCard.jsx

import {
  Card, CardContent, CardActions, Typography, Box,
  Button, Chip, Tooltip, IconButton
} from '@mui/material';
import { Gauge } from '@mui/x-charts/Gauge';
import InfoIcon from '@mui/icons-material/Info';
import EditIcon from '@mui/icons-material/Edit';
import ShowChartIcon from '@mui/icons-material/ShowChart';
import { useTranslation } from 'react-i18next';

import { formatBytes, parseCpu, parseMemory } from '../utils/formatters'; 

// Funkcje pomocnicze (bez zmian)
const getUsagePercent = (usage, request) => {
  if (!request || request === 0) return 0;
  if (!usage || usage === 0) return 0;
  const percent = (usage / request) * 100;
  return Math.round(percent);
};
const getGaugeColor = (value) => {
  if (value > 85) return 'error';
  if (value > 65) return 'warning';
  return 'success';
};


// --- ZMIANA: Odbieramy `canSeeRecommendations` ---
export default function WorkloadCard({ workload, onOpenChart, onOpenEdit, onOpenDetails, userRole, canSeeRecommendations }) {
  const { t } = useTranslation();
  
  const isAdmin = userRole === 'Admin';
  const canEditResources = userRole === 'Admin' || userRole === 'Editor';
  
  // Logika parsowania (bez zmian)
  const cpuReqParsed = parseCpu(workload.cpuRequests);
  const memReqParsed = parseMemory(workload.memoryRequests);
  const cpuPercent = getUsagePercent(workload.avgCpuUsage, cpuReqParsed);
  const memPercent = getUsagePercent(workload.avgMemoryUsage, memReqParsed);
  const cpuColor = getGaugeColor(cpuPercent);
  const memColor = getGaugeColor(memPercent);

  // Calculate total recommendations
  const totalRecommendations = workload.containers 
    ? workload.containers.reduce((acc, c) => acc + (c.recommendations ? c.recommendations.length : 0), 0)
    : (workload.recommendations ? workload.recommendations.length : 0);

  return (
    <Card sx={{ 
      display: 'flex', 
      flexDirection: 'column', 
      height: '100%',
      transition: 'all 0.3s ease-in-out',
      '&:hover': {
        transform: 'translateY(-5px)',
        boxShadow: (theme) => theme.palette.mode === 'dark' 
          ? '0 12px 40px rgba(0,0,0,0.3)' 
          : '0 12px 40px rgba(0,0,0,0.1)',
        borderColor: (theme) => theme.palette.primary.main,
      }
    }}>
      <CardContent sx={{ flexGrow: 1 }}>
        <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
          <Box>
            <Typography variant="body2" color="text.secondary" gutterBottom>
              {workload.kind}
            </Typography>
            <Typography variant="h6" component="div">
              {workload.name}
            </Typography>
          </Box>
          
          {/* --- ZMIANA: Cały blok Chipa jest teraz warunkowy --- */}
          {canSeeRecommendations ? (
            // Jeśli może widzieć, sprawdzamy czy są rekomendacje
            totalRecommendations > 0 ? (
              <Tooltip title={t('workloads.recs_chip', { count: totalRecommendations })}>
                <Chip
                  icon={<InfoIcon />}
                  label={totalRecommendations}
                  onClick={() => onOpenDetails(workload)} // Tylko Admin/Editor może tu kliknąć
                  color="warning"
                  size="small"
                  clickable
                />
              </Tooltip>
            ) : (
              <Chip label={t('workload_card.recommendations_ok')} size="small" color="success" variant="outlined" />
            )
          ) : (
            // Jeśli nie może widzieć (Viewer), pokazujemy domyślny Chip "OK" bez funkcji klikania
            <Chip label={t('workload_card.recommendations_ok')} size="small" color="success" variant="outlined" />
          )}
          {/* --- KONIEC ZMIANY --- */}
          
        </Box>

        {/* Wskaźniki Gauge (bez zmian) */}
        <Box sx={{ display: 'flex', justifyContent: 'space-around', alignItems: 'center', my: 2 }}>
          <Box sx={{ textAlign: 'center' }}>
            <Gauge
              width={100} height={100} value={cpuPercent} valueMin={0} valueMax={100}
              startAngle={-90} endAngle={90} text={`${cpuPercent}%`}
              sx={(theme) => ({
                [`& .MuiGauge-valueText`]: { fontSize: 20, fill: theme.palette.text.primary },
                [`& .MuiGauge-valueArc`]: { fill: theme.palette[cpuColor].main },
                [`& .MuiGauge-referenceArc`]: { fill: theme.palette.grey[800] },
              })}
            />
            <Typography variant="caption">{t('workload_card.cpu_usage')}</Typography>
            <Typography variant="body2">{workload.avgCpuUsage}m / {workload.cpuRequests || '0'}</Typography>
          </Box>
          <Box sx={{ textAlign: 'center' }}>
            <Gauge
              width={100} height={100} value={memPercent} valueMin={0} valueMax={100}
              startAngle={-90} endAngle={90} text={`${memPercent}%`}
              sx={(theme) => ({
                [`& .MuiGauge-valueText`]: { fontSize: 20, fill: theme.palette.text.primary },
                [`& .MuiGauge-valueArc`]: { fill: theme.palette[memColor].main },
                [`& .MuiGauge-referenceArc`]: { fill: theme.palette.grey[800] },
              })}
            />
            <Typography variant="caption">{t('workload_card.mem_usage')}</Typography>
            <Typography variant="body2">{formatBytes(workload.avgMemoryUsage, 1)} / {workload.memoryRequests || '0'}</Typography>
          </Box>
        </Box>

      </CardContent>

      {/* Card Actions (bez zmian, logika `isAdmin` jest już poprawna) */}
      <CardActions sx={{ justifyContent: 'flex-end' }}>
        {canEditResources && (
          <Tooltip title={t('workload_card.tooltip_edit')}>
            <IconButton size="small" onClick={() => onOpenEdit(workload)}>
              <EditIcon />
            </IconButton>
          </Tooltip>
        )}
        <Tooltip title={t('workload_card.tooltip_chart')}>
          <IconButton size="small" onClick={() => onOpenChart(workload)}>
            <ShowChartIcon />
          </IconButton>
        </Tooltip>
      </CardActions>
    </Card>
  );
}