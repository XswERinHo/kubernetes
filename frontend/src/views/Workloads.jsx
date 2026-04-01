// frontend/src/views/Workloads.jsx

import { useState, useMemo } from 'react';
import { useOutletContext } from 'react-router-dom';
import {
  Container, Paper, Alert as MuiAlert,
  Modal, Box, Button, Chip, Dialog, DialogActions, DialogContent,
  DialogContentText, DialogTitle, Snackbar, TextField, Tooltip,
  Typography,
  Accordion, AccordionSummary, AccordionDetails,
  Select, MenuItem, FormControl, InputLabel, Grid, Divider,
  Tabs, Tab
} from '@mui/material';
import InfoIcon from '@mui/icons-material/Info';
import ApplyChangesIcon from '@mui/icons-material/Send';
import ExpandMoreIcon from '@mui/icons-material/ExpandMore';
import SearchIcon from '@mui/icons-material/Search';
import MemoryIcon from '@mui/icons-material/Memory';
import SpeedIcon from '@mui/icons-material/Speed';
import { useTranslation } from 'react-i18next';

import ChartModal from '../components/ChartModal';
import WorkloadCard from '../components/WorkloadCard';
import PercentageBar from '../components/PercentageBar';
import { parseActionableRecommendation } from '../utils/recommendations';
import { parseCpu, parseMemory, formatBytes, formatMilliCpu } from '../utils/formatters';
import { useAuth } from '../context/AuthContext';

const modalStyle = { position: 'absolute', top: '50%', left: '50%', transform: 'translate(-50%, -50%)', width: 600, bgcolor: 'background.paper', border: '2px solid #000', boxShadow: 24, p: 4, color: 'white', backgroundColor: '#424242' };

export default function Workloads() {
  const { workloads, fetchData, selectedCluster, userRole } = useOutletContext();
  const { getAuthHeader } = useAuth();
  const { t } = useTranslation();
  
  const [selectedWorkload, setSelectedWorkload] = useState(null);
  const [editWorkload, setEditWorkload] = useState(null);
  const [chartWorkload, setChartWorkload] = useState(null);
  
  // --- ZMIANA: Stan dla edycji kontenerów ---
  const [containerForms, setContainerForms] = useState({}); // Mapa: containerName -> { cpuRequests, ... }
  const [activeContainerTab, setActiveContainerTab] = useState(0); // Indeks aktywnego taba (jeśli użyjemy tabów)
  
  const [confirmDialogOpen, setConfirmDialogOpen] = useState(false);
  const [actionToConfirm, setActionToConfirm] = useState(null);
  const [snackbarOpen, setSnackbarOpen] = useState(false);
  const [snackbarMessage, setSnackbarMessage] = useState('');
  const [snackbarSeverity, setSnackbarSeverity] = useState('success');

  const [searchTerm, setSearchTerm] = useState('');
  const [selectedNamespace, setSelectedNamespace] = useState('all');

  const isAdmin = userRole === 'Admin';
  const canEditResources = userRole === 'Admin' || userRole === 'Editor';
  const canApplyRecommendation = canEditResources;
  const canSeeRecommendations = userRole === 'Admin' || userRole === 'Editor';

  // Handlery Modali
  const handleShowDetails = (workload) => setSelectedWorkload(workload);
  const handleCloseDetails = () => setSelectedWorkload(null);
  
  const handleOpenEditModal = (workload) => {
    if (!canEditResources) return;
    
    // Inicjalizacja formularzy dla każdego kontenera
    const forms = {};
    if (workload.containers && workload.containers.length > 0) {
      workload.containers.forEach(c => {
        forms[c.name] = {
          cpuRequests: c.cpuRequests === "0" ? "" : c.cpuRequests,
          cpuLimits: c.cpuLimits === "0" ? "" : c.cpuLimits,
          memoryRequests: c.memoryRequests === "0" ? "" : c.memoryRequests,
          memoryLimits: c.memoryLimits === "0" ? "" : c.memoryLimits,
        };
      });
    } else {
      // Fallback dla starych danych (jeden "wirtualny" kontener)
      forms['default'] = {
        cpuRequests: workload.cpuRequests === "0" ? "" : workload.cpuRequests,
        cpuLimits: workload.cpuLimits === "0" ? "" : workload.cpuLimits,
        memoryRequests: workload.memoryRequests === "0" ? "" : workload.memoryRequests,
        memoryLimits: workload.memoryLimits === "0" ? "" : workload.memoryLimits,
      };
    }
    
    setContainerForms(forms);
    setActiveContainerTab(0);
    setEditWorkload(workload);
  };
  
  const handleCloseEditModal = () => setEditWorkload(null);

  const handleOpenChartModal = (workload) => setChartWorkload(workload);
  const handleCloseChartModal = () => setChartWorkload(null);
  
  const handleContainerFormChange = (containerName, field, value) => {
    setContainerForms(prev => ({
      ...prev,
      [containerName]: {
        ...prev[containerName],
        [field]: value
      }
    }));
  };
  
  // --- Helper to calculate projected usage ---
  const calculateProjectedUsage = (currentUsage, newValue, type) => {
    if (!currentUsage || !newValue) return 0;
    let limit = 0;
    if (type === 'cpu') {
      limit = parseCpu(newValue);
    } else {
      limit = parseMemory(newValue);
    }
    if (!limit || limit === 0) return 0;
    return Math.min(Math.round((currentUsage / limit) * 100), 100);
  };

  const handleFormSubmit = () => {
    // Przygotuj payload z listą kontenerów
    const containersPayload = Object.entries(containerForms).map(([name, data]) => ({
      name: name === 'default' ? (editWorkload.containers?.[0]?.name || editWorkload.name) : name,
      ...data
    }));

    const change = { 
      resource: 'manualUpdate', 
      value: { containers: containersPayload }, // Nowa struktura
      text: t('workloads.confirm_manual_update', { name: `${editWorkload.namespace}/${editWorkload.name}` })
    };
    setActionToConfirm({ workload: editWorkload, change: change });
    setConfirmDialogOpen(true);
    handleCloseEditModal();
  };
  const handleApplyClick = (workload, change, containerName = null) => {
    const actionText = t('workloads.confirm_recommendation_update', { 
      resource: change.resource, 
      name: `${workload.namespace}/${workload.name}` + (containerName ? `/${containerName}` : ''), 
      value: change.value 
    });
    setActionToConfirm({ workload, change: { ...change, text: actionText }, containerName });
    setConfirmDialogOpen(true);
  };
  const handleConfirmDialogClose = (confirmed) => {
    setConfirmDialogOpen(false);
    if (confirmed && actionToConfirm) {
      let requestBody = {};
      if (actionToConfirm.containerName) {
        requestBody = {
          containers: [{
            name: actionToConfirm.containerName,
            [actionToConfirm.change.resource]: actionToConfirm.change.value
          }]
        };
      } else if (actionToConfirm.change.resource === 'manualUpdate') {
        requestBody = Object.fromEntries(
          Object.entries(actionToConfirm.change.value).filter(([_, v]) => v !== '')
        );
      } else {
        requestBody = { [actionToConfirm.change.resource]: actionToConfirm.change.value };
      }
      applyResourceUpdate(actionToConfirm.workload, requestBody);
    }
    setActionToConfirm(null);
  };
  const handleSnackbarClose = () => { setSnackbarOpen(false); };

  const applyResourceUpdate = (workload, body) => {
    const { namespace, name, kind } = workload;
    
    const url = `/api/clusters/${selectedCluster}/workloads/${namespace}/${kind}/${name}/resources`;
    
    fetch(url, {
      method: 'PATCH',
      headers: { 
        'Content-Type': 'application/json',
        ...getAuthHeader()
      },
      body: JSON.stringify(body),
    })
    .then(async response => {
      if (response.status === 401) {
        throw new Error('Sesja wygasła');
      }
      if (response.status === 403) {
        throw new Error('Brak uprawnień (Wymagana rola Admin lub Editor)');
      }

      const text = await response.text();
      if (!response.ok) {
        throw new Error(text || `HTTP error! status: ${response.status}`);
      }

      return { status: response.status, message: text };
    })
    .then(({ status, message }) => {
      const isPending = status === 202;
      setSnackbarMessage(isPending ? 'Zmiana oczekuje na zatwierdzenie' : 'Zasoby zaktualizowane pomyślnie!');
      setSnackbarSeverity(isPending ? 'info' : 'success');
      setSnackbarOpen(true);

      handleCloseDetails();
      handleCloseEditModal();

      if (!isPending) {
        fetchData(); 
      }
    })
    .catch(error => {
      console.error('Error applying update:', error);
      setSnackbarMessage(`Failed to apply update: ${error.message}`);
      setSnackbarSeverity('error');
      setSnackbarOpen(true);
    });
  };

  const namespaces = useMemo(() => {
    return ['all', ...new Set(workloads.map(w => w.namespace))];
  }, [workloads]);
  const filteredWorkloads = useMemo(() => {
    return workloads.filter(w => {
      const matchesSearch = w.name.toLowerCase().includes(searchTerm.toLowerCase());
      const matchesNs = selectedNamespace === 'all' || w.namespace === selectedNamespace;
      return matchesSearch && matchesNs;
    });
  }, [workloads, searchTerm, selectedNamespace]);
  const groupedWorkloads = useMemo(() => {
    return filteredWorkloads.reduce((acc, workload) => {
      const { namespace } = workload;
      if (!acc[namespace]) {
        acc[namespace] = {
          workloads: [],
          totalRecommendations: 0,
        };
      }
      acc[namespace].workloads.push(workload);
      if (canSeeRecommendations) {
        acc[namespace].totalRecommendations += workload.recommendations.length;
      }
      return acc;
    }, {});
  }, [filteredWorkloads, canSeeRecommendations]);


  return (
    <>
      <Container maxWidth="xl" sx={{ mt: 0, mb: 4 }}>
        
        <Paper sx={{ p: 2, mb: 3, display: 'flex', gap: 2, alignItems: 'center' }}>
          <Box sx={{ display: 'flex', alignItems: 'flex-end', flexGrow: 1 }}>
            <SearchIcon sx={{ color: 'action.active', mr: 1, my: 0.5 }} />
            <TextField
              label={t('workloads.search_label')}
              variant="standard"
              fullWidth
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
            />
          </Box>
          <FormControl sx={{ minWidth: 240 }} variant="standard">
            <InputLabel>{t('workloads.ns_label')}</InputLabel>
            <Select
              value={selectedNamespace}
              onChange={(e) => setSelectedNamespace(e.target.value)}
            >
              {namespaces.map(ns => (
                <MenuItem key={ns} value={ns}>{ns === 'all' ? t('workloads.ns_all') : ns}</MenuItem>
              ))}
            </Select>
          </FormControl>
        </Paper>

        {Object.entries(groupedWorkloads).length === 0 && (
          <Paper sx={{p: 3, textAlign: 'center'}}>
            <Typography>{t('workloads.no_workloads_found')}</Typography>
          </Paper>
        )}
        
        {Object.entries(groupedWorkloads)
          .sort(([nsA], [nsB]) => nsA.localeCompare(nsB))
          .map(([namespace, data]) => (
          <Accordion key={namespace} defaultExpanded>
            <AccordionSummary
              expandIcon={<ExpandMoreIcon />}
              sx={{ 
                backgroundColor: 'background.paper',
                borderBottom: 1,
                borderColor: 'divider'
              }}
            >
              <Box sx={{ display: 'flex', justifyContent: 'space-between', width: '100%', alignItems: 'center', pr: 2 }}>
                <Typography variant="h6">{namespace}</Typography>
                <Box sx={{ display: 'flex', gap: 3, alignItems: 'center' }}>
                  {canSeeRecommendations && (
                    <Chip
                      icon={<InfoIcon />}
                      label={t('workloads.recs_chip', { count: data.totalRecommendations })}
                      color={data.totalRecommendations > 0 ? "warning" : "default"}
                      size="small"
                    />
                  )}
                </Box>
              </Box>
            </AccordionSummary>
            <AccordionDetails sx={{ backgroundColor: 'action.hover', p: 2 }}>
              <Box
                sx={{
                  display: 'grid',
                  gap: 2,
                  gridTemplateColumns: 'repeat(auto-fill, minmax(340px, 1fr))',
                }}
              >
                {data.workloads.map((workload) => (
                  <WorkloadCard
                    key={workload.name}
                    workload={workload}
                    onOpenChart={handleOpenChartModal}
                    onOpenEdit={handleOpenEditModal}
                    onOpenDetails={handleShowDetails}
                    userRole={userRole}
                    canSeeRecommendations={canSeeRecommendations}
                  />
                ))}
              </Box>
            </AccordionDetails>
          </Accordion>
        ))}
      </Container>

      {/* --- MODALE --- */}
      
      <Modal open={Boolean(selectedWorkload)} onClose={handleCloseDetails}>
        <Box sx={modalStyle}>
          <Typography variant="h6" component="h2"> Recommendations for {selectedWorkload?.namespace}/{selectedWorkload?.name} </Typography>
          {canSeeRecommendations && (
            <Box sx={{ mt: 2, maxHeight: '60vh', overflowY: 'auto' }}>
              {/* Container Recommendations */}
              {selectedWorkload?.containers?.map((container) => (
                container.recommendations && container.recommendations.length > 0 && (
                  <Box key={container.name} sx={{ mb: 2 }}>
                    <Typography variant="subtitle1" sx={{ fontWeight: 'bold', mb: 1, color: 'primary.main' }}>
                      Container: {container.name}
                    </Typography>
                    <ul style={{ paddingLeft: '20px', listStyle: 'none', margin: 0 }}>
                      {container.recommendations.map((rec, index) => {
                        const parsedRec = parseActionableRecommendation(rec);
                        return (
                          <li key={index} style={{ marginBottom: '10px', display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                            <span>{parsedRec.text}</span>
                            {canApplyRecommendation && parsedRec.type === 'apply' && ( 
                              <Button 
                                variant="contained" 
                                color="primary" 
                                size="small" 
                                startIcon={<ApplyChangesIcon />} 
                                onClick={() => handleApplyClick(selectedWorkload, parsedRec, container.name)} 
                                sx={{ ml: 2, flexShrink: 0 }}
                              > 
                                Apply 
                              </Button> 
                            )}
                          </li>
                        );
                      })}
                    </ul>
                  </Box>
                )
              ))}
              
              {/* Pod Level Recommendations (Fallback/Legacy) */}
              {selectedWorkload?.recommendations && selectedWorkload.recommendations.length > 0 && (
                 <Box sx={{ mb: 2 }}>
                    <Typography variant="subtitle1" sx={{ fontWeight: 'bold', mb: 1 }}>
                      Pod Level:
                    </Typography>
                    <ul style={{ paddingLeft: '20px', listStyle: 'none', margin: 0 }}>
                      {selectedWorkload.recommendations.map((rec, index) => {
                        const parsedRec = parseActionableRecommendation(rec);
                        return (
                          <li key={index} style={{ marginBottom: '10px', display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                            <span>{parsedRec.text}</span>
                            {canApplyRecommendation && parsedRec.type === 'apply' && ( 
                              <Button 
                                variant="contained" 
                                color="primary" 
                                size="small" 
                                startIcon={<ApplyChangesIcon />} 
                                onClick={() => handleApplyClick(selectedWorkload, parsedRec)} 
                                sx={{ ml: 2, flexShrink: 0 }}
                              > 
                                Apply 
                              </Button> 
                            )}
                          </li>
                        );
                      })}
                    </ul>
                 </Box>
              )}
            </Box>
          )}
          <Button onClick={handleCloseDetails} variant="outlined" sx={{ mt: 2 }}> Close </Button>
        </Box>
      </Modal>

      <Dialog open={Boolean(editWorkload)} onClose={handleCloseEditModal} maxWidth="md" fullWidth>
        <DialogTitle>{t('workloads.edit_modal_title', { name: `${editWorkload?.namespace}/${editWorkload?.name}` })}</DialogTitle>
        <DialogContent>
          <DialogContentText sx={{mb: 3}}>
            {t('workloads.edit_modal_desc')}
          </DialogContentText>
          
          <Tabs 
            value={activeContainerTab} 
            onChange={(e, newValue) => setActiveContainerTab(newValue)}
            variant="scrollable"
            scrollButtons="auto"
            sx={{ borderBottom: 1, borderColor: 'divider', mb: 3 }}
          >
            {editWorkload?.containers?.map((container, index) => (
              <Tab key={container.name} label={container.name} />
            ))}
          </Tabs>

          {editWorkload?.containers?.map((container, index) => (
            <Box 
              key={container.name} 
              role="tabpanel" 
              hidden={activeContainerTab !== index}
              component="form" 
              noValidate 
              autoComplete="off"
            >
              {activeContainerTab === index && (
                <Grid container spacing={4}>
                  {/* CPU Section */}
                  <Grid item xs={12} md={6}>
                    <Box sx={{ display: 'flex', alignItems: 'center', mb: 2, color: 'primary.main' }}>
                      <SpeedIcon sx={{ mr: 1 }} />
                      <Typography variant="h6">{t('workloads.cpu_section')}</Typography>
                    </Box>
                    
                    {/* CPU Visualization */}
                    <Box sx={{ mb: 3, p: 2, bgcolor: 'background.default', borderRadius: 1 }}>
                       <Typography variant="caption" color="text.secondary">{t('workloads.current_usage')}: {formatMilliCpu(container.avgCpuUsage)}</Typography>
                       <Box sx={{ mt: 1 }}>
                         <Typography variant="caption">{t('workloads.projected_request')}: {calculateProjectedUsage(container.avgCpuUsage, containerForms[container.name]?.cpuRequests, 'cpu')}%</Typography>
                         <PercentageBar value={calculateProjectedUsage(container.avgCpuUsage, containerForms[container.name]?.cpuRequests, 'cpu')} />
                       </Box>
                       <Box sx={{ mt: 1 }}>
                         <Typography variant="caption">{t('workloads.projected_limit')}: {calculateProjectedUsage(container.avgCpuUsage, containerForms[container.name]?.cpuLimits, 'cpu')}%</Typography>
                         <PercentageBar value={calculateProjectedUsage(container.avgCpuUsage, containerForms[container.name]?.cpuLimits, 'cpu')} />
                       </Box>
                    </Box>

                    <TextField 
                      autoFocus 
                      margin="dense" 
                      name="cpuRequests" 
                      label={t('workloads.requests')} 
                      placeholder="e.g. 250m"
                      type="text" 
                      fullWidth 
                      variant="outlined" 
                      value={containerForms[container.name]?.cpuRequests || ''} 
                      onChange={(e) => handleContainerFormChange(container.name, e.target.name, e.target.value)}
                      helperText={t('workloads.cpu_req_helper')}
                    />
                    <TextField 
                      margin="dense" 
                      name="cpuLimits" 
                      label={t('workloads.limits')} 
                      placeholder="e.g. 500m"
                      type="text" 
                      fullWidth 
                      variant="outlined" 
                      value={containerForms[container.name]?.cpuLimits || ''} 
                      onChange={(e) => handleContainerFormChange(container.name, e.target.name, e.target.value)}
                      helperText={t('workloads.cpu_lim_helper')}
                    />
                  </Grid>

                  {/* Memory Section */}
                  <Grid item xs={12} md={6}>
                    <Box sx={{ display: 'flex', alignItems: 'center', mb: 2, color: 'primary.main' }}>
                      <MemoryIcon sx={{ mr: 1 }} />
                      <Typography variant="h6">{t('workloads.mem_section')}</Typography>
                    </Box>

                    {/* Memory Visualization */}
                    <Box sx={{ mb: 3, p: 2, bgcolor: 'background.default', borderRadius: 1 }}>
                       <Typography variant="caption" color="text.secondary">{t('workloads.current_usage')}: {formatBytes(container.avgMemoryUsage)}</Typography>
                       <Box sx={{ mt: 1 }}>
                         <Typography variant="caption">{t('workloads.projected_request')}: {calculateProjectedUsage(container.avgMemoryUsage, containerForms[container.name]?.memoryRequests, 'memory')}%</Typography>
                         <PercentageBar value={calculateProjectedUsage(container.avgMemoryUsage, containerForms[container.name]?.memoryRequests, 'memory')} />
                       </Box>
                       <Box sx={{ mt: 1 }}>
                         <Typography variant="caption">{t('workloads.projected_limit')}: {calculateProjectedUsage(container.avgMemoryUsage, containerForms[container.name]?.memoryLimits, 'memory')}%</Typography>
                         <PercentageBar value={calculateProjectedUsage(container.avgMemoryUsage, containerForms[container.name]?.memoryLimits, 'memory')} />
                       </Box>
                    </Box>

                    <TextField 
                      margin="dense" 
                      name="memoryRequests" 
                      label={t('workloads.requests')} 
                      placeholder="e.g. 128Mi"
                      type="text" 
                      fullWidth 
                      variant="outlined" 
                      value={containerForms[container.name]?.memoryRequests || ''} 
                      onChange={(e) => handleContainerFormChange(container.name, e.target.name, e.target.value)}
                      helperText={t('workloads.mem_req_helper')}
                    />
                    <TextField 
                      margin="dense" 
                      name="memoryLimits" 
                      label={t('workloads.limits')} 
                      placeholder="e.g. 256Mi"
                      type="text" 
                      fullWidth 
                      variant="outlined" 
                      value={containerForms[container.name]?.memoryLimits || ''} 
                      onChange={(e) => handleContainerFormChange(container.name, e.target.name, e.target.value)}
                      helperText={t('workloads.mem_lim_helper')}
                    />
                  </Grid>
                </Grid>
              )}
            </Box>
          ))}
        </DialogContent>
        <DialogActions>
          <Button onClick={handleCloseEditModal}>{t('workloads.cancel')}</Button>
          <Button onClick={handleFormSubmit} variant="contained">{t('workloads.save')}</Button>
        </DialogActions>
      </Dialog>
      
      <Dialog open={confirmDialogOpen} onClose={() => handleConfirmDialogClose(false)}>
        <DialogTitle>{t('workloads.confirm_title')}</DialogTitle>
        <DialogContent>
          <DialogContentText>
            {actionToConfirm?.change.text}
            <br/><br/>
            {t('workloads.confirm_desc')}
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => handleConfirmDialogClose(false)}>{t('workloads.cancel')}</Button>
          <Button onClick={() => handleConfirmDialogClose(true)} color="primary" autoFocus>{t('workloads.confirm_btn')}</Button>
        </DialogActions>
      </Dialog>

      <Snackbar open={snackbarOpen} autoHideDuration={6000} onClose={handleSnackbarClose} anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}>
        <MuiAlert onClose={handleSnackbarClose} severity={snackbarSeverity} sx={{ width: '100%' }}>
          {snackbarMessage}
        </MuiAlert>
      </Snackbar>

      <ChartModal
        open={Boolean(chartWorkload)}
        onClose={handleCloseChartModal}
        workload={chartWorkload}
      />
    </>
  );
}