// frontend/src/components/ChartModal.jsx

import { useState, useEffect } from 'react';
import {
  Modal, Box, Button, Typography, ToggleButtonGroup, ToggleButton,
  CircularProgress, Alert as MuiAlert
} from '@mui/material';
// --- POPRAWKA: Brakujące importy ---
import { LineChart, axisClasses, ChartsReferenceLine } from '@mui/x-charts';
import { formatBytes, parseCpu, parseMemory } from '../utils/formatters';
// --- KONIEC POPRAWKI ---
import { useTranslation } from 'react-i18next';
import { useCluster } from '../context/ClusterContext'; 
import { useAuth } from '../context/AuthContext'; 

const chartModalStyle = {
  position: 'absolute', top: '50%', left: '50%', transform: 'translate(-50%, -50%)',
  width: '80%', maxWidth: 900, bgcolor: 'background.paper', border: '2px solid #000',
  boxShadow: 24, p: 4, maxHeight: '90vh', display: 'flex', flexDirection: 'column'
};
const spinnerHeight = 400;

const getAxisBounds = (dataPoints) => {
  const values = dataPoints.map(d => d.y);
  if (values.length === 0) {
    return { yMin: 0, yMax: 100 };
  }
  let min = Math.min(...values);
  let max = Math.max(...values);
  if (max === min) {
    max = max * 1.2 + 10;
    min = min * 0.8 - 10;
  } else {
    const padding = (max - min) * 0.15;
    max = max + padding;
    min = min - padding;
  }
  if (min > 0) {
    min = 0;
  }
  if (max > 0 && max - min < max * 0.1) {
    max = max * 1.1;
  }
  if (max === 0 && min === 0) {
    max = 10;
  }
  return { yMin: min, yMax: max };
};


export default function ChartModal({ workload, open, onClose }) {
  const { t, i18n } = useTranslation(); 
  const { selectedCluster } = useCluster(); 
  const { getAuthHeader } = useAuth(); 

  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [data, setData] = useState({ cpuUsage: [], memoryUsage: [] });
  const [timeRange, setTimeRange] = useState('1h');

  const cpuReq = parseCpu(workload?.cpuRequests);
  const cpuLim = parseCpu(workload?.cpuLimits);
  const memReq = parseMemory(workload?.memoryRequests);
  const memLim = parseMemory(workload?.memoryLimits);

  const handleTimeRangeChange = (event, newRange) => {
    if (newRange) {
      setTimeRange(newRange);
    }
  };

  useEffect(() => {
    if (!open || !workload || !selectedCluster) {
      return;
    }

    setLoading(true);
    setError(null);
    
    const url = `/api/clusters/${selectedCluster}/workloads/${workload.namespace}/${workload.kind}/${workload.name}/metrics?range=${timeRange}`;

    fetch(url, {
      headers: getAuthHeader()
    }) 
      .then(response => {
        if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);
        return response.json();
      })
      .then(historyData => {
        const cpuUsage = (historyData.cpuUsage || []).map(d => ({ x: new Date(d.timestamp), y: d.value }));
        const memoryUsage = (historyData.memoryUsage || []).map(d => ({ x: new Date(d.timestamp), y: d.value }));
        setData({ cpuUsage, memoryUsage });
      })
      .catch(err => {
        console.error('Error fetching metrics:', err);
        setError(`Failed to load metrics: ${err.message}`);
      })
      .finally(() => setLoading(false));
    
  }, [open, workload, timeRange, selectedCluster, getAuthHeader]); 

  useEffect(() => {
    if (!open) {
      setLoading(true);
      setTimeRange('1h');
      setData({ cpuUsage: [], memoryUsage: [] });
      setError(null);
    }
  }, [open]);

  const memValueFormatter = (value) => formatBytes(value, 0);
  const cpuValueFormatter = (value) => `${value.toFixed(0)}m`;
  const timeFormatter = (date) => {
    const lang = i18n.language.startsWith('pl') ? 'pl-PL' : 'en-US';
    if (timeRange === '7d') {
      return date.toLocaleDateString(lang, { month: '2-digit', day: '2-digit' });
    }
    return date.toLocaleTimeString(lang, { hour: '2-digit', minute: '2-digit' });
  };

  const cpuBounds = getAxisBounds(data.cpuUsage, cpuReq, cpuLim);
  const memBounds = getAxisBounds(data.memoryUsage, memReq, memLim);

  return (
    <Modal open={open} onClose={onClose}>
      <Box sx={chartModalStyle}>
        
        <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 2, flexShrink: 0 }}>
          <Typography variant="h6" component="h2">
            {t('chart_modal.title', { namespace: workload?.namespace, name: workload?.name })}
          </Typography>
          <ToggleButtonGroup
            color="primary"
            value={timeRange}
            exclusive
            onChange={handleTimeRangeChange}
            size="small"
          >
            <ToggleButton value="1h">1H</ToggleButton>
            <ToggleButton value="6h">6H</ToggleButton>
            <ToggleButton value="24h">24H</ToggleButton>
            <ToggleButton value="7d">7D</ToggleButton>
          </ToggleButtonGroup>
        </Box>
        
        <Box sx={{ flexGrow: 1, overflowY: 'auto', mt: 3, pr: 2 }}>
          {loading && (
            <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: spinnerHeight }}>
              <CircularProgress />
            </Box>
          )}
          {error && (
            <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: spinnerHeight }}>
              <MuiAlert severity="error">{error}</MuiAlert>
            </Box>
          )}
          {!loading && !error && (
            <Box> 
              <Typography variant="subtitle1">
                {t('chart_modal.cpu_title', { timeRange: timeRange })}
              </Typography>
              {data.cpuUsage.length > 0 ? (
                <Box sx={{ height: 300, width: '100%' }}>
                  <LineChart
                    dataset={data.cpuUsage}
                    series={[{ dataKey: 'y', label: 'CPU (mCores)', valueFormatter: cpuValueFormatter, showMark: false }]}
                    xAxis={[{ dataKey: 'x', scaleType: 'time', valueFormatter: timeFormatter }]}
                    yAxis={[{ valueFormatter: cpuValueFormatter, min: cpuBounds.yMin, max: cpuBounds.yMax }]}
                    sx={{ [`.${axisClasses.left} .${axisClasses.label}`]: { transform: 'translate(-10px, 0)' } }}
                    margin={{ left: 60 }}
                  >
                    {cpuReq && ( <ChartsReferenceLine y={cpuReq} label={t('chart_modal.cpu_req_label', { value: cpuReq })} labelAlign="start" lineStyle={{ stroke: 'green', strokeDasharray: '4 4' }} labelStyle={{ fill: 'green' }}/> )}
                    {cpuLim && ( <ChartsReferenceLine y={cpuLim} label={t('chart_modal.cpu_lim_label', { value: cpuLim })} labelAlign="start" lineStyle={{ stroke: 'red', strokeDasharray: '4 4' }} labelStyle={{ fill: 'red' }}/> )}
                  </LineChart>
                </Box>
              ) : <Typography>{t('chart_modal.no_cpu_data')}</Typography>}

              <Typography variant="subtitle1" sx={{ mt: 4 }}>
                {t('chart_modal.mem_title', { timeRange: timeRange })}
              </Typography>
              {data.memoryUsage.length > 0 ? (
                <Box sx={{ height: 300, width: '100%' }}>
                  <LineChart
                    dataset={data.memoryUsage}
                    series={[{ dataKey: 'y', label: 'Memory', valueFormatter: memValueFormatter, showMark: false }]}
                    xAxis={[{ dataKey: 'x', scaleType: 'time', valueFormatter: timeFormatter }]}
                    yAxis={[{ valueFormatter: memValueFormatter, min: memBounds.yMin, max: memBounds.yMax }]}
                    sx={{ [`.${axisClasses.left} .${axisClasses.label}`]: { transform: 'translate(-10px, 0)' } }}
                    margin={{ left: 70 }}
                  >
                    {memReq && ( <ChartsReferenceLine y={memReq} label={t('chart_modal.mem_req_label', { value: formatBytes(memReq, 0) })} labelAlign="start" lineStyle={{ stroke: 'green', strokeDasharray: '4 4' }} labelStyle={{ fill: 'green' }}/> )}
                    {memLim && ( <ChartsReferenceLine y={memLim} label={t('chart_modal.mem_lim_label', { value: formatBytes(memLim, 0) })} labelAlign="start" lineStyle={{ stroke: 'red', strokeDasharray: '4 4' }} labelStyle={{ fill: 'red' }}/> )}
                  </LineChart>
                </Box>
              ) : <Typography>{t('chart_modal.no_mem_data')}</Typography>}
            </Box>
          )}
        </Box>

        <Button onClick={onClose} variant="outlined" sx={{ mt: 2, flexShrink: 0 }}>
          {t('chart_modal.close_btn')}
        </Button>
      </Box>
    </Modal>
  );
}