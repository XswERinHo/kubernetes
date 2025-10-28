import { useState, useEffect } from 'react';
import './App.css';

// Funkcja formatująca bajty (bez zmian)
function formatBytes(bytes, decimals = 2) {
  if (bytes === 0) return '0 Bytes';
  const k = 1024;
  const dm = decimals < 0 ? 0 : decimals;
  const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i];
}

function App() {
  const [deployments, setDeployments] = useState([]);
  const [error, setError] = useState(null);
  // --- NOWY STAN DO POKAZYWANIA SZCZEGÓŁÓW ---
  const [selectedDeployment, setSelectedDeployment] = useState(null); // Przechowuje obiekt wdrożenia do pokazania w modalu

  useEffect(() => {
    fetch('/api/deployments')
      .then((response) => {
        if (!response.ok) { throw new Error(`HTTP error! status: ${response.status}`); }
        return response.json();
      })
      .then((data) => {
        // Upewniamy się, że recommendations zawsze jest tablicą
        const deploymentsWithRecs = (data || []).map(dep => ({
          ...dep,
          recommendations: dep.recommendations || []
        }));
        setDeployments(deploymentsWithRecs);
      })
      .catch((error) => {
        console.error('Error fetching data:', error);
        setError(`Failed to load deployments: ${error.message}`);
      });
  }, []);

  // --- FUNKCJE DO OBSŁUGI MODALA ---
  const handleShowDetails = (deployment) => {
    setSelectedDeployment(deployment);
  };
  const handleCloseDetails = () => {
    setSelectedDeployment(null);
  };
  // --- KONIEC FUNKCJI MODALA ---


  if (error) { /* ... (obsługa błędów bez zmian) ... */
     return (
      <div className="App">
        <h1>K8s Resource Manager</h1>
        <div className="card error-card"> <h2>Error Loading Data</h2> <p>{error}</p> </div>
      </div>
    );
  }

  const tableStyle = { /* ... (style bez zmian) ... */ width: '100%', borderCollapse: 'collapse', marginTop: '20px', };
  const thStyle = { /* ... (style bez zmian) ... */ borderBottom: '2px solid #646cff', padding: '12px 15px', textAlign: 'left', backgroundColor: '#3a3a3a', };
  const tdStyle = { /* ... (style bez zmian) ... */ borderBottom: '1px solid #444', padding: '10px 15px', verticalAlign: 'top'}; // verticalAlign: top dla lepszego wyglądu
  // --- NOWY STYL DLA PRZYCISKU SZCZEGÓŁÓW ---
  const detailsButtonStyle = {
      padding: '5px 10px',
      fontSize: '0.9em',
      cursor: 'pointer',
      backgroundColor: '#555',
      border: 'none',
      borderRadius: '4px',
      color: 'white',
      marginLeft: '10px',
  };

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
                  <th style={thStyle}>CPU Usage</th>
                  <th style={thStyle}>Mem Req.</th>
                  <th style={thStyle}>Mem Lim.</th>
                  <th style={thStyle}>Mem Usage</th>
                  {/* NOWY NAGŁÓWEK DLA REKOMENDACJI */}
                  <th style={thStyle}>Recs 💡</th>
                </tr>
              </thead>
              <tbody>
                {deployments.map((deployment) => (
                  <tr key={`${deployment.namespace}-${deployment.name}`}>
                    <td style={tdStyle}>{deployment.namespace}</td>
                    <td style={tdStyle}>{deployment.name}</td>
                    <td style={tdStyle}>{deployment.cpuRequests || '-'}</td>
                    <td style={tdStyle}>{deployment.cpuLimits || '-'}</td>
                    <td style={tdStyle}>{deployment.currentCpuUsage}m</td>
                    <td style={tdStyle}>{deployment.memoryRequests || '-'}</td>
                    <td style={tdStyle}>{deployment.memoryLimits || '-'}</td>
                    <td style={tdStyle}>{formatBytes(deployment.currentMemoryUsage)}</td>
                    {/* NOWA KOMÓRKA Z LICZBĄ REKOMENDACJI I PRZYCISKIEM */}
                    <td style={{...tdStyle, textAlign: 'center'}}>
                      {deployment.recommendations.length > 0 ? (
                        <>
                          <span style={{ fontWeight: 'bold' }}>{deployment.recommendations.length}</span>
                          <button style={detailsButtonStyle} onClick={() => handleShowDetails(deployment)}>
                            Details
                          </button>
                        </>
                      ) : (
                        <span>0</span> // Wyświetl 0, jeśli nie ma rekomendacji
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* --- NOWY KOMPONENT MODAL DO WYŚWIETLANIA REKOMENDACJI --- */}
      {selectedDeployment && (
        <div style={modalOverlayStyle}>
          <div style={modalContentStyle}>
            <h3>Recommendations for {selectedDeployment.namespace}/{selectedDeployment.name}</h3>
            <ul>
              {selectedDeployment.recommendations.map((rec, index) => (
                <li key={index} style={{ marginBottom: '10px' }}>{rec}</li>
              ))}
            </ul>
            <button style={{...detailsButtonStyle, marginTop: '20px'}} onClick={handleCloseDetails}>
              Close
            </button>
          </div>
        </div>
      )}
      {/* --- KONIEC MODALA --- */}

    </div>
  );
}

// --- Style dla Modala (dodaj je na dole pliku App.jsx) ---
const modalOverlayStyle = {
  position: 'fixed',
  top: 0,
  left: 0,
  right: 0,
  bottom: 0,
  backgroundColor: 'rgba(0, 0, 0, 0.7)',
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  zIndex: 1000, // Aby był na wierzchu
};

const modalContentStyle = {
  backgroundColor: '#333',
  padding: '30px',
  borderRadius: '8px',
  maxWidth: '600px',
  width: '90%',
  boxShadow: '0 5px 15px rgba(0, 0, 0, 0.5)',
};
// --- Koniec stylów dla Modala ---


export default App;