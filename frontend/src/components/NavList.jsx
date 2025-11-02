import { Link as RouterLink } from 'react-router-dom';
import { List, ListItem, ListItemButton, ListItemIcon, ListItemText } from '@mui/material';
import DashboardIcon from '@mui/icons-material/Dashboard';
import ListAltIcon from '@mui/icons-material/ListAlt';
import SettingsIcon from '@mui/icons-material/Settings';
import { useTranslation } from 'react-i18next'; // <-- IMPORT i18n

// Definicja linków używa teraz kluczy 'tKey'
const navItems = [
  { tKey: 'nav.dashboard', icon: <DashboardIcon />, path: '/dashboard' },
  { tKey: 'nav.workloads', icon: <ListAltIcon />, path: '/workloads' },
  { tKey: 'nav.settings', icon: <SettingsIcon />, path: '/settings' },
];

export default function NavList() {
  const { t } = useTranslation(); // <-- Używamy hooka

  return (
    <List>
      {navItems.map((item) => (
        <ListItem key={item.tKey} disablePadding>
          <ListItemButton component={RouterLink} to={item.path}>
            <ListItemIcon>
              {item.icon}
            </ListItemIcon>
            {/* Używamy t() do tłumaczenia tekstu */}
            <ListItemText primary={t(item.tKey)} />
          </ListItemButton>
        </ListItem>
      ))}
    </List>
  );
}