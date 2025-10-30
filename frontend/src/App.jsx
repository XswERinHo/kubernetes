import { useState, useEffect } from 'react';
// --- Dodatkowe importy z MUI ---
import {
  AppBar, Toolbar, Typography, Container, Paper, TableContainer, Table,
  TableHead, TableBody, TableRow, TableCell, CircularProgress, Alert as MuiAlert,
  Modal, Box, Button, Chip, Dialog, DialogActions, DialogContent,
  DialogContentText, DialogTitle, Snackbar, IconButton, TextField // Dodano IconButton i TextField
} from '@mui/material';
import InfoIcon from '@mui/icons-material/Info';
import ApplyChangesIcon from '@mui/icons-material/Send';
import EditIcon from '@mui/icons-material/Edit'; // --- NOWA IKONA EDYCJI ---
// --- Koniec dodatkowych importów ---
import './App.css';

// Funkcja formatBytes (bez zmian)
function formatBytes(bytes, decimals = 2) { /* ... (kod funkcji bez zmian) ... */ if (!bytes || bytes === 0) return '0 Bytes'; const k = 1024; const dm = decimals < 0 ? 0 : decimals; const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB']; const i = Math.floor(Math.log(bytes) / Math.log(k)); if (i < 0 || i >= sizes.length) return '0 Bytes'; return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i]; }

// Styl dla Modala Rekomendacji (bez zmian)
const modalStyle = { /* ... (kod stylu bez zmian) ... */ position: 'absolute', top: '50%', left: '50%', transform: 'translate(-50%, -50%)', width: 600, bgcolor: 'background.paper', border: '2px solid #000', boxShadow: 24, p: 4, color: 'white', backgroundColor: '#424242' };

// Funkcja parsowania rekomendacji (bez zmian)
function parseActionableRecommendation(recText) { /* ... (kod funkcji bez zmian) ... */ const matchCpu = recText.match(/Niskie zużycie CPU.*?Rozważ zmniejszenie żądań do (\d+m)\./i); if (matchCpu && matchCpu[1]) { return { type: 'apply', resource: 'cpuRequests', value: matchCpu[1], text: recText }; } const matchMem = recText.match(/Niskie zużycie Pamięci.*?Rozważ zmniejszenie żądań do (\d+(Mi|Gi))\./i); if (matchMem && matchMem[1]) { return { type: 'apply', resource: 'memoryRequests', value: matchMem[1], text: recText }; } return { type: 'info', text: recText }; }

function App() {
  const [deployments, setDeployments] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [selectedDeployment, setSelectedDeployment] = useState(null); // Dla modala rekomendacji
  // --- NOWY STAN DLA MODALA EDYCJI ---
  const [editDeployment, setEditDeployment] = useState(null); // Przechowuje obiekt wdrożenia do edycji
  const [formData, setFormData] = useState({ cpuRequests: '', cpuLimits: '', memoryRequests: '', memoryLimits: '' });
  // --- Koniec nowego stanu ---
  const [confirmDialogOpen, setConfirmDialogOpen] = useState(false);
  const [actionToConfirm, setActionToConfirm] = useState(null);
  const [snackbarOpen, setSnackbarOpen] = useState(false);
  const [snackbarMessage, setSnackbarMessage] = useState('');
  const [snackbarSeverity, setSnackbarSeverity] = useState('success');

  const fetchData = () => { /* ... (kod funkcji bez zmian) ... */ setLoading(true); setError(null); fetch('/api/deployments').then(response => { if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`); return response.json(); }).then(data => { const deploymentsWithRecs = (data || []).map(dep => ({ ...dep, recommendations: dep.recommendations || [] })); setDeployments(deploymentsWithRecs); }).catch(error => { console.error('Error fetching data:', error); setError(`Failed to load deployments: ${error.message}`); }).finally(() => setLoading(false)); };
  useEffect(() => { fetchData(); }, []);

  const handleShowDetails = (deployment) => setSelectedDeployment(deployment);
  const handleCloseDetails = () => setSelectedDeployment(null);

  // --- NOWE FUNKCJE DLA MODALA EDYCJI ---
  const handleOpenEditModal = (deployment) => {
    // Ustaw formularz z obecnymi wartościami (zamień "0" na "" dla lepszego wyglądu w formularzu)
    setFormData({
      cpuRequests: deployment.cpuRequests === "0" ? "" : deployment.cpuRequests,
      cpuLimits: deployment.cpuLimits === "0" ? "" : deployment.cpuLimits,
      memoryRequests: deployment.memoryRequests === "0" ? "" : deployment.memoryRequests,
      memoryLimits: deployment.memoryLimits === "0" ? "" : deployment.memoryLimits,
    });
    setEditDeployment(deployment); // To otworzy modal edycji
  };

  const handleCloseEditModal = () => {
    setEditDeployment(null);
  };

  const handleFormChange = (event) => {
    const { name, value } = event.target;
    setFormData(prev => ({ ...prev, [name]: value }));
  };

  // Wywoływane przy zapisie formularza edycji
  const handleFormSubmit = () => {
    // Przygotuj obiekt akcji dla okna potwierdzenia
    const change = { 
      resource: 'manualUpdate', // Specjalny typ akcji
      value: formData, // Przekaż cały obiekt formularza
      text: `Czy na pewno chcesz ręcznie zaktualizować zasoby dla ${editDeployment.namespace}/${editDeployment.name}?`
    };
    setActionToConfirm({ deployment: editDeployment, change: change });
    setConfirmDialogOpen(true);
    handleCloseEditModal(); // Zamknij modal edycji
  };
  // --- KONIEC NOWYCH FUNKCJI ---

  // Wywoływane przy kliknięciu "Apply" na rekomendacji
  const handleApplyClick = (deployment, change) => {
    const actionText = `Czy na pewno chcesz zmienić **${change.resource}** dla **${deployment.namespace}/${deployment.name}** na **${change.value}**?`;
    setActionToConfirm({ 
      deployment, 
      change: { ...change, text: actionText } // Dołącz tekst do obiektu zmiany
    });
    setConfirmDialogOpen(true);
  };

  const handleConfirmDialogClose = (confirmed) => {
    setConfirmDialogOpen(false);
    if (confirmed && actionToConfirm) {
      let requestBody = {};
      // Sprawdź, czy to ręczna aktualizacja czy zastosowanie rekomendacji
      if (actionToConfirm.change.resource === 'manualUpdate') {
        // Weź dane z formularza, odfiltruj puste pola
        requestBody = Object.fromEntries(
          Object.entries(actionToConfirm.change.value).filter(([_, v]) => v !== '')
        );
      } else {
        // Zbuduj ciało z pojedynczej rekomendacji
        requestBody = { [actionToConfirm.change.resource]: actionToConfirm.change.value };
      }
      // Wyślij żądanie do API
      applyResourceUpdate(actionToConfirm.deployment, requestBody);
    }
    setActionToConfirm(null); // Wyczyść akcję
  };

  const handleSnackbarClose = () => { setSnackbarOpen(false); };

  // ZMIENIONA NAZWA: applyRecommendation -> applyResourceUpdate
  // Ta funkcja przyjmuje teraz gotowe "body" żądania
  const applyResourceUpdate = (deployment, body) => {
    const { namespace, name } = deployment;

    fetch(`/api/deployments/${namespace}/${name}/resources`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    })
    .then(response => { /* ... (logika obsługi odpowiedzi bez zmian) ... */ if (!response.ok) { return response.text().then(text => { throw new Error(text || `HTTP error! status: ${response.status}`) }); } return response.text(); })
    .then(message => {
      setSnackbarMessage(message || 'Resources updated successfully!');
      setSnackbarSeverity('success');
      setSnackbarOpen(true);
      handleCloseDetails(); // Zamknij modal rekomendacji (jeśli był otwarty)
      handleCloseEditModal(); // Zamknij modal edycji (jeśli był otwarty)
      fetchData(); // Odśwież dane w tabeli
    })
    .catch(error => { /* ... (logika obsługi błędu bez zmian) ... */ console.error('Error applying update:', error); setSnackbarMessage(`Failed to apply update: ${error.message}`); setSnackbarSeverity('error'); setSnackbarOpen(true); });
  };

  if (error && !loading) { /* ... (obsługa błędów bez zmian) ... */ }

  return (
    <>
      <AppBar position="static"> <Toolbar> <Typography variant="h6" component="div" sx={{ flexGrow: 1 }}> K8s Resource Manager </Typography> </Toolbar> </AppBar>
      <Container maxWidth="lg" sx={{ mt: 4, mb: 4 }}>
        {loading && ( <Box sx={{ display: 'flex', justifyContent: 'center', p: 5 }}> <CircularProgress /> </Box> )}
        {!loading && (
           <Paper sx={{ width: '100%', overflow: 'hidden' }}>
             <TableContainer sx={{ maxHeight: 640 }}>
               <Table stickyHeader>
                 <TableHead>
                   <TableRow>
                     <TableCell>Namespace</TableCell> <TableCell>Name</TableCell> <TableCell>CPU Req.</TableCell> <TableCell>CPU Lim.</TableCell> <TableCell>CPU Usage</TableCell> <TableCell>Mem Req.</TableCell> <TableCell>Mem Lim.</TableCell> <TableCell>Mem Usage</TableCell> <TableCell align="center">Recs 💡</TableCell>
                     {/* --- NOWA KOLUMNA EDYCJI --- */}
                     <TableCell align="center">Edit ✏️</TableCell> 
                   </TableRow>
                 </TableHead>
                 <TableBody>
                   {/* ... (Renderowanie błędów i pustej listy bez zmian) ... */}
                   {!error && deployments.length === 0 && <TableRow><TableCell colSpan={10} align="center">No deployments found in cluster.</TableCell></TableRow>}
                   {!error && deployments.map((deployment) => (
                     <TableRow hover key={`${deployment.namespace}-${deployment.name}`}>
                       <TableCell>{deployment.namespace}</TableCell> <TableCell>{deployment.name}</TableCell> <TableCell>{deployment.cpuRequests || '-'}</TableCell> <TableCell>{deployment.cpuLimits === "0" ? '-' : deployment.cpuLimits}</TableCell> <TableCell>{deployment.currentCpuUsage}m</TableCell> <TableCell>{deployment.memoryRequests || '-'}</TableCell> <TableCell>{deployment.memoryLimits === "0" ? '-' : deployment.memoryLimits}</TableCell> <TableCell>{formatBytes(deployment.currentMemoryUsage)}</TableCell>
                       <TableCell align="center">
                         {deployment.recommendations.length > 0 ? ( <Chip icon={<InfoIcon />} label={deployment.recommendations.length} onClick={() => handleShowDetails(deployment)} color="warning" size="small" clickable /> ) : ( <Chip label="0" size="small" /> )}
                       </TableCell>
                       {/* --- NOWA KOMÓRKA Z PRZYCISKIEM EDYCJI --- */}
                       <TableCell align="center">
                         <IconButton size="small" onClick={() => handleOpenEditModal(deployment)}>
                           <EditIcon />
                         </IconButton>
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
      <Modal open={Boolean(selectedDeployment)} onClose={handleCloseDetails}>
        <Box sx={modalStyle}>
          {/* ... (Zawartość modala rekomendacji bez zmian, nadal używa handleApplyClick) ... */}
          <Typography variant="h6" component="h2"> Recommendations for {selectedDeployment?.namespace}/{selectedDeployment?.name} </Typography>
          <ul style={{ marginTop: '16px', paddingLeft: '20px', listStyle: 'none' }}>
            {selectedDeployment?.recommendations.map((rec, index) => {
              const parsedRec = parseActionableRecommendation(rec);
              return (
                <li key={index} style={{ marginBottom: '15px', display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                  <span>{parsedRec.text}</span>
                  {parsedRec.type === 'apply' && ( <Button variant="contained" color="primary" size="small" startIcon={<ApplyChangesIcon />} onClick={() => handleApplyClick(selectedDeployment, parsedRec)} sx={{ ml: 2, flexShrink: 0 }}> Apply </Button> )}
                </li>
              );
            })}
          </ul>
          <Button onClick={handleCloseDetails} variant="outlined" sx={{ mt: 2 }}> Close </Button>
        </Box>
      </Modal>

      {/* --- NOWY MODAL EDYCJI --- */}
      <Dialog open={Boolean(editDeployment)} onClose={handleCloseEditModal}>
        <DialogTitle>Edit Resources: {editDeployment?.namespace}/{editDeployment?.name}</DialogTitle>
        <DialogContent>
          <DialogContentText sx={{mb: 2}}>
            Enter new resource values. Leave a field blank to remove the setting.
          </DialogContentText>
          <Box component="form" noValidate autoComplete="off">
            <TextField
              autoFocus
              margin="dense"
              name="cpuRequests"
              label="CPU Requests (e.g., '250m')"
              type="text"
              fullWidth
              variant="outlined"
              value={formData.cpuRequests}
              onChange={handleFormChange}
            />
            <TextField
              margin="dense"
              name="cpuLimits"
              label="CPU Limits (e.g., '500m')"
              type="text"
              fullWidth
              variant="outlined"
              value={formData.cpuLimits}
              onChange={handleFormChange}
            />
            <TextField
              margin="dense"
              name="memoryRequests"
              label="Memory Requests (e.g., '128Mi')"
              type="text"
              fullWidth
              variant="outlined"
              value={formData.memoryRequests}
              onChange={handleFormChange}
            />
            <TextField
              margin="dense"
              name="memoryLimits"
              label="Memory Limits (e.g., '256Mi')"
              type="text"
              fullWidth
              variant="outlined"
              value={formData.memoryLimits}
              onChange={handleFormChange}
            />
          </Box>
        </DialogContent>
        <DialogActions>
          <Button onClick={handleCloseEditModal}>Cancel</Button>
          <Button onClick={handleFormSubmit} variant="contained">Save Changes</Button>
        </DialogActions>
      </Dialog>
      {/* --- KONIEC MODALA EDYCJI --- */}


      {/* Dialog Potwierdzenia (zmodyfikowany, aby obsługiwać oba przypadki) */}
      <Dialog open={confirmDialogOpen} onClose={() => handleConfirmDialogClose(false)}>
        <DialogTitle>Confirm Action</DialogTitle>
        <DialogContent>
          <DialogContentText>
            {/* Wyświetl niestandardowy tekst z obiektu akcji */}
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
    </>
  );
}

export default App;