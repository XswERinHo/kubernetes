import { useState, useEffect } from 'react';
import './App.css';

function App() {
  // Zmieniamy stan: zamiast 'message' będziemy przechowywać listę (tablicę) wdrożeń
  const [deployments, setDeployments] = useState([]);
  const [error, setError] = useState(null);

  useEffect(() => {
    // Zmieniamy endpoint: zamiast '/api/health' odpytujemy '/api/deployments'
    fetch('/api/deployments')
      .then((response) => {
        if (!response.ok) {
          throw new Error('Network response was not ok');
        }
        return response.json(); // Oczekujemy odpowiedzi w formacie JSON
      })
      .then((data) => {
        setDeployments(data); // Zapisujemy tablicę wdrożeń w naszym stanie
      })
      .catch((error) => {
        console.error('Error fetching data:', error);
        setError(error.message);
      });
  }, []);

  // Prosta obsługa błędów
  if (error) {
    return (
      <>
        <h1>K8s Resource Manager</h1>
        <div className="card">
          <h2>Error:</h2>
          <p>{error}</p>
        </div>
      </>
    );
  }

  // Nowy JSX do wyświetlania danych
  return (
    <>
      <h1>K8s Resource Manager</h1>
      <div className="card">
        <h2>Deployments</h2>
        {/* Sprawdzamy, czy lista jest pusta */}
        {deployments.length === 0 ? (
          <p>No deployments found in cluster.</p>
        ) : (
          // Jeśli lista nie jest pusta, tworzymy tabelę
          <table style={{ width: '100%', textAlign: 'left' }}>
            <thead>
              <tr>
                <th>Namespace</th>
                <th>Name</th>
              </tr>
            </thead>
            <tbody>
              {/* Używamy .map() aby zamienić każdy obiekt z listy na wiersz tabeli */}
              {deployments.map((deployment) => (
                <tr key={deployment.name}>
                  <td>{deployment.namespace}</td>
                  <td>{deployment.name}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </>
  );
}

export default App;