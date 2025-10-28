import { useState, useEffect } from 'react';
// --- NOWE IMPORTY Z MUI ---
import {
  AppBar, Toolbar, Typography, Container, Paper, TableContainer, Table,
  TableHead, TableBody, TableRow, TableCell, CircularProgress, Alert,
  Modal, Box, Button, Chip // Chip to ładna etykieta
} from '@mui/material';
import InfoIcon from '@mui/icons-material/Info'; // Ikona żarówki
// --- KONIEC IMPORTÓW Z MUI ---
import './App.css'; // Nadal możemy używać globalnych stylów

// Funkcja formatująca bajty (bez zmian)
function formatBytes(bytes, decimals = 2) {
  if (!bytes || bytes === 0) return '0 Bytes'; // Dodano sprawdzenie !bytes
  const k = 1024;
  const dm = decimals < 0 ? 0 : decimals;
  const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  // Naprawiono potencjalny błąd z NaN, gdy i jest poza zakresem
  if (i < 0 || i >= sizes.length) return '0 Bytes';
  return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i];
}

// --- NOWY STYL DLA MODALA Z MUI ---
// Box to komponent MUI do tworzenia layoutu, stylizujemy go bezpośrednio
const modalStyle = {
  position: 'absolute',
  top: '50%',
  left: '50%',
  transform: 'translate(-50%, -50%)',
  width: 400,
  bgcolor: 'background.paper', // Używa kolorów z motywu MUI
  border: '2px solid #000',
  boxShadow: 24, // Standardowy cień MUI
  p: 4, // Padding w jednostkach MUI (4 * 8px = 32px)
  color: 'white', // Ustawienie koloru tekstu na biały dla ciemnego motywu
  backgroundColor: '#424242', // Ciemniejsze tło dla modala
};
// --- KONIEC STYLU MODALA ---

function App() {
  const [deployments, setDeployments] = useState([]);
  const [loading, setLoading] = useState(true); // Nowy stan do pokazywania ładowania
  const [error, setError] = useState(null);
  const [selectedDeployment, setSelectedDeployment] = useState(null);

  useEffect(() => {
    setLoading(true); // Rozpocznij ładowanie
    fetch('/api/deployments')
      .then((response) => {
        if (!response.ok) { throw new Error(`HTTP error! status: ${response.status}`); }
        return response.json();
      })
      .then((data) => {
        const deploymentsWithRecs = (data || []).map(dep => ({
          ...dep,
          recommendations: dep.recommendations || []
        }));
        setDeployments(deploymentsWithRecs);
        setError(null); // Wyczyść błąd, jeśli się powiodło
      })
      .catch((error) => {
        console.error('Error fetching data:', error);
        setError(`Failed to load deployments: ${error.message}`);
      })
      .finally(() => {
        setLoading(false); // Zakończ ładowanie (zarówno przy sukcesie, jak i błędzie)
      });
  }, []);

  const handleShowDetails = (deployment) => setSelectedDeployment(deployment);
  const handleCloseDetails = () => setSelectedDeployment(null);

  return (
    // Używamy komponentów MUI do stworzenia podstawowego layoutu
    <>
      {/* Górny pasek aplikacji */}
      <AppBar position="static">
        <Toolbar>
          <Typography variant="h6" component="div" sx={{ flexGrow: 1 }}>
            K8s Resource Manager
          </Typography>
        </Toolbar>
      </AppBar>

      {/* Główny kontener na zawartość */}
      <Container maxWidth="lg" sx={{ mt: 4, mb: 4 }}> {/* mt: margin-top, mb: margin-bottom */}
        {loading && (
          // Prosty wskaźnik ładowania
          <Box sx={{ display: 'flex', justifyContent: 'center', p: 5 }}>
            <CircularProgress />
          </Box>
        )}

        {error && (
          // Komponent Alert do wyświetlania błędów
          <Alert severity="error" sx={{ mt: 2 }}>{error}</Alert>
        )}

        {/* Wyświetl tabelę tylko jeśli nie ma ładowania i nie ma błędu */}
        {!loading && !error && (
           <Paper sx={{ width: '100%', overflow: 'hidden' }}> {/* Paper to kontener z tłem i cieniem */}
             <TableContainer sx={{ maxHeight: 640 }}> {/* Ogranicza wysokość i dodaje scroll */}
               <Table stickyHeader aria-label="deployments table"> {/* stickyHeader przypina nagłówek */}
                 <TableHead>
                   <TableRow>
                     {/* Definiujemy nagłówki kolumn */}
                     <TableCell>Namespace</TableCell>
                     <TableCell>Name</TableCell>
                     <TableCell>CPU Req.</TableCell>
                     <TableCell>CPU Lim.</TableCell>
                     <TableCell>CPU Usage</TableCell>
                     <TableCell>Mem Req.</TableCell>
                     <TableCell>Mem Lim.</TableCell>
                     <TableCell>Mem Usage</TableCell>
                     <TableCell align="center">Recs 💡</TableCell> {/* Wyśrodkowana kolumna */}
                   </TableRow>
                 </TableHead>
                 <TableBody>
                   {deployments.length === 0 ? (
                     <TableRow>
                       <TableCell colSpan={9} align="center">
                         No deployments found in cluster.
                       </TableCell>
                     </TableRow>
                   ) : (
                     deployments.map((deployment) => (
                       <TableRow
                         hover // Efekt podświetlenia przy najechaniu
                         key={`${deployment.namespace}-${deployment.name}`}
                       >
                         <TableCell>{deployment.namespace}</TableCell>
                         <TableCell>{deployment.name}</TableCell>
                         <TableCell>{deployment.cpuRequests || '-'}</TableCell>
                         <TableCell>{deployment.cpuLimits === "0" ? '-' : deployment.cpuLimits}</TableCell> {/* Poprawka dla "0" */}
                         <TableCell>{deployment.currentCpuUsage}m</TableCell>
                         <TableCell>{deployment.memoryRequests || '-'}</TableCell>
                         <TableCell>{deployment.memoryLimits === "0" ? '-' : deployment.memoryLimits}</TableCell> {/* Poprawka dla "0" */}
                         <TableCell>{formatBytes(deployment.currentMemoryUsage)}</TableCell>
                         <TableCell align="center">
                           {deployment.recommendations.length > 0 ? (
                             // Chip to ładna etykieta z liczbą, która jest klikalna
                             <Chip
                               icon={<InfoIcon />}
                               label={deployment.recommendations.length}
                               onClick={() => handleShowDetails(deployment)}
                               color="warning" // Kolor ostrzegawczy
                               size="small"
                               clickable // Pokazuje, że można kliknąć
                             />
                           ) : (
                             <Chip label="0" size="small" /> // Szary chip z zerem
                           )}
                         </TableCell>
                       </TableRow>
                     ))
                   )}
                 </TableBody>
               </Table>
             </TableContainer>
           </Paper>
        )}
      </Container>

      {/* --- MODAL Z MUI --- */}
      <Modal
        open={Boolean(selectedDeployment)} // Modal jest otwarty, gdy selectedDeployment nie jest null
        onClose={handleCloseDetails} // Funkcja do zamknięcia modala (np. kliknięcie obok)
        aria-labelledby="recommendation-modal-title"
        aria-describedby="recommendation-modal-description"
      >
        {/* Box zawiera zawartość modala i używa zdefiniowanego stylu */}
        <Box sx={modalStyle}>
          <Typography id="recommendation-modal-title" variant="h6" component="h2">
             Recommendations for {selectedDeployment?.namespace}/{selectedDeployment?.name}
          </Typography>
          <ul id="recommendation-modal-description" style={{ marginTop: '16px', paddingLeft: '20px' }}>
            {selectedDeployment?.recommendations.map((rec, index) => (
              <li key={index} style={{ marginBottom: '10px' }}>{rec}</li>
            ))}
          </ul>
          <Button onClick={handleCloseDetails} variant="contained" sx={{ mt: 2 }}>
            Close
          </Button>
        </Box>
      </Modal>
      {/* --- KONIEC MODALA Z MUI --- */}
    </>
  );
}

export default App;