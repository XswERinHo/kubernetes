import { useState, useEffect } from 'react';
import './App.css';

function App() {
  // 'useState' tworzy "pudełko" w pamięci komponentu do przechowywania danych
  const [message, setMessage] = useState('Loading...');

  // 'useEffect' uruchamia kod po pierwszym wyrenderowaniu komponentu
  useEffect(() => {
    // Używamy 'fetch', aby wysłać zapytanie do naszego backendu
    fetch('/api/health')
      .then((response) => response.text()) // Oczekujemy odpowiedzi tekstowej
      .then((data) => setMessage(data)) // Zapisujemy odpowiedź w naszym "pudełku"
      .catch((error) => {
        console.error('Error fetching data:', error);
        setMessage('Failed to load data from API');
      });
  }, []); // Pusta tablica [] oznacza, że efekt uruchomi się tylko raz

  return (
    <>
      <h1>Vite + React + Go</h1>
      <div className="card">
        <h2>Message from API:</h2>
        <p>{message}</p>
      </div>
    </>
  );
}

export default App;