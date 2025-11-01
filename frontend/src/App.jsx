import { useState, useEffect } from 'react';
import {
  AppBar, Toolbar, Typography, Container, Paper, TableContainer, Table,
  TableHead, TableBody, TableRow, TableCell, CircularProgress, Alert as MuiAlert,
  Modal, Box, Button, Chip, Dialog, DialogActions, DialogContent,
  DialogContentText, DialogTitle, Snackbar, IconButton, TextField, Tooltip
} from '@mui/material';
import InfoIcon from '@mui/icons-material/Info';
import ApplyChangesIcon from '@mui/icons-material/Send';
import EditIcon from '@mui/icons-material/Edit';
import ShowChartIcon from '@mui/icons-material/ShowChart';
import { LineChart, axisClasses, ChartsReferenceLine } from '@mui/x-charts';
import './App.css';

// Funkcja formatBytes (bez zmian)
function formatBytes(bytes, decimals = 2) { if (!bytes || bytes === 0) return '0 Bytes'; const k = 1024; const dm = decimals < 0 ? 0 : decimals; const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB']; const i = Math.floor(Math.log(bytes) / Math.log(k)); if (i < 0 || i >= sizes.length) return '0 Bytes'; return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i]; }

// Style Modali (bez zmian)
const modalStyle = { position: 'absolute', top: '50%', left: '50%', transform: 'translate(-50%, -50%)', width: 600, bgcolor: 'background.paper', border: '2px solid #000', boxShadow: 24, p: 4, color: 'white', backgroundColor: '#424242' };
const chartModalStyle = { position: 'absolute', top: '50%', left: '50%', transform: 'translate(-50%, -50%)', width: '70%', maxWidth: 900, bgcolor: 'background.paper', border: '2px solid #000', boxShadow: 24, p: 4 };

// Funkcja parsowania rekomendacji (bez zmian)
function parseActionableRecommendation(recText) { const matchCpu = recText.match(/Niskie zużycie CPU.*?Rozważ zmniejszenie żądań do (\d+m)\./i); if (matchCpu && matchCpu[1]) { return { type: 'apply', resource: 'cpuRequests', value: matchCpu[1], text: recText }; } const matchMem = recText.match(/Niskie zużycie Pamięci.*?Rozważ zmniejszenie żądań do (\d+(Mi|Gi))\./i); if (matchMem && matchMem[1]) { return { type: 'apply', resource: 'memoryRequests', value: matchMem[1], text: recText }; } return { type: 'info', text: recText }; }

// Funkcje parsowania zasobów (bez zmian)
function parseCpu(valueString) {
  if (!valueString || valueString === "0") return null;
  if (valueString.endsWith('m')) {
    return parseInt(valueString.slice(0, -1), 10);
  }
  const cores = parseFloat(valueString);
  return cores * 1000;
}
function parseMemory(valueString) {
  if (!valueString || valueString === "0") return null;
  const units = { 'Ki': 1024, 'Mi': 1024**2, 'Gi': 1024**3, 'Ti': 1024**4 };
  const match = valueString.match(/^(\d+)(Ki|Mi|Gi|Ti)?$/);
  if (!match) return null;
  const value = parseInt(match[1], 10);
  const unit = match[2];
  if (units[unit]) {
    return value * units[unit];
  }
  return value;
}

// Komponent ChartModal (zmieniono nazwę propsu 'deployment' na 'workload' dla spójności)
function ChartModal({ workload, open, onClose }) {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [data, setData] = useState({ cpuUsage: [], memoryUsage: [] });

  const cpuReq = parseCpu(workload?.cpuRequests);
  const cpuLim = parseCpu(workload?.cpuLimits);
  const memReq = parseMemory(workload?.memoryRequests);
  const memLim = parseMemory(workload?.memoryLimits);

  useEffect(() => {
    if (open && workload) {
      setLoading(true);
      setError(null);
      // ZMIANA: Użycie nowego, elastycznego endpointu
      fetch(`/api/workloads/${workload.namespace}/${workload.kind}/${workload.name}/metrics`)
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
    }
  }, [open, workload]);

  const memValueFormatter = (value) => formatBytes(value, 0);
  const cpuValueFormatter = (value) => `${value.toFixed(0)}m`;
  const timeFormatter = (date) => date.toLocaleTimeString('pl-PL', { hour: '2-digit', minute: '2-digit' });

  const getAxisBounds = (dataPoints, req, lim) => {
    const values = dataPoints.map(d => d.y);
    if (req) values.push(req);
    if (lim) values.push(lim);
    if (values.length === 0) return { yMin: 0, yMax: 100 };
    let min = Math.min(...values);
    let max = Math.max(...values);
    if (max - min < max * 0.1) {
      min = Math.max(0, min - (max * 0.2));
      max = max + (max * 0.2);
    } else {
      const padding = (max - min) * 0.1;
      min = Math.max(0, min - padding);
      max = max + padding;
    }
    return { yMin: min, yMax: max };
  };

  const cpuBounds = getAxisBounds(data.cpuUsage, cpuReq, cpuLim);
  const memBounds = getAxisBounds(data.memoryUsage, memReq, memLim);

  return (
    <Modal open={open} onClose={onClose}>
      <Box sx={chartModalStyle}>
        <Typography variant="h6" component="h2">
          Metrics (Last 1h): {workload?.namespace}/{workload?.name}
        </Typography>
        {loading && ( <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: 400 }}><CircularProgress /></Box> )}
        {error && ( <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: 400 }}><MuiAlert severity="error">{error}</MuiAlert></Box> )}
        {!loading && !error && (
          <Box sx={{ mt: 3 }}>
            <Typography variant="subtitle1">CPU Usage (avg 5m)</Typography>
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
                  {cpuReq && ( <ChartsReferenceLine y={cpuReq} label={`Request (${cpuReq}m)`} labelAlign="start" lineStyle={{ stroke: 'green', strokeDasharray: '4 4' }} labelStyle={{ fill: 'green' }}/> )}
                  {cpuLim && ( <ChartsReferenceLine y={cpuLim} label={`Limit (${cpuLim}m)`} labelAlign="start" lineStyle={{ stroke: 'red', strokeDasharray: '4 4' }} labelStyle={{ fill: 'red' }}/> )}
                </LineChart>
              </Box>
            ) : <Typography>No CPU data available.</Typography>}

            <Typography variant="subtitle1" sx={{ mt: 4 }}>Memory Usage (Working Set)</Typography>
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
                  {memReq && ( <ChartsReferenceLine y={memReq} label={`Request (${formatBytes(memReq, 0)})`} labelAlign="start" lineStyle={{ stroke: 'green', strokeDasharray: '4 4' }} labelStyle={{ fill: 'green' }}/> )}
                  {memLim && ( <ChartsReferenceLine y={memLim} label={`Limit (${formatBytes(memLim, 0)})`} labelAlign="start" lineStyle={{ stroke: 'red', strokeDasharray: '4 4' }} labelStyle={{ fill: 'red' }}/> )}
                </LineChart>
              </Box>
            ) : <Typography>No Memory data available.</Typography>}
          </Box>
        )}
        <Button onClick={onClose} variant="outlined" sx={{ mt: 2 }}> Close </Button>
      </Box>
    </Modal>
  );
}

function App() {
  const [workloads, setWorkloads] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [selectedWorkload, setSelectedWorkload] = useState(null);
  const [editWorkload, setEditWorkload] = useState(null);
  const [chartWorkload, setChartWorkload] = useState(null);

  const [formData, setFormData] = useState({ cpuRequests: '', cpuLimits: '', memoryRequests: '', memoryLimits: '' });
  const [confirmDialogOpen, setConfirmDialogOpen] = useState(false);
  const [actionToConfirm, setActionToConfirm] = useState(null);
  const [snackbarOpen, setSnackbarOpen] = useState(false);
  const [snackbarMessage, setSnackbarMessage] = useState('');
  const [snackbarSeverity, setSnackbarSeverity] = useState('success');

  // ZMIANA: Użycie nowego endpointu /api/workloads
  const fetchData = () => {
    setLoading(true);
    setError(null);
    fetch('/api/workloads')
      .then(response => {
        if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`);
        return response.json();
      })
      .then(data => {
        const workloadsWithRecs = (data || []).map(dep => ({ ...dep, recommendations: dep.recommendations || [] }));
        setWorkloads(workloadsWithRecs);
      })
      .catch(error => {
        console.error('Error fetching data:', error);
        setError(`Failed to load workloads: ${error.message}`);
      })
      .finally(() => setLoading(false));
  };
  useEffect(() => { fetchData(); }, []);

  const handleShowDetails = (workload) => setSelectedWorkload(workload);
  const handleCloseDetails = () => setSelectedWorkload(null);
  const handleOpenEditModal = (workload) => {
    setFormData({
      cpuRequests: workload.cpuRequests === "0" ? "" : workload.cpuRequests,
      cpuLimits: workload.cpuLimits === "0" ? "" : workload.cpuLimits,
      memoryRequests: workload.memoryRequests === "0" ? "" : workload.memoryRequests,
      memoryLimits: workload.memoryLimits === "0" ? "" : workload.memoryLimits,
    });
    setEditWorkload(workload);
  };
  const handleCloseEditModal = () => setEditWorkload(null);
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

  // ZMIANA: Użycie nowego endpointu do aktualizacji zasobów
  const applyResourceUpdate = (workload, body) => {
    const { namespace, name, kind } = workload;
    fetch(`/api/workloads/${namespace}/${kind}/${name}/resources`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    })
    .then(response => {
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

  if (error && !loading) { /* ... (obsługa błędów bez zmian) ... */ }

  return (
    <>
      <AppBar position="static"> <Toolbar> <Typography variant="h6" component="div" sx={{ flexGrow: 1 }}> K8s Resource Manager </Typography> </Toolbar> </AppBar>
      <Container maxWidth="xl" sx={{ mt: 4, mb: 4 }}>
        {loading && ( <Box sx={{ display: 'flex', justifyContent: 'center', p: 5 }}> <CircularProgress /> </Box> )}
        {!loading && (
           <Paper sx={{ width: '100%', overflow: 'hidden' }}>
             <TableContainer sx={{ maxHeight: 840 }}>
               <Table stickyHeader>
                 <TableHead>
                   <TableRow>
                     <TableCell>Namespace</TableCell>
                     <TableCell>Kind</TableCell>
                     <TableCell>Name</TableCell>
                     <TableCell>CPU Req.</TableCell> <TableCell>CPU Lim.</TableCell> <TableCell>CPU Usage</TableCell>
                     <TableCell>Mem Req.</TableCell> <TableCell>Mem Lim.</TableCell> <TableCell>Mem Usage</TableCell>
                     <TableCell align="center">Recs 💡</TableCell>
                     <TableCell align="center">Edit ✏️</TableCell>
                     <TableCell align="center">Chart 📈</TableCell>
                   </TableRow>
                 </TableHead>
                 <TableBody>
                   {!error && workloads.length === 0 && <TableRow><TableCell colSpan={12} align="center">No workloads found in cluster.</TableCell></TableRow>}
                   {!error && workloads.map((workload) => (
                     <TableRow hover key={`${workload.namespace}-${workload.kind}-${workload.name}`}>
                       <TableCell>{workload.namespace}</TableCell>
                       <TableCell>{workload.kind}</TableCell>
                       <TableCell>{workload.name}</TableCell>
                       <TableCell>{workload.cpuRequests || '-'}</TableCell>
                       <TableCell>{workload.cpuLimits === "0" ? '-' : workload.cpuLimits}</TableCell>
                       <TableCell>{workload.avgCpuUsage}m</TableCell>
                       <TableCell>{workload.memoryRequests || '-'}</TableCell>
                       <TableCell>{workload.memoryLimits === "0" ? '-' : workload.memoryLimits}</TableCell>
                       <TableCell>{formatBytes(workload.avgMemoryUsage)}</TableCell>
                       <TableCell align="center">
                         {workload.recommendations.length > 0 ? ( <Chip icon={<InfoIcon />} label={workload.recommendations.length} onClick={() => handleShowDetails(workload)} color="warning" size="small" clickable /> ) : ( <Chip label="0" size="small" /> )}
                       </TableCell>

                       {/* ZMIANA: Usunięto atrybut 'disabled' i zaktualizowano Tooltip */}
                       <TableCell align="center">
                         <Tooltip title="Edit Resources">
                           <IconButton size="small" onClick={() => handleOpenEditModal(workload)}>
                             <EditIcon />
                           </IconButton>
                         </Tooltip>
                       </TableCell>
                       <TableCell align="center">
                         <Tooltip title="Show Chart">
                           <IconButton size="small" onClick={() => handleOpenChartModal(workload)}>
                             <ShowChartIcon />
                           </IconButton>
                         </Tooltip>
                       </TableCell>
                     </TableRow>
                   ))}
                 </TableBody>
               </Table>
             </TableContainer>
           </Paper>
        )}
      </Container>

      {/* Modal Rekomendacji (bez zmian) */}
      <Modal open={Boolean(selectedWorkload)} onClose={handleCloseDetails}>
        <Box sx={modalStyle}>
          <Typography variant="h6" component="h2"> Recommendations for {selectedWorkload?.namespace}/{selectedWorkload?.name} </Typography>
          <ul style={{ marginTop: '16px', paddingLeft: '20px', listStyle: 'none' }}>
            {selectedWorkload?.recommendations.map((rec, index) => {
              const parsedRec = parseActionableRecommendation(rec);
              return (
                <li key={index} style={{ marginBottom: '15px', display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                  <span>{parsedRec.text}</span>
                  {parsedRec.type === 'apply' && ( <Button variant="contained" color="primary" size="small" startIcon={<ApplyChangesIcon />} onClick={() => handleApplyClick(selectedWorkload, parsedRec)} sx={{ ml: 2, flexShrink: 0 }}> Apply </Button> )}
                </li>
              );
            })}
          </ul>
          <Button onClick={handleCloseDetails} variant="outlined" sx={{ mt: 2 }}> Close </Button>
        </Box>
      </Modal>

      {/* Modal Edycji (bez zmian) */}
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

      {/* Dialog Potwierdzenia (bez zmian) */}
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

      {/* Snackbar (bez zmian) */}
      <Snackbar open={snackbarOpen} autoHideDuration={6000} onClose={handleSnackbarClose} anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}>
        <MuiAlert onClose={handleSnackbarClose} severity={snackbarSeverity} sx={{ width: '100%' }}>
          {snackbarMessage}
        </MuiAlert>
      </Snackbar>

      {/* ZMIANA: Wywołanie ChartModal z props 'workload' */}
      <ChartModal
        open={Boolean(chartWorkload)}
        onClose={handleCloseChartModal}
        workload={chartWorkload}
      />
    </>
  );
}

export default App;