import {
  Card, CardContent, CardActions, Typography, Box,
  Button, Chip, Tooltip, IconButton
} from '@mui/material';
import { Gauge } from '@mui/x-charts/Gauge';
import InfoIcon from '@mui/icons-material/Info';
import EditIcon from '@mui/icons-material/Edit';
import ShowChartIcon from '@mui/icons-material/ShowChart';
import { useTranslation } from 'react-i18next'; // <-- IMPORT i18n

import { formatCurrency, formatBytes, parseCpu, parseMemory } from '../utils/formatters';

// ... (funkcje pomocnicze getUsagePercent i getGaugeColor bez zmian) ...
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


export default function WorkloadCard({ workload, onOpenChart, onOpenEdit, onOpenDetails }) {
  const { t } = useTranslation(); // <-- Używamy hooka
  
  // ... (logika parsowania bez zmian) ...
  const cpuReqParsed = parseCpu(workload.cpuRequests);
  const memReqParsed = parseMemory(workload.memoryRequests);
  const cpuPercent = getUsagePercent(workload.avgCpuUsage, cpuReqParsed);
  const memPercent = getUsagePercent(workload.avgMemoryUsage, memReqParsed);
  const cpuColor = getGaugeColor(cpuPercent);
  const memColor = getGaugeColor(memPercent);

  return (
    <Card sx={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
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
          {workload.recommendations.length > 0 ? (
            <Tooltip title={t('workloads.recs_chip', { count: workload.recommendations.length })}>
              <Chip
                icon={<InfoIcon />}
                label={workload.recommendations.length}
                onClick={() => onOpenDetails(workload)}
                color="warning"
                size="small"
                clickable
              />
            </Tooltip>
          ) : (
            <Chip label={t('workload_card.recommendations_ok')} size="small" color="success" variant="outlined" />
          )}
        </Box>

        <Box sx={{ display: 'flex', justifyContent: 'space-around', alignItems: 'center', my: 2 }}>
          <Box sx={{ textAlign: 'center' }}>
            <Gauge
              // ... (propsy Gauge bez zmian) ...
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
              // ... (propsy Gauge bez zmian) ...
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

        <Box sx={{ display: 'flex', justifyContent: 'space-between', mt: 2 }}>
          <Typography variant="body2">{t('workload_card.cost_usage')}</Typography>
          <Typography variant="body2" sx={{ fontWeight: 'bold' }}>{formatCurrency(workload.usageCost)}</Typography>
        </Box>
        <Box sx={{ display: 'flex', justifyContent: 'space-between' }}>
          <Typography variant="body2">{t('workload_card.cost_request')}</Typography>
          <Typography variant="body2">{formatCurrency(workload.requestCost)}</Typography>
        </Box>
      </CardContent>

      <CardActions sx={{ justifyContent: 'flex-end' }}>
        <Tooltip title={t('workload_card.tooltip_edit')}>
          <IconButton size="small" onClick={() => onOpenEdit(workload)}>
            <EditIcon />
          </IconButton>
        </Tooltip>
        <Tooltip title={t('workload_card.tooltip_chart')}>
          <IconButton size="small" onClick={() => onOpenChart(workload)}>
            <ShowChartIcon />
          </IconButton>
        </Tooltip>
      </CardActions>
    </Card>
  );
}