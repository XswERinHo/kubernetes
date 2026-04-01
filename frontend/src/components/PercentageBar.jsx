// frontend/src/components/PercentageBar.jsx

import { Box, LinearProgress, useTheme } from '@mui/material';

// Funkcja pomocnicza do kolorowania paska
const getBarColor = (value, theme) => {
  if (value > 85) return theme.palette.error.main;
  if (value > 65) return theme.palette.warning.main;
  return theme.palette.success.main;
};

export default function PercentageBar({ value, label, colorStart, colorEnd }) {
  const theme = useTheme();
  
  // Domyślne kolory jeśli nie podano (zachowanie kompatybilności)
  const defaultColor = getBarColor(value, theme);
  
  const backgroundStyle = (colorStart && colorEnd) 
    ? `linear-gradient(90deg, ${colorStart} 0%, ${colorEnd} 100%)`
    : defaultColor;

  return (
    <Box sx={{ width: '100%' }}>
      <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 0.5 }}>
        {label && (
          <Box component="span" sx={{ fontSize: '0.75rem', fontWeight: 600, color: 'text.secondary' }}>
            {label}
          </Box>
        )}
      </Box>
      <LinearProgress
        variant="determinate"
        value={value}
        sx={{
          height: 8,
          borderRadius: 4,
          backgroundColor: theme.palette.mode === 'dark' ? 'rgba(255,255,255,0.1)' : 'rgba(0,0,0,0.1)',
          [`& .MuiLinearProgress-bar`]: {
            background: backgroundStyle,
            borderRadius: 4,
          },
        }}
      />
    </Box>
  );
}