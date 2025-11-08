// frontend/src/context/ClusterContext.jsx

import { createContext, useContext, useState, useEffect, useMemo } from 'react';

// Tworzymy kontekst
const ClusterContext = createContext(null);

/**
 * Provider, który pobiera listę klastrów i zarządza stanem
 */
export function ClusterProvider({ children }) {
  const [clusters, setClusters] = useState([]);
  const [selectedCluster, setSelectedCluster] = useState(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  // Przy pierwszym załadowaniu pobieramy listę klastrów
  useEffect(() => {
    fetch('/api/clusters')
      .then(res => {
        if (!res.ok) {
          throw new Error('Failed to fetch cluster list');
        }
        return res.json();
      })
      .then((clusterNames) => {
        setClusters(clusterNames || []);
        // Jeśli mamy klastry, wybierz pierwszy z listy jako domyślny
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
  }, []); // Pusta tablica oznacza "uruchom tylko raz"

  // Tworzymy wartość kontekstu, którą przekażemy "w dół"
  const value = useMemo(() => ({
    clusters,
    selectedCluster,
    setSelectedCluster, // Funkcja do zmiany klastra
    loading,
    error
  }), [clusters, selectedCluster, loading, error]);

  return (
    <ClusterContext.Provider value={value}>
      {children}
    </ClusterContext.Provider>
  );
}

/**
 * Hook pomocniczy do łatwego używania kontekstu
 * @returns {{clusters: string[], selectedCluster: string, setSelectedCluster: (cluster: string) => void, loading: boolean, error: string}}
 */
export const useCluster = () => {
  const context = useContext(ClusterContext);
  if (!context) {
    throw new Error('useCluster must be used within a ClusterProvider');
  }
  return context;
};