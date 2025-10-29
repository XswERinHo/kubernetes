import { useState, useEffect } from 'react';
// --- Dodatkowe importy z MUI ---
import {
  AppBar, Toolbar, Typography, Container, Paper, TableContainer, Table,
  TableHead, TableBody, TableRow, TableCell, CircularProgress, Alert as MuiAlert, // Zmieniono nazwę Alert na MuiAlert, aby uniknąć konfliktu
  Modal, Box, Button, Chip, Dialog, DialogActions, DialogContent,
  DialogContentText, DialogTitle, Snackbar // Komponenty dla Dialogu i Powiadomień
} from '@mui/material';
import InfoIcon from '@mui/icons-material/Info';
import ApplyChangesIcon from '@mui/icons-material/Send'; // Ikona dla przycisku Zastosuj
// --- Koniec dodatkowych importów ---
import './App.css';

// Funkcja formatBytes (bez zmian)
function formatBytes(bytes, decimals = 2) { /* ... (kod funkcji bez zmian) ... */ if (!bytes || bytes === 0) return '0 Bytes'; const k = 1024; const dm = decimals < 0 ? 0 : decimals; const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB']; const i = Math.floor(Math.log(bytes) / Math.log(k)); if (i < 0 || i >= sizes.length) return '0 Bytes'; return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i]; }

// Styl dla Modala (bez zmian)
const modalStyle = { /* ... (kod stylu bez zmian) ... */ position: 'absolute', top: '50%', left: '50%', transform: 'translate(-50%, -50%)', width: 600, bgcolor: 'background.paper', border: '2px solid #000', boxShadow: 24, p: 4, color: 'white', backgroundColor: '#424242' };

// --- NOWA FUNKCJA DO PARSOWANIA REKOMENDACJI ---
// Próbuje wyciągnąć typ zasobu i nową wartość z tekstu rekomendacji
// Na razie obsługuje tylko naszą rekomendację zmniejszenia CPU requests
function parseActionableRecommendation(recText) {
  const matchCpu = recText.match(/Info: Niskie zużycie CPU.*?Rozważ zmniejszenie żądań.*?do (\d+m)\./); // Przykładowa modyfikacja tekstu rek. w backendzie, aby zawierała nową wartość
  if (matchCpu && matchCpu[1]) {
    return { type: 'apply', resource: 'cpuRequests', value: matchCpu[1], text: recText };
  }
  // Tutaj można dodać parsowanie innych typów rekomendacji (np. pamięć, limity)
  return { type: 'info', text: recText }; // Domyślnie traktuj jako informacyjną
}
// --- KONIEC FUNKCJI PARSOWANIA ---

function App() {
  const [deployments, setDeployments] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [selectedDeployment, setSelectedDeployment] = useState(null);
  // --- NOWY STAN DLA DIALOGU I POWIADOMIEŃ ---
  const [confirmDialogOpen, setConfirmDialogOpen] = useState(false);
  const [actionToConfirm, setActionToConfirm] = useState(null); // Przechowuje {deployment, change}
  const [snackbarOpen, setSnackbarOpen] = useState(false);
  const [snackbarMessage, setSnackbarMessage] = useState('');
  const [snackbarSeverity, setSnackbarSeverity] = useState('success'); // 'success' lub 'error'
  // --- KONIEC NOWEGO STANU ---

  // Funkcja do pobierania danych (z dodanym resetem stanu przy pobieraniu)
  const fetchData = () => {
    setLoading(true);
    setError(null); // Resetuj błąd przed nowym pobraniem
    fetch('/api/deployments')
      .then(response => { if (!response.ok) throw new Error(`HTTP error! status: ${response.status}`); return response.json(); })
      .then(data => {
        const deploymentsWithRecs = (data || []).map(dep => ({ ...dep, recommendations: dep.recommendations || [] }));
        setDeployments(deploymentsWithRecs);
      })
      .catch(error => { console.error('Error fetching data:', error); setError(`Failed to load deployments: ${error.message}`); })
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    fetchData(); // Wywołaj przy pierwszym ładowaniu
  }, []); // Pusta tablica zależności

  const handleShowDetails = (deployment) => setSelectedDeployment(deployment);
  const handleCloseDetails = () => setSelectedDeployment(null);

  // --- NOWE FUNKCJE OBSŁUGI AKCJI ---
  const handleApplyClick = (deployment, change) => {
    setActionToConfirm({ deployment, change });
    setConfirmDialogOpen(true);
  };

  const handleConfirmDialogClose = (confirmed) => {
    setConfirmDialogOpen(false);
    if (confirmed && actionToConfirm) {
      applyRecommendation(actionToConfirm.deployment, actionToConfirm.change);
    }
    setActionToConfirm(null); // Wyczyść akcję
  };

  const handleSnackbarClose = () => {
    setSnackbarOpen(false);
  };

  // Funkcja wysyłająca żądanie PATCH
  const applyRecommendation = (deployment, change) => {
    const { namespace, name } = deployment;
    const body = { [change.resource]: change.value }; // Tworzymy obiekt np. { "cpuRequests": "150m" }

    fetch(`/api/deployments/${namespace}/${name}/resources`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    })
    .then(response => {
      if (!response.ok) {
        // Spróbuj odczytać tekst błędu z odpowiedzi
        return response.text().then(text => { throw new Error(text || `HTTP error! status: ${response.status}`) });
      }
      return response.text(); // Odczytaj tekst sukcesu
    })
    .then(message => {
      setSnackbarMessage(message || 'Recommendation applied successfully!');
      setSnackbarSeverity('success');
      setSnackbarOpen(true);
      handleCloseDetails(); // Zamknij modal po sukcesie
      fetchData(); // Odśwież dane w tabeli
    })
    .catch(error => {
      console.error('Error applying recommendation:', error);
      setSnackbarMessage(`Failed to apply recommendation: ${error.message}`);
      setSnackbarSeverity('error');
      setSnackbarOpen(true);
    });
  };
  // --- KONIEC NOWYCH FUNKCJI ---


  if (error && !loading) { /* ... (obsługa błędów bez zmian) ... */
     return ( <div className="App"> <h1>K8s Resource Manager</h1> <div className="card error-card"> <h2>Error Loading Data</h2> <MuiAlert severity="error">{error}</MuiAlert> </div> </div> );
  }


  return (
    <>
      <AppBar position="static"> {/* ... (AppBar bez zmian) ... */}
        <Toolbar> <Typography variant="h6" component="div" sx={{ flexGrow: 1 }}> K8s Resource Manager </Typography> </Toolbar>
      </AppBar>

      <Container maxWidth="lg" sx={{ mt: 4, mb: 4 }}>
        {loading && ( <Box sx={{ display: 'flex', justifyContent: 'center', p: 5 }}> <CircularProgress /> </Box> )}

        {/* Tabelę wyświetlamy nawet jeśli jest błąd, ale może być pusta */}
        {!loading && (
           <Paper sx={{ width: '100%', overflow: 'hidden' }}>
             <TableContainer sx={{ maxHeight: 640 }}>
               <Table stickyHeader>
                 <TableHead>
                   <TableRow> {/* ... (Nagłówki tabeli bez zmian) ... */}
                     <TableCell>Namespace</TableCell> <TableCell>Name</TableCell> <TableCell>CPU Req.</TableCell> <TableCell>CPU Lim.</TableCell> <TableCell>CPU Usage</TableCell> <TableCell>Mem Req.</TableCell> <TableCell>Mem Lim.</TableCell> <TableCell>Mem Usage</TableCell> <TableCell align="center">Recs 💡</TableCell>
                   </TableRow>
                 </TableHead>
                 <TableBody>
                   {/* Jeśli jest błąd, pokaż informację */}
                   {error && <TableRow><TableCell colSpan={9} align="center"><MuiAlert severity="warning">Could not load deployments due to an error.</MuiAlert></TableCell></TableRow>}
                   {/* Jeśli nie ma błędu i lista jest pusta */}
                   {!error && deployments.length === 0 && <TableRow><TableCell colSpan={9} align="center">No deployments found in cluster.</TableCell></TableRow>}
                   {/* Jeśli nie ma błędu i są dane */}
                   {!error && deployments.map((deployment) => (
                     <TableRow hover key={`${deployment.namespace}-${deployment.name}`}> {/* ... (Komórki tabeli bez zmian) ... */}
                       <TableCell>{deployment.namespace}</TableCell> <TableCell>{deployment.name}</TableCell> <TableCell>{deployment.cpuRequests || '-'}</TableCell> <TableCell>{deployment.cpuLimits === "0" ? '-' : deployment.cpuLimits}</TableCell> <TableCell>{deployment.currentCpuUsage}m</TableCell> <TableCell>{deployment.memoryRequests || '-'}</TableCell> <TableCell>{deployment.memoryLimits === "0" ? '-' : deployment.memoryLimits}</TableCell> <TableCell>{formatBytes(deployment.currentMemoryUsage)}</TableCell>
                       <TableCell align="center"> {/* ... (Logika Chip bez zmian) ... */}
                         {deployment.recommendations.length > 0 ? ( <Chip icon={<InfoIcon />} label={deployment.recommendations.length} onClick={() => handleShowDetails(deployment)} color="warning" size="small" clickable /> ) : ( <Chip label="0" size="small" /> )}
                       </TableCell>
                     </TableRow>
                   ))}
                 </TableBody>
               </Table>
             </TableContainer>
           </Paper>
        )}
      </Container>

      {/* --- MODAL Z DODANYMI PRZYCISKAMI AKCJI --- */}
      <Modal open={Boolean(selectedDeployment)} onClose={handleCloseDetails}>
        <Box sx={modalStyle}>
          <Typography variant="h6" component="h2"> Recommendations for {selectedDeployment?.namespace}/{selectedDeployment?.name} </Typography>
          <ul style={{ marginTop: '16px', paddingLeft: '20px', listStyle: 'none' }}>
            {selectedDeployment?.recommendations.map((rec, index) => {
              const parsedRec = parseActionableRecommendation(rec); // Parsujemy rekomendację
              return (
                <li key={index} style={{ marginBottom: '15px', display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                  <span>{parsedRec.text}</span>
                  {/* Pokaż przycisk tylko jeśli rekomendacja jest akcją */}
                  {parsedRec.type === 'apply' && (
                    <Button
                      variant="contained"
                      color="primary"
                      size="small"
                      startIcon={<ApplyChangesIcon />}
                      onClick={() => handleApplyClick(selectedDeployment, parsedRec)}
                      sx={{ ml: 2 }} // marginLeft
                    >
                      Apply
                    </Button>
                  )}
                </li>
              );
            })}
          </ul>
          <Button onClick={handleCloseDetails} variant="outlined" sx={{ mt: 2 }}> Close </Button>
        </Box>
      </Modal>
      {/* --- KONIEC MODALA --- */}

      {/* --- DIALOG POTWIERDZENIA --- */}
      <Dialog open={confirmDialogOpen} onClose={() => handleConfirmDialogClose(false)}>
        <DialogTitle>Confirm Action</DialogTitle>
        <DialogContent>
          <DialogContentText>
            Are you sure you want to change **{actionToConfirm?.change.resource}** for deployment **{actionToConfirm?.deployment.namespace}/{actionToConfirm?.deployment.name}** to **{actionToConfirm?.change.value}**?
            <br/><br/>
            This will trigger a rolling update of the pods.
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => handleConfirmDialogClose(false)}>Cancel</Button>
          <Button onClick={() => handleConfirmDialogClose(true)} color="primary" autoFocus>Confirm</Button>
        </DialogActions>
      </Dialog>
      {/* --- KONIEC DIALOGU --- */}

      {/* --- SNACKBAR DO POWIADOMIEŃ --- */}
      <Snackbar open={snackbarOpen} autoHideDuration={6000} onClose={handleSnackbarClose} anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}>
        {/* Używamy Alert wewnątrz Snackbar dla ładniejszego wyglądu */}
        <MuiAlert onClose={handleSnackbarClose} severity={snackbarSeverity} sx={{ width: '100%' }}>
          {snackbarMessage}
        </MuiAlert>
      </Snackbar>
      {/* --- KONIEC SNACKBARA --- */}
    </>
  );
}

export default App;