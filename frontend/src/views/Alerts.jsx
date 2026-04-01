import { useState, useEffect, useCallback, useMemo } from 'react';
import {
  Box,
  Grid,
  Paper,
  Typography,
  Chip,
  List,
  ListItem,
  ListItemText,
  IconButton,
  Tooltip,
  TextField,
  Select,
  MenuItem,
  Switch,
  FormControlLabel,
  Button,
  Snackbar,
  Alert as MuiAlert,
  Stack,
  Divider,
  InputAdornment,
} from '@mui/material';
import NotificationsActiveIcon from '@mui/icons-material/NotificationsActive';
import SyncIcon from '@mui/icons-material/Sync';
import SaveIcon from '@mui/icons-material/Save';
import AddIcon from '@mui/icons-material/Add';
import { useTranslation } from 'react-i18next';
import { useAuth } from '../context/AuthContext';

function SeverityChip({ severity, label }) {
  const colorMap = {
    critical: 'error',
    warning: 'warning',
    info: 'info',
  };
  return <Chip label={label} color={colorMap[severity] || 'default'} size="small" />;
}

export default function Alerts() {
  const { getAuthHeader } = useAuth();
  const { t } = useTranslation();

  const [overview, setOverview] = useState(null);
  const [rulesDraft, setRulesDraft] = useState([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [snackbar, setSnackbar] = useState({ open: false, severity: 'success', message: '' });
  const [channelInputs, setChannelInputs] = useState({});

  const fetchAlerts = useCallback(() => {
    setLoading(true);
    fetch('/api/alerts', { headers: getAuthHeader() })
      .then((res) => {
        if (!res.ok) {
          throw new Error(`${res.status}`);
        }
        return res.json();
      })
      .then((payload) => {
        setOverview(payload);
        setRulesDraft(payload.rules || []);
      })
      .catch((err) => {
        console.error(err);
        setSnackbar({ open: true, severity: 'error', message: t('alerts.fetch_error') });
      })
      .finally(() => setLoading(false));
  }, [getAuthHeader, t]);

  useEffect(() => {
    fetchAlerts();
  }, [fetchAlerts]);

  const handleRuleChange = (ruleId, field, value) => {
    setRulesDraft((prev) =>
      prev.map((rule) => (rule.id === ruleId ? { ...rule, [field]: value } : rule)),
    );
  };

  const handleAddChannel = (ruleId, channel) => {
    if (!channel) return;
    setRulesDraft((prev) =>
      prev.map((rule) => {
        if (rule.id === ruleId) {
          const channels = rule.channels || [];
          if (channels.includes(channel)) return rule;
          return { ...rule, channels: [...channels, channel] };
        }
        return rule;
      }),
    );
  };

  const handleRemoveChannel = (ruleId, channel) => {
    setRulesDraft((prev) =>
      prev.map((rule) => {
        if (rule.id === ruleId) {
          return { ...rule, channels: (rule.channels || []).filter((c) => c !== channel) };
        }
        return rule;
      }),
    );
  };

  const handleThresholdChange = (ruleId, value) => {
    const numeric = Number(value);
    handleRuleChange(ruleId, 'threshold', Number.isNaN(numeric) ? 0 : numeric);
  };

  const handleSave = () => {
    setSaving(true);
    fetch('/api/alerts', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...getAuthHeader() },
      body: JSON.stringify({ rules: rulesDraft }),
    })
      .then((res) => {
        if (!res.ok) {
          throw new Error('Save failed');
        }
        return res.text();
      })
      .then(() => {
        setSnackbar({ open: true, severity: 'success', message: t('alerts.save_success') });
        fetchAlerts();
      })
      .catch(() => {
        setSnackbar({ open: true, severity: 'error', message: t('alerts.save_error') });
      })
      .finally(() => setSaving(false));
  };

  const hasChanges = useMemo(() => {
    if (!overview) return false;
    return JSON.stringify(overview.rules || []) !== JSON.stringify(rulesDraft);
  }, [overview, rulesDraft]);

  const statsCards = overview
    ? [
        {
          label: t('alerts.stats_critical'),
          value: overview.stats?.criticalRecommendations || 0,
        },
        {
          label: t('alerts.stats_active'),
          value: overview.active?.length || 0,
        },
      ]
    : [];

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <Box>
          <Typography variant="h4" gutterBottom>
            {t('alerts.title')}
          </Typography>
          <Typography variant="body2" color="text.secondary">
            {t('alerts.subtitle')}
          </Typography>
        </Box>
        <Box>
          <Tooltip title={t('alerts.refresh')}>
            <IconButton onClick={fetchAlerts} disabled={loading}>
              <SyncIcon />
            </IconButton>
          </Tooltip>
        </Box>
      </Box>

      <Grid container spacing={3}>
        {statsCards.map((card) => (
          <Grid item xs={12} md={3} key={card.label}>
            <Paper sx={{ p: 2 }}>
              <Typography variant="body2" color="text.secondary">
                {card.label}
              </Typography>
              <Typography variant="h5" fontWeight="bold">
                {card.value}
              </Typography>
            </Paper>
          </Grid>
        ))}
      </Grid>

      <Grid container spacing={3}>
        <Grid item xs={12} md={6}>
          <Paper sx={{ p: 2, minHeight: 320 }}>
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 2 }}>
              <NotificationsActiveIcon color="warning" />
              <Typography variant="h6">{t('alerts.active_title')}</Typography>
            </Box>
            {overview?.active?.length ? (
              <List>
                {overview.active.map((alert) => (
                  <ListItem key={alert.id} divider>
                    <ListItemText
                      primary={alert.message}
                      secondary={new Date(alert.triggeredAt).toLocaleString()}
                    />
                    <SeverityChip severity={alert.severity} label={t(`alerts.severity_${alert.severity}`)} />
                  </ListItem>
                ))}
              </List>
            ) : (
              <Typography variant="body2">{t('alerts.no_active')}</Typography>
            )}
          </Paper>
        </Grid>
        <Grid item xs={12} md={6}>
          <Paper sx={{ p: 2, minHeight: 320 }}>
            <Typography variant="h6" gutterBottom>
              {t('alerts.history_title')}
            </Typography>
            {overview?.history?.length ? (
              <List>
                {overview.history.map((event) => (
                  <ListItem key={event.id} divider>
                    <ListItemText
                      primary={event.message || `${event.ruleName} ${event.comparison} ${event.threshold}`}
                      secondary={`${new Date(event.triggeredAt).toLocaleString()} · ${event.ruleName}`}
                    />
                    <SeverityChip severity={event.severity} label={t(`alerts.severity_${event.severity}`)} />
                  </ListItem>
                ))}
              </List>
            ) : (
              <Typography variant="body2">{t('alerts.no_history')}</Typography>
            )}
          </Paper>
        </Grid>
      </Grid>

      <Paper sx={{ p: 2 }}>
        <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 2 }}>
          <Typography variant="h6">{t('alerts.rules_title')}</Typography>
          <Tooltip title={t('alerts.save_rules')}>
            <span>
              <Button
                variant="contained"
                startIcon={<SaveIcon />}
                onClick={handleSave}
                disabled={!hasChanges || saving}
              >
                {t('alerts.save_btn')}
              </Button>
            </span>
          </Tooltip>
        </Box>
        {rulesDraft.length ? (
          <Stack spacing={2}>
            {rulesDraft.map((rule) => (
              <Paper key={rule.id} variant="outlined" sx={{ p: 2 }}>
                <Grid container spacing={2}>
                  <Grid item xs={12} md={4}>
                    <Typography variant="subtitle2">{rule.name}</Typography>
                    <Typography variant="body2" color="text.secondary">
                      {rule.metric} • {rule.window}
                    </Typography>
                  </Grid>
                  <Grid item xs={12} md={2}>
                    <TextField
                      type="number"
                      label={t('alerts.threshold')}
                      size="small"
                      fullWidth
                      value={rule.threshold}
                      onChange={(e) => handleThresholdChange(rule.id, e.target.value)}
                    />
                  </Grid>
                  <Grid item xs={12} md={2}>
                    <Select
                      size="small"
                      label={t('alerts.comparison')}
                      fullWidth
                      value={rule.comparison}
                      onChange={(e) => handleRuleChange(rule.id, 'comparison', e.target.value)}
                    >
                      <MenuItem value="gt">&gt;</MenuItem>
                      <MenuItem value="gte">&gt;=</MenuItem>
                      <MenuItem value="lt">&lt;</MenuItem>
                      <MenuItem value="lte">&lt;=</MenuItem>
                    </Select>
                  </Grid>
                  <Grid item xs={12} md={2}>
                    <Select
                      size="small"
                      label={t('alerts.severity')}
                      fullWidth
                      value={rule.severity}
                      onChange={(e) => handleRuleChange(rule.id, 'severity', e.target.value)}
                    >
                      <MenuItem value="critical">{t('alerts.severity_critical')}</MenuItem>
                      <MenuItem value="warning">{t('alerts.severity_warning')}</MenuItem>
                      <MenuItem value="info">{t('alerts.severity_info')}</MenuItem>
                    </Select>
                  </Grid>
                  <Grid item xs={12} md={2}>
                    <FormControlLabel
                      control={
                        <Switch
                          checked={rule.enabled}
                          onChange={(e) => handleRuleChange(rule.id, 'enabled', e.target.checked)}
                        />
                      }
                      label={t('alerts.enabled')}
                    />
                  </Grid>
                  <Grid item xs={12}>
                    <Divider sx={{ mb: 1 }} />
                    <Typography variant="body2" color="text.secondary">
                      {t('alerts.channels')}
                    </Typography>
                    <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 1, mt: 1, mb: 1 }}>
                      {(rule.channels || []).map((channel) => (
                        <Chip
                          key={channel}
                          label={channel}
                          size="small"
                          onDelete={() => handleRemoveChannel(rule.id, channel)}
                        />
                      ))}
                    </Box>
                    <TextField
                      size="small"
                      placeholder="Webhook URL or Email"
                      fullWidth
                      value={channelInputs[rule.id] || ''}
                      onChange={(e) => setChannelInputs({ ...channelInputs, [rule.id]: e.target.value })}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter') {
                          e.preventDefault();
                          const val = channelInputs[rule.id];
                          if (val && val.trim()) {
                            handleAddChannel(rule.id, val.trim());
                            setChannelInputs({ ...channelInputs, [rule.id]: '' });
                          }
                        }
                      }}
                      InputProps={{
                        endAdornment: (
                          <InputAdornment position="end">
                            <IconButton
                              size="small"
                              onClick={() => {
                                const val = channelInputs[rule.id];
                                if (val && val.trim()) {
                                  handleAddChannel(rule.id, val.trim());
                                  setChannelInputs({ ...channelInputs, [rule.id]: '' });
                                }
                              }}
                              edge="end"
                            >
                              <AddIcon />
                            </IconButton>
                          </InputAdornment>
                        ),
                      }}
                    />
                  </Grid>
                </Grid>
              </Paper>
            ))}
          </Stack>
        ) : (
          <Typography variant="body2">{t('alerts.no_rules')}</Typography>
        )}
      </Paper>

      <Snackbar
        open={snackbar.open}
        autoHideDuration={3000}
        onClose={() => setSnackbar((prev) => ({ ...prev, open: false }))}
      >
        <MuiAlert
          elevation={6}
          variant="filled"
          severity={snackbar.severity}
          onClose={() => setSnackbar((prev) => ({ ...prev, open: false }))}
          sx={{ width: '100%' }}
        >
          {snackbar.message}
        </MuiAlert>
      </Snackbar>
    </Box>
  );
}
