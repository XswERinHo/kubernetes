// frontend/src/components/ClusterSelector.jsx

import { FormControl, Select, MenuItem, Box, CircularProgress, Typography } from '@mui/material';
import { useCluster } from '../context/ClusterContext';
import { useTranslation } from 'react-i18next';

export default function ClusterSelector() {
  const { t } = useTranslation();
  const { clusters, selectedCluster, setSelectedCluster, loading, error } = useCluster();

  const handleChange = (event) => {
    setSelectedCluster(event.target.value);
  };

  if (loading) {
    return (
      <Box sx={{ display: 'flex', alignItems: 'center', color: 'inherit', mr: 2 }}>
        <CircularProgress color="inherit" size={20} sx={{ mr: 1 }} />
        <Typography variant="body2">{t('app_bar.loading_clusters')}</Typography>
      </Box>
    );
  }

  if (error || clusters.length === 0) {
    return (
      <Typography variant="body2" sx={{ mr: 2, color: 'error.main' }}>
        {t('app_bar.no_clusters_found')}
      </Typography>
    );
  }

  return (
    <FormControl variant="standard" sx={{ minWidth: 180, mr: 2 }}>
      <Select
        value={selectedCluster}
        onChange={handleChange}
        disableUnderline
        sx={{
          color: 'inherit',
          '& .MuiSvgIcon-root': {
            color: 'inherit',
          },
        }}
      >
        {clusters.map((clusterName) => (
          <MenuItem key={clusterName} value={clusterName}>
            {clusterName}
          </MenuItem>
        ))}
      </Select>
    </FormControl>
  );
}