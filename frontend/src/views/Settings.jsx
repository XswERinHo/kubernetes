import {
  Typography, Paper, Box, FormControlLabel, Switch,
  Select, MenuItem, FormControl, InputLabel,
  TextField, Button, List, ListItem, ListItemText, IconButton, Divider, Alert
} from '@mui/material';
import DeleteIcon from '@mui/icons-material/Delete';
import { useState, useEffect, useCallback } from 'react';
import { useTranslation } from 'react-i18next'; // <-- IMPORT i18n
import { useThemeMode } from '../context/ThemeContext';
import { useAuth } from '../context/AuthContext';

export default function Settings() {
  // Pobieramy dane z kontekstów
  const { t, i18n } = useTranslation();
  const { mode, toggleThemeMode } = useThemeMode(); // <-- Używamy kontekstu
  const { userRole, getAuthHeader } = useAuth();

  const [users, setUsers] = useState([]);
  const [newUser, setNewUser] = useState({ username: '', password: '', role: 'Viewer' });
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');

  const handleLanguageChange = (event) => {
    i18n.changeLanguage(event.target.value);
  };

  const fetchUsers = useCallback(() => {
    if (userRole !== 'Admin') return;
    fetch('/api/users', { headers: getAuthHeader() })
      .then(res => res.json())
      .then(data => setUsers(data || []))
      .catch(err => console.error(err));
  }, [userRole, getAuthHeader]);

  useEffect(() => {
    fetchUsers();
  }, [fetchUsers]);

  const handleAddUser = () => {
    console.log('Adding user:', newUser);
    setError('');
    setSuccess('');
    if (!newUser.username || !newUser.password) {
      setError(t('settings.required_fields'));
      return;
    }
    fetch('/api/users', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...getAuthHeader() },
      body: JSON.stringify(newUser)
    })
    .then(async res => {
      if (!res.ok) {
        const text = await res.text();
        throw new Error(text || 'Failed to create user');
      }
      setSuccess(t('settings.user_created'));
      setNewUser({ username: '', password: '', role: 'Viewer' });
      fetchUsers();
    })
    .catch(err => {
      console.error('Add user error:', err);
      setError(err.message);
      alert('Error adding user: ' + err.message);
    });
  };

  const handleDeleteUser = (username) => {
    if (!window.confirm(`Delete user ${username}?`)) return;
    fetch(`/api/users/${username}`, {
      method: 'DELETE',
      headers: getAuthHeader()
    })
    .then(res => {
      if (!res.ok) throw new Error('Failed to delete');
      fetchUsers();
    })
    .catch(err => console.error(err));
  };

  return (
    <Paper sx={{ p: 3, maxWidth: 800 }}>
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
      <Box sx={{ mb: 4 }}>
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

      {userRole === 'Admin' && (
        <>
          <Divider sx={{ my: 3 }} />
          <Box>
            <Typography variant="h6" gutterBottom>{t('settings.user_management')}</Typography>
            
            {error && <Alert severity="error" sx={{ mb: 2 }}>{error}</Alert>}
            {success && <Alert severity="success" sx={{ mb: 2 }}>{success}</Alert>}

            <Box sx={{ display: 'flex', gap: 2, mb: 3, alignItems: 'flex-end' }}>
              <TextField 
                label={t('settings.username')} 
                variant="standard" 
                value={newUser.username} 
                onChange={e => setNewUser({...newUser, username: e.target.value})}
              />
              <TextField 
                label={t('settings.password')} 
                type="password" 
                variant="standard" 
                value={newUser.password} 
                onChange={e => setNewUser({...newUser, password: e.target.value})}
              />
              <FormControl variant="standard" sx={{ minWidth: 120 }}>
                <InputLabel>{t('settings.role')}</InputLabel>
                <Select
                  value={newUser.role}
                  onChange={e => setNewUser({...newUser, role: e.target.value})}
                >
                  <MenuItem value="Admin">Admin</MenuItem>
                  <MenuItem value="Editor">Editor</MenuItem>
                  <MenuItem value="Viewer">Viewer</MenuItem>
                </Select>
              </FormControl>
              <Button variant="contained" onClick={handleAddUser}>{t('settings.add_user')}</Button>
            </Box>

            <List>
              {users.map(user => (
                <ListItem 
                  key={user.username}
                  secondaryAction={
                    user.username !== 'admin' && (
                      <IconButton edge="end" onClick={() => handleDeleteUser(user.username)}>
                        <DeleteIcon />
                      </IconButton>
                    )
                  }
                >
                  <ListItemText 
                    primary={user.username} 
                    secondary={`${t('settings.role')}: ${user.role}`} 
                  />
                </ListItem>
              ))}
            </List>
          </Box>
        </>
      )}
    </Paper>
  );
}