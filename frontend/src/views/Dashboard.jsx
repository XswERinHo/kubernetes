import { useState, useMemo, useEffect, useCallback } from 'react';
import { useOutletContext } from 'react-router-dom';
import { 
  Typography, Paper, Grid, Box, List, ListItem, ListItemText, 
  Chip, Stack, LinearProgress, Divider, Table, TableBody, 
  TableCell, TableHead, TableRow, Tab, Tabs
} from '@mui/material';
import { useTranslation } from 'react-i18next';
import HealthStatus from '../components/HealthStatus';
import { useAuth } from '../context/AuthContext';
import PercentageBar from '../components/PercentageBar';
import { formatBytes } from '../utils/formatters';

// Komponent pomocniczy dla karty KPI
function KpiCard({ title, value, subValue, color = 'text.primary', loading = false, children }) {
  return (
    <Paper sx={{ p: 2, height: '100%', display: 'flex', flexDirection: 'column', justifyContent: 'space-between' }}>
      <Box>
        <Typography variant="body2" color="text.secondary" gutterBottom>
          {title}
        </Typography>
        {loading ? (
          <LinearProgress sx={{ mt: 1, mb: 1 }} />
        ) : (
          <Box sx={{ display: 'flex', alignItems: 'baseline', gap: 1 }}>
            <Typography variant="h4" component="p" sx={{ 
              fontWeight: 'bold', 
              color: color === 'text.primary' ? 'inherit' : color,
              background: color === 'text.primary' 
                ? 'linear-gradient(45deg, #38bdf8 30%, #818cf8 90%)' 
                : 'inherit',
              WebkitBackgroundClip: color === 'text.primary' ? 'text' : 'none',
              WebkitTextFillColor: color === 'text.primary' ? 'transparent' : 'inherit',
            }}>
              {value}
            </Typography>
            {subValue && (
              <Typography variant="body2" color="text.secondary">
                {subValue}
              </Typography>
            )}
          </Box>
        )}
      </Box>
      {children && <Box sx={{ mt: 2 }}>{children}</Box>}
    </Paper>
  );
}

function TopConsumers({ workloads }) {
  const [tab, setTab] = useState(0);
  const { t } = useTranslation();

  const sortedByCpu = useMemo(() => 
    [...workloads].sort((a, b) => b.avgCpuUsage - a.avgCpuUsage).slice(0, 5), 
  [workloads]);

  const sortedByMem = useMemo(() => 
    [...workloads].sort((a, b) => b.avgMemoryUsage - a.avgMemoryUsage).slice(0, 5), 
  [workloads]);

  const currentList = tab === 0 ? sortedByCpu : sortedByMem;

  return (
    <Paper sx={{ p: 2, height: '100%' }}>
      <Box sx={{ borderBottom: 1, borderColor: 'divider', mb: 2 }}>
        <Tabs value={tab} onChange={(e, v) => setTab(v)}>
          <Tab label={t('dashboard.top_cpu')} />
          <Tab label={t('dashboard.top_memory')} />
        </Tabs>
      </Box>
      <Table size="small">
        <TableHead>
          <TableRow>
            <TableCell>{t('dashboard.workload_name')}</TableCell>
            <TableCell align="right">{t('dashboard.usage')}</TableCell>
            <TableCell align="right">{t('dashboard.request')}</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {currentList.map((w) => (
            <TableRow key={`${w.namespace}/${w.name}`}>
              <TableCell>
                <Typography variant="body2" fontWeight="bold">{w.name}</Typography>
                <Typography variant="caption" color="text.secondary">{w.namespace}</Typography>
              </TableCell>
              <TableCell align="right">
                {tab === 0 ? `${w.avgCpuUsage}m` : formatBytes(w.avgMemoryUsage)}
              </TableCell>
              <TableCell align="right">
                {tab === 0 ? w.cpuRequests : w.memoryRequests}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </Paper>
  );
}

function ClusterEvents({ events, loading }) {
  const { t } = useTranslation();

  if (loading) return <LinearProgress />;
  if (!events || events.length === 0) {
    return (
      <Typography variant="body2" color="text.secondary" sx={{ py: 2, textAlign: 'center' }}>
        {t('dashboard.no_events', 'Brak ostatnich zdarzeń.')}
      </Typography>
    );
  }

  return (
    <List dense sx={{ maxHeight: 300, overflow: 'auto' }}>
      {events.map((event, i) => (
        <ListItem key={i} divider>
          <ListItemText
            primary={
              <Box sx={{ display: 'flex', justifyContent: 'space-between' }}>
                <Typography variant="subtitle2" component="span">
                  {event.reason}
                </Typography>
                <Typography variant="caption" color="text.secondary">
                  {new Date(event.lastTimestamp).toLocaleString()}
                </Typography>
              </Box>
            }
            secondary={
              <>
                <Typography variant="caption" display="block" color="text.primary">
                  {event.involvedObject.kind}/{event.involvedObject.name}
                </Typography>
                {event.message}
              </>
            }
          />
          <Chip 
            label={event.type} 
            size="small" 
            color={event.type === 'Warning' ? 'warning' : 'default'} 
            variant="outlined"
            sx={{ ml: 1 }}
          />
        </ListItem>
      ))}
    </List>
  );
}

export default function Dashboard() {
  const { workloads, selectedCluster } = useOutletContext();
  const { getAuthHeader } = useAuth();
  const { t } = useTranslation();

  const [alertsOverview, setAlertsOverview] = useState(null);
  const [alertsLoading, setAlertsLoading] = useState(true);
  const [nodes, setNodes] = useState([]);
  const [nodesLoading, setNodesLoading] = useState(true);
  const [events, setEvents] = useState([]);
  const [eventsLoading, setEventsLoading] = useState(false);

  const fetchAlertsOverview = useCallback(() => {
    setAlertsLoading(true);
    fetch('/api/alerts', { headers: getAuthHeader() })
      .then((res) => res.ok ? res.json() : {})
      .then((payload) => setAlertsOverview(payload))
      .catch(console.error)
      .finally(() => setAlertsLoading(false));
  }, [getAuthHeader]);

  const fetchNodes = useCallback(() => {
    if (!selectedCluster) return;
    setNodesLoading(true);
    fetch(`/api/clusters/${selectedCluster}/nodes`, { headers: getAuthHeader() })
      .then((res) => res.ok ? res.json() : [])
      .then((data) => setNodes(data || []))
      .catch(console.error)
      .finally(() => setNodesLoading(false));
  }, [selectedCluster, getAuthHeader]);

  const fetchEvents = useCallback(() => {
    if (!selectedCluster) return;
    setEventsLoading(true);
    fetch(`/api/clusters/${selectedCluster}/events`, { headers: getAuthHeader() })
      .then((res) => res.ok ? res.json() : [])
      .then((data) => setEvents(data || []))
      .catch(console.error)
      .finally(() => setEventsLoading(false));
  }, [selectedCluster, getAuthHeader]);

  useEffect(() => {
    fetchAlertsOverview();
    fetchNodes();
    fetchEvents();
  }, [fetchAlertsOverview, fetchNodes, fetchEvents]);

  const clusterStats = useMemo(() => {
    let totalCpuUsage = 0;
    let totalCpuCapacity = 0;
    let totalMemUsage = 0;
    let totalMemCapacity = 0;
    let readyNodes = 0;

    nodes.forEach(node => {
      if (node.status === 'Ready') readyNodes++;
      totalCpuUsage += node.cpuUsage || 0;
      totalCpuCapacity += node.cpuAllocatableMilli || 0;
      totalMemUsage += node.memoryUsage || 0;
      totalMemCapacity += node.memoryAllocatableBytes || 0;
    });

    const cpuPercent = totalCpuCapacity > 0 ? ((totalCpuUsage / totalCpuCapacity) * 100).toFixed(1) : 0;
    const memPercent = totalMemCapacity > 0 ? ((totalMemUsage / totalMemCapacity) * 100).toFixed(1) : 0;

    return {
      readyNodes,
      totalNodes: nodes.length,
      cpuPercent,
      memPercent,
      totalCpuUsage,
      totalMemUsage
    };
  }, [nodes]);

  const activeAlerts = useMemo(() => alertsOverview?.active || [], [alertsOverview]);

  const severityColor = (severity) => {
    switch (severity) {
      case 'critical': return 'error';
      case 'warning': return 'warning';
      case 'info': return 'info';
      default: return 'default';
    }
  };

  return (
    <Box>
      <Typography variant="h4" gutterBottom sx={{ mb: 3 }}>
        {t('nav.dashboard')}
      </Typography>

      <Grid container spacing={3} sx={{ mb: 3 }}>
        {/* Nodes Status */}
        <Grid item xs={12} md={3}>
          <KpiCard 
            title={t('dashboard.nodes_status')} 
            value={`${clusterStats.readyNodes} / ${clusterStats.totalNodes}`}
            subValue={t('dashboard.nodes_ready')}
            loading={nodesLoading}
            color={clusterStats.readyNodes === clusterStats.totalNodes ? 'success.main' : 'warning.main'}
          />
        </Grid>

        {/* CPU Usage */}
        <Grid item xs={12} md={3}>
          <KpiCard 
            title={t('dashboard.cluster_cpu')} 
            value={`${clusterStats.cpuPercent}%`}
            loading={nodesLoading}
          >
            <PercentageBar value={clusterStats.cpuPercent} />
          </KpiCard>
        </Grid>

        {/* Memory Usage */}
        <Grid item xs={12} md={3}>
          <KpiCard 
            title={t('dashboard.cluster_memory')} 
            value={`${clusterStats.memPercent}%`}
            loading={nodesLoading}
          >
            <PercentageBar value={clusterStats.memPercent} />
          </KpiCard>
        </Grid>

        {/* Active Alerts */}
        <Grid item xs={12} md={3}>
          <KpiCard 
            title={t('dashboard.kpi_active_alerts')} 
            value={activeAlerts.length}
            color={activeAlerts.length > 0 ? 'error.main' : 'text.secondary'}
            loading={alertsLoading}
          />
        </Grid>
      </Grid>

      <Grid container spacing={3} sx={{ mb: 3 }}>
        {/* Lewa kolumna: Health Status & Alerts */}
        <Grid item xs={12} lg={6}>
          <Paper sx={{ p: 2, height: '100%', display: 'flex', flexDirection: 'column' }}>
            <HealthStatus />
            
            <Divider sx={{ my: 2 }} />
            
            <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
              <Typography variant="h6">
                {t('dashboard.active_alerts_title')}
              </Typography>
              <Chip label={activeAlerts.length} size="small" color={activeAlerts.length ? 'error' : 'default'} />
            </Box>
            {activeAlerts.length > 0 ? (
              <List dense>
                {activeAlerts.slice(0, 5).map((alert) => (
                  <ListItem key={alert.id} divider disableGutters>
                    <ListItemText
                      primary={alert.ruleName}
                      secondary={alert.message}
                      primaryTypographyProps={{ variant: 'subtitle2' }}
                      secondaryTypographyProps={{ noWrap: true, variant: 'caption' }}
                    />
                    <Chip label={alert.severity} color={severityColor(alert.severity)} size="small" sx={{ ml: 1 }} />
                  </ListItem>
                ))}
              </List>
            ) : (
              <Typography variant="body2" color="text.secondary" sx={{ py: 2, textAlign: 'center' }}>
                {t('dashboard.no_alerts')}
              </Typography>
            )}
          </Paper>
        </Grid>

        {/* Prawa kolumna: Events */}
        <Grid item xs={12} lg={6}>
          <Paper sx={{ p: 2, height: '100%' }}>
            <Typography variant="h6" gutterBottom>
              {t('dashboard.events_title', 'Ostatnie zdarzenia')}
            </Typography>
            <ClusterEvents events={events} loading={eventsLoading} />
          </Paper>
        </Grid>
      </Grid>

      <Grid container spacing={3}>
        <Grid item xs={12}>
          <TopConsumers workloads={workloads} />
        </Grid>
      </Grid>
    </Box>
  );
}