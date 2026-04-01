// frontend/src/components/NavList.jsx

import { Link as RouterLink } from 'react-router-dom';
import { List, ListItem, ListItemButton, ListItemIcon, ListItemText } from '@mui/material';
import DashboardIcon from '@mui/icons-material/Dashboard';
import ListAltIcon from '@mui/icons-material/ListAlt';
import SettingsIcon from '@mui/icons-material/Settings';
import HardwareIcon from '@mui/icons-material/Hardware';
import FactCheckIcon from '@mui/icons-material/FactCheck';
import NotificationsActiveIcon from '@mui/icons-material/NotificationsActive';
import { useTranslation } from 'react-i18next';

import { useAuth } from '../context/AuthContext';

export default function NavList() {
  const { t } = useTranslation(); 
  const { userRole } = useAuth();

  const navItems = [
    { tKey: 'nav.dashboard', icon: <DashboardIcon />, path: '/dashboard' },
    { tKey: 'nav.workloads', icon: <ListAltIcon />, path: '/workloads' },
    { tKey: 'nav.nodes', icon: <HardwareIcon />, path: '/nodes' },
    { tKey: 'nav.alerts', icon: <NotificationsActiveIcon />, path: '/alerts' },
  ];

  if (userRole === 'Admin') {
    navItems.push({ tKey: 'nav.approvals', icon: <FactCheckIcon />, path: '/approvals' });
  }

  navItems.push({ tKey: 'nav.settings', icon: <SettingsIcon />, path: '/settings' });

  return (
    <List>
      {navItems.map((item) => (
        <ListItem key={item.tKey} disablePadding>
          <ListItemButton component={RouterLink} to={item.path}>
            <ListItemIcon>
              {item.icon}
            </ListItemIcon>
            <ListItemText primary={t(item.tKey)} />
          </ListItemButton>
        </ListItem>
      ))}
    </List>
  );
}