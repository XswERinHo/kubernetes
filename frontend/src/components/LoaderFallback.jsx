import { Box, CircularProgress } from '@mui/material';

// Komponent zastępczy, gdy tłumaczenia się ładują
export default function LoaderFallback() {
  return (
    <Box
      sx={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        height: '100vh',
        width: '100vw',
        backgroundColor: '#242424' // Pasuje do tła
      }}
    >
      <CircularProgress />
    </Box>
  );
}