import {
  Typography, Paper, Box, FormControlLabel, Switch,
  Select, MenuItem, FormControl, InputLabel
} from '@mui/material';
import { useTranslation } from 'react-i18next'; // <-- IMPORT i18n
import { useThemeMode } from '../context/ThemeContext';

export default function Settings() {
  // Pobieramy dane z kontekstów
  const { t, i18n } = useTranslation();
  const { mode, toggleThemeMode } = useThemeMode(); // <-- Używamy kontekstu

  const handleLanguageChange = (event) => {
    i18n.changeLanguage(event.target.value);
  };

  return (
    <Paper sx={{ p: 3, maxWidth: 600 }}>
      <Typography variant="h4" gutterBottom>
        {t('settings.title')}
      </Typography>
      
      {/* Przełącznik Motywu */}
      <Box sx={{ mb: 4 }}>
        <Typography variant="h6">{t('settings.appearance')}</Typography>
        <FormControlLabel
          control={
            <Switch
              checked={mode === 'dark'}
              onChange={toggleThemeMode}
            />
          }
          label={mode === 'dark' ? t('settings.dark_mode') : t('settings.light_mode')}
        />
        <Typography variant="body2" color="text.secondary">
          {t('settings.theme_toggle_desc')}
        </Typography>
      </Box>

      {/* Przełącznik Języka */}
      <Box>
        <Typography variant="h6">{t('settings.language')}</Typography>
        <FormControl variant="standard" sx={{ m: 1, minWidth: 120 }}>
          <InputLabel id="lang-select-label">{t('settings.language')}</InputLabel>
          <Select
            labelId="lang-select-label"
            value={i18n.language.startsWith('pl') ? 'pl' : 'en'}
            onChange={handleLanguageChange}
          >
            <MenuItem value={'en'}>English</MenuItem>
            <MenuItem value={'pl'}>Polski</MenuItem>
          </Select>
        </FormControl>
        <Typography variant="body2" color="text.secondary">
          {t('settings.language_desc')}
        </Typography>
      </Box>
    </Paper>
  );
}