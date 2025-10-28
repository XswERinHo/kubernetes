import { useState, useEffect } from 'react';
import './App.css';

// --- NOWA FUNKCJA POMOCNICZA DO FORMATOWANIA BAJTÓW ---
// Konwertuje bajty na bardziej czytelny format (KB, MB, GB)
function formatBytes(bytes, decimals = 2) {
  if (bytes === 0) return '0 Bytes';
  const k = 1024;
  const dm = decimals < 0 ? 0 : decimals;
  const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i];
}
// --- KONIEC FUNKCJI POMOCNICZEJ ---

function App() {
  const [deployments, setDeployments] = useState([]);
  const [error, setError] = useState(null);

  useEffect(() => {
    fetch('/api/deployments')
      .then((response) => {
        if (!response.ok) {
          throw new Error(`HTTP error! status: ${response.status}`);
        }
        return response.json();
      })
      .then((data) => {
        setDeployments(data || []);
      })
      .catch((error) => {
        console.error('Error fetching data:', error);
        setError(`Failed to load deployments: ${error.message}`);
      });
  }, []);

  if (error) {
    // ... (obsługa błędów bez zmian)
     return (
      <div className="App">
        <h1>K8s Resource Manager</h1>
        <div className="card error-card">
          <h2>Error Loading Data</h2>
          <p>{error}</p>
        </div>
      </div>
    );
  }

  const tableStyle = { /* ... (style bez zmian) */ width: '100%', borderCollapse: 'collapse', marginTop: '20px', };
  const thStyle = { /* ... (style bez zmian) */ borderBottom: '2px solid #646cff', padding: '12px 15px', textAlign: 'left', backgroundColor: '#3a3a3a', };
  const tdStyle = { /* ... (style bez zmian) */ borderBottom: '1px solid #444', padding: '10px 15px', };

  return (
    <div className="App">
      <h1>K8s Resource Manager</h1>
      <div className="card">
        <h2>Deployments</h2>
        {deployments.length === 0 ? (
          <p>Loading deployments or none found in cluster...</p>
        ) : (
          <div style={{ overflowX: 'auto' }}>
            <table style={tableStyle}>
              <thead>
                <tr>
                  <th style={thStyle}>Namespace</th>
                  <th style={thStyle}>Name</th>
                  <th style={thStyle}>CPU Req.</th>
                  <th style={thStyle}>CPU Lim.</th>
                  {/* NOWY NAGŁÓWEK ZUŻYCIA CPU */}
                  <th style={thStyle}>CPU Usage</th>
                  <th style={thStyle}>Mem Req.</th>
                  <th style={thStyle}>Mem Lim.</th>
                  {/* NOWY NAGŁÓWEK ZUŻYCIA PAMIĘCI */}
                  <th style={thStyle}>Mem Usage</th>
                </tr>
              </thead>
              <tbody>
                {deployments.map((deployment) => (
                  <tr key={`${deployment.namespace}-${deployment.name}`}>
                    <td style={tdStyle}>{deployment.namespace}</td>
                    <td style={tdStyle}>{deployment.name}</td>
                    <td style={tdStyle}>{deployment.cpuRequests || '-'}</td>
                    <td style={tdStyle}>{deployment.cpuLimits || '-'}</td>
                    {/* NOWA KOMÓRKA - WYŚWIETLAMY ZUŻYCIE CPU W MILICPU */}
                    <td style={tdStyle}>{deployment.currentCpuUsage}m</td>
                    <td style={tdStyle}>{deployment.memoryRequests || '-'}</td>
                    <td style={tdStyle}>{deployment.memoryLimits || '-'}</td>
                    {/* NOWA KOMÓRKA - UŻYWAMY FUNKCJI formatBytes */}
                    <td style={tdStyle}>{formatBytes(deployment.currentMemoryUsage)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}

export default App;