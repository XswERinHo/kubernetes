import { useMemo } from 'react';
import { useOutletContext } from 'react-router-dom';
import { Typography, Paper, Grid, Box } from '@mui/material';
import { PieChart } from '@mui/x-charts/PieChart';
import { useCurrencyFormatter } from '../hooks/useCurrencyFormatter'; 
import { useTranslation } from 'react-i18next';
import HealthStatus from '../components/HealthStatus'; // <-- NOWY IMPORT

// Komponent pomocniczy dla karty KPI
function KpiCard({ title, value, color = 'text.primary' }) {
  // ... (bez zmian)
  return (
    <Paper sx={{ p: 2, height: '100%' }}>
      <Typography variant="body2" color="text.secondary">
        {title}
      </Typography>
      <Typography variant="h4" component="p" sx={{ fontWeight: 'bold', color }}>
        {value}
      </Typography>
    </Paper>
  );
}

function extractSavings(recommendationText) {
  // ... (bez zmian)
  const match = recommendationText.match(/Oszczędność: ([\d.]+)\s*zł/i);
  if (match && match[1]) {
    return parseFloat(match[1]);
  }
  return 0;
}

export default function Dashboard() {
  const { workloads } = useOutletContext();
  const { t } = useTranslation();
  const formatCurrency = useCurrencyFormatter(); 

  const dashboardStats = useMemo(() => {
    // ... (bez zmian)
    let totalUsageCost = 0;
    let totalRequestCost = 0;
    let totalSavings = 0;
    let criticalRecs = 0;
    const namespaceCosts = {};

    for (const w of workloads) {
      totalUsageCost += w.usageCost;
      totalRequestCost += w.requestCost;

      if (!namespaceCosts[w.namespace]) {
        namespaceCosts[w.namespace] = 0;
      }
      namespaceCosts[w.namespace] += w.usageCost;

      for (const rec of w.recommendations) {
        if (rec.startsWith('Krytyczne:')) {
          criticalRecs++;
        }
        totalSavings += extractSavings(rec);
      }
    }

    const pieChartData = Object.entries(namespaceCosts)
      .filter(([, cost]) => cost > 0)
      .map(([ns, cost], id) => ({
        id: id,
        value: parseFloat(cost.toFixed(2)),
        label: ns,
      }))
      .sort((a, b) => b.value - a.value);

    return {
      totalUsageCost,
      totalRequestCost,
      totalSavings,
      criticalRecs,
      pieChartData,
      totalNamespaces: pieChartData.length,
      totalWorkloads: workloads.length
    };
  }, [workloads]);

  return (
    <Box>
      <Grid container spacing={3}>
        {/* KPI Cards (bez zmian) */}
        <Grid item xs={12} md={3}>
          <KpiCard
            title={t('dashboard.kpi_usage_cost')}
            value={formatCurrency(dashboardStats.totalUsageCost)}
            color="primary.main"
          />
        </Grid>
        <Grid item xs={12} md={3}>
          <KpiCard
            title={t('dashboard.kpi_savings')}
            value={formatCurrency(dashboardStats.totalSavings)}
            color="success.main"
          />
        </Grid>
        <Grid item xs={12} md={3}>
          <KpiCard
            title={t('dashboard.kpi_request_cost')}
            value={formatCurrency(dashboardStats.totalRequestCost)}
          />
        </Grid>
        <Grid item xs={12} md={3}>
          <KpiCard
            title={t('dashboard.kpi_critical_recs')}
            value={dashboardStats.criticalRecs}
            color={dashboardStats.criticalRecs > 0 ? 'error.main' : 'text.primary'}
          />
        </Grid>
      </Grid>

      <Grid container spacing={3} sx={{ mt: 2 }}>
        {/* Wykres Kołowy (bez zmian) */}
        <Grid item xs={12} md={8}>
          <Paper sx={{ p: 2, height: 400 }}>
            <Typography variant="h6" gutterBottom>
              {t('dashboard.chart_title')}
            </Typography>
            {dashboardStats.pieChartData.length > 0 ? (
              <PieChart
                series={[
                  {
                    data: dashboardStats.pieChartData,
                    highlightScope: { faded: 'global', highlighted: 'item' },
                    faded: { innerRadius: 30, additionalRadius: -30, color: 'gray' },
                    valueFormatter: (item) => formatCurrency(item.value),
                  },
                ]}
                height={300}
                slotProps={{
                  legend: { 
                    direction: 'column', 
                    position: { vertical: 'top', horizontal: 'right' },
                    padding: 0,
                  },
                }}
              />
            ) : (
              <Box sx={{display: 'flex', height: '100%', alignItems: 'center', justifyContent: 'center'}}>
                <Typography>{t('dashboard.no_cost_data')}</Typography>
              </Box>
            )}
          </Paper>
        </Grid>
        
        {/* --- NOWA SEKCJA --- */}
        <Grid item xs={12} md={4}>
           <Grid container spacing={3}>
              <Grid item xs={12}>
                <HealthStatus /> 
              </Grid>
              <Grid item xs={12}>
                <Paper sx={{ p: 2, height: 280, display: 'flex', flexDirection: 'column', justifyContent: 'center' }}>
                  <Typography variant="h6" gutterBottom>{t('dashboard.summary_title')}</Typography>
                  <Box>
                    <Typography variant="h5" component="p" gutterBottom>
                      <strong>{dashboardStats.totalWorkloads}</strong> {t('dashboard.summary_workloads')}
                    </Typography>
                    <Typography variant="h5" component="p">
                      <strong>{dashboardStats.totalNamespaces}</strong> {t('dashboard.summary_namespaces')}
                    </Typography>
                  </Box>
                </Paper>
              </Grid>
           </Grid>
        </Grid>
        {/* --- KONIEC NOWEJ SEKCJI --- */}
      </Grid>
    </Box>
  );
}