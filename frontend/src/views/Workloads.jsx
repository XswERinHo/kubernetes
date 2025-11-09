// frontend/src/views/Workloads.jsx

import { useState, useMemo } from 'react';
import { useOutletContext } from 'react-router-dom';
import {
  Container, Paper, Alert as MuiAlert,
  Modal, Box, Button, Chip, Dialog, DialogActions, DialogContent,
  DialogContentText, DialogTitle, Snackbar, TextField, Tooltip,
  Typography,
  Accordion, AccordionSummary, AccordionDetails,
  Select, MenuItem, FormControl, InputLabel
} from '@mui/material';
import InfoIcon from '@mui/icons-material/Info';
import ApplyChangesIcon from '@mui/icons-material/Send';
import ExpandMoreIcon from '@mui/icons-material/ExpandMore';
import SearchIcon from '@mui/icons-material/Search';
import { useTranslation } from 'react-i18next';

import ChartModal from '../components/ChartModal';
import WorkloadCard from '../components/WorkloadCard';
import { useCurrencyFormatter } from '../hooks/useCurrencyFormatter'; 
import { parseActionableRecommendation } from '../utils/recommendations';
import { useAuth } from '../context/AuthContext';

const modalStyle = { position: 'absolute', top: '50%', left: '50%', transform: 'translate(-50%, -50%)', width: 600, bgcolor: 'background.paper', border: '2px solid #000', boxShadow: 24, p: 4, color: 'white', backgroundColor: '#424242' };

export default function Workloads() {
  const { workloads, fetchData, selectedCluster, userRole } = useOutletContext();
  const { getAuthHeader } = useAuth();
  const { t } = useTranslation();
  const formatCurrency = useCurrencyFormatter(); 
  
  const [selectedWorkload, setSelectedWorkload] = useState(null);
  const [editWorkload, setEditWorkload] = useState(null);
  const [chartWorkload, setChartWorkload] = useState(null);
  const [formData, setFormData] = useState({ cpuRequests: '', cpuLimits: '', memoryRequests: '', memoryLimits: '' });
  const [confirmDialogOpen, setConfirmDialogOpen] = useState(false);
  const [actionToConfirm, setActionToConfirm] = useState(null);
  const [snackbarOpen, setSnackbarOpen] = useState(false);
  const [snackbarMessage, setSnackbarMessage] = useState('');
  const [snackbarSeverity, setSnackbarSeverity] = useState('success');

  const [searchTerm, setSearchTerm] = useState('');
  const [selectedNamespace, setSelectedNamespace] = useState('all');

  const isAdmin = userRole === 'Admin';
  const canApplyRecommendation = userRole === 'Admin' || userRole === 'Editor';
  const canSeeRecommendations = userRole === 'Admin' || userRole === 'Editor';

  // Handlery Modali
  const handleShowDetails = (workload) => setSelectedWorkload(workload);
  const handleCloseDetails = () => setSelectedWorkload(null);
  const handleOpenEditModal = (workload) => {
    if (!isAdmin) return; 
    setFormData({
      cpuRequests: workload.cpuRequests === "0" ? "" : workload.cpuRequests,
      cpuLimits: workload.cpuLimits === "0" ? "" : workload.cpuLimits,
      memoryRequests: workload.memoryRequests === "0" ? "" : workload.memoryRequests,
      memoryLimits: workload.memoryLimits === "0" ? "" : workload.memoryLimits,
    });
    setEditWorkload(workload);
  };
  
  // --- POPRAWKA: TA LINIA ZOSTAŁA PRZYWRÓCONA ---
  const handleCloseEditModal = () => setEditWorkload(null);
  // ------------------------------------------

  const handleOpenChartModal = (workload) => setChartWorkload(workload);
  const handleCloseChartModal = () => setChartWorkload(null);
  const handleFormChange = (event) => { const { name, value } = event.target; setFormData(prev => ({ ...prev, [name]: value })); };
  const handleFormSubmit = () => {
    const change = { resource: 'manualUpdate', value: formData, text: `Czy na pewno chcesz ręcznie zaktualizować zasoby dla ${editWorkload.namespace}/${editWorkload.name}?` };
    setActionToConfirm({ workload: editWorkload, change: change });
    setConfirmDialogOpen(true);
    handleCloseEditModal();
  };
  const handleApplyClick = (workload, change) => {
    const actionText = `Czy na pewno chcesz zmienić **${change.resource}** dla **${workload.namespace}/${workload.name}** na **${change.value}**?`;
    setActionToConfirm({ workload, change: { ...change, text: actionText } });
    setConfirmDialogOpen(true);
  };
  const handleConfirmDialogClose = (confirmed) => {
    setConfirmDialogOpen(false);
    if (confirmed && actionToConfirm) {
      let requestBody = {};
      if (actionToConfirm.change.resource === 'manualUpdate') {
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
    .then(response => {
      if (response.status === 401) {
         throw new Error('Sesja wygasła');
      }
      if (response.status === 403) { 
         throw new Error('Brak uprawnień (Wymagana rola Admin lub Editor)');
      }
      if (!response.ok) {
        return response.text().then(text => {
          throw new Error(text || `HTTP error! status: ${response.status}`)
        });
      }
      return response.text();
    })
    .then(message => {
      setSnackbarMessage(message || 'Resources updated successfully!');
      setSnackbarSeverity('success');
      setSnackbarOpen(true);
      handleCloseDetails();
      handleCloseEditModal();
      fetchData(); 
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
          totalRequestCost: 0,
          totalUsageCost: 0,
          totalRecommendations: 0,
        };
      }
      acc[namespace].workloads.push(workload);
      acc[namespace].totalRequestCost += workload.requestCost;
      acc[namespace].totalUsageCost += workload.usageCost;
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
                  <Typography variant="body2">
                    {t('workloads.usage_cost')} <strong>{formatCurrency(data.totalUsageCost)}</strong>
                  </Typography>
                  <Typography variant="body2" color="text.secondary">
                    {t('workloads.request_cost')} {formatCurrency(data.totalRequestCost)}
                  </Typography>
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
            <ul style={{ marginTop: '16px', paddingLeft: '20px', listStyle: 'none' }}>
              {selectedWorkload?.recommendations.map((rec, index) => {
                const parsedRec = parseActionableRecommendation(rec);
                return (
                  <li key={index} style={{ marginBottom: '15px', display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
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
          )}
          <Button onClick={handleCloseDetails} variant="outlined" sx={{ mt: 2 }}> Close </Button>
        </Box>
      </Modal>

      <Dialog open={Boolean(editWorkload)} onClose={handleCloseEditModal}>
        <DialogTitle>Edit Resources: {editWorkload?.namespace}/{editWorkload?.name}</DialogTitle>
        <DialogContent>
          <DialogContentText sx={{mb: 2}}>
            Enter new resource values. Leave a field blank to remove the setting.
          </DialogContentText>
          <Box component="form" noValidate autoComplete="off">
            <TextField autoFocus margin="dense" name="cpuRequests" label="CPU Requests (e.g., '250m')" type="text" fullWidth variant="outlined" value={formData.cpuRequests} onChange={handleFormChange}/>
            <TextField margin="dense" name="cpuLimits" label="CPU Limits (e.g., '500m')" type="text" fullWidth variant="outlined" value={formData.cpuLimits} onChange={handleFormChange}/>
            <TextField margin="dense" name="memoryRequests" label="Memory Requests (e.g., '128Mi')" type="text" fullWidth variant="outlined" value={formData.memoryRequests} onChange={handleFormChange}/>
            <TextField margin="dense" name="memoryLimits" label="Memory Limits (e.g., '256Mi')" type="text" fullWidth variant="outlined" value={formData.memoryLimits} onChange={handleFormChange}/>
          </Box>
        </DialogContent>
        <DialogActions>
          <Button onClick={handleCloseEditModal}>Cancel</Button>
          <Button onClick={handleFormSubmit} variant="contained">Save Changes</Button>
        </DialogActions>
      </Dialog>
      
      <Dialog open={confirmDialogOpen} onClose={() => handleConfirmDialogClose(false)}>
        <DialogTitle>Confirm Action</DialogTitle>
        <DialogContent>
          <DialogContentText>
            {actionToConfirm?.change.text}
            <br/><br/>
            This will trigger a rolling update of the pods.
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => handleConfirmDialogClose(false)}>Cancel</Button>
          <Button onClick={() => handleConfirmDialogClose(true)} color="primary" autoFocus>Confirm</Button>
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