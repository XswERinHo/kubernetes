// frontend/src/components/PercentageBar.jsx

import { Box, LinearProgress, useTheme } from '@mui/material';

// Funkcja pomocnicza do kolorowania paska
const getBarColor = (value, theme) => {
  if (value > 85) return theme.palette.error.main;
  if (value > 65) return theme.palette.warning.main;
  return theme.palette.success.main;
};

export default function PercentageBar({ value }) {
  const theme = useTheme();
  const color = getBarColor(value, theme);

  return (
    <Box sx={{ display: 'flex', alignItems: 'center' }}>
      <Box sx={{ width: '100%', mr: 1 }}>
        <LinearProgress
          variant="determinate"
          value={value}
          sx={{
            height: 10,
            borderRadius: 5,
            [`& .MuiLinearProgress-bar`]: {
              backgroundColor: color,
            },
          }}
        />
      </Box>
    </Box>
  );
}