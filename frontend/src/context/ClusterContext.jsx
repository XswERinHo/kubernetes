// frontend/src/context/ClusterContext.jsx

import { createContext, useContext, useState, useEffect, useMemo } from 'react';
import { useAuth } from './AuthContext'; // <-- NOWY IMPORT

const ClusterContext = createContext(null);

export function ClusterProvider({ children }) {
  const { isAuthenticated, getAuthHeader } = useAuth(); // <-- POBIERAMY STAN I FUNKCJĘ z AuthContext
  
  const [clusters, setClusters] = useState([]);
  const [selectedCluster, setSelectedCluster] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  useEffect(() => {
    // --- ZMIANA: Nie rób nic, jeśli nie jesteśmy zalogowani ---
    if (!isAuthenticated) {
      setLoading(false);
      return;
    }
    
    setLoading(true);
    setError(null);

    fetch('/api/clusters', {
      headers: getAuthHeader() // <-- UŻYWAMY NAGŁÓWKA AUTORYZACJI
    })
      .then(res => {
        if (res.status === 401) {
          throw new Error('Unauthorized: Token may be invalid. Please log in again.');
        }
        if (!res.ok) {
          throw new Error('Failed to fetch cluster list');
        }
        return res.json();
      })
      .then((clusterNames) => {
        setClusters(clusterNames || []);
        if (clusterNames && clusterNames.length > 0) {
          setSelectedCluster(clusterNames[0]);
        } else {
          setError('No clusters found in kubeconfig.');
        }
      })
      .catch(err => {
        console.error(err);
        setError(err.message);
      })
      .finally(() => {
        setLoading(false);
      });
  }, [isAuthenticated, getAuthHeader]); // <-- ZALEŻNOŚĆ OD STANU LOGOWANIA

  const value = useMemo(() => ({
    clusters,
    selectedCluster,
    setSelectedCluster,
    loading,
    error
  }), [clusters, selectedCluster, loading, error]);

  return (
    <ClusterContext.Provider value={value}>
      {children}
    </ClusterContext.Provider>
  );
}

export const useCluster = () => {
  // ... (bez zmian)
  const context = useContext(ClusterContext);
  if (!context) {
    throw new Error('useCluster must be used within a ClusterProvider');
  }
  return context;
};