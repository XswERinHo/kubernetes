// frontend/src/context/AuthContext.jsx

import { createContext, useContext, useState, useMemo } from 'react';

// Przechowujemy token i rolę
const AuthContext = createContext(null);

export function AuthProvider({ children }) {
  // Próbujemy odczytać zapisany stan z localStorage przy starcie
  const [token, setToken] = useState(localStorage.getItem('authToken'));
  const [userRole, setUserRole] = useState(localStorage.getItem('userRole'));

  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);

  // Funkcja logowania
  const login = async (username, password) => {
    setLoading(true);
    setError(null);
    try {
      const response = await fetch('/api/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, password }),
      });

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(errorText || 'Błąd logowania');
      }

      const { token, role } = await response.json();

      // Zapisz token i rolę w stanie i localStorage
      localStorage.setItem('authToken', token);
      localStorage.setItem('userRole', role);
      setToken(token);
      setUserRole(role);

      return true;
    } catch (err) {
      console.error(err);
      setError(err.message);
      return false;
    } finally {
      setLoading(false);
    }
  };

  // Funkcja wylogowania
  const logout = () => {
    localStorage.removeItem('authToken');
    localStorage.removeItem('userRole');
    setToken(null);
    setUserRole(null);
  };

  // Stworzenie "publicznego" API naszego kontekstu
  const value = useMemo(() => ({
    token,
    userRole,
    isAuthenticated: !!token, // Prosty boolean, czy jesteśmy zalogowani
    login,
    logout,
    loading,
    error,
    // Funkcja pomocnicza do tworzenia nagłówków
    getAuthHeader: () => ({
      'Authorization': `Bearer ${token}`
    })
  }), [token, userRole, loading, error]);

  return (
    <AuthContext.Provider value={value}>
      {children}
    </AuthContext.Provider>
  );
}

/**
 * Hook pomocniczy do używania kontekstu autoryzacji
 * @returns {{token: string, userRole: string, isAuthenticated: boolean, login: (u: string, p: string) => Promise<boolean>, logout: () => void, loading: boolean, error: string, getAuthHeader: () => {Authorization: string}}}
 */
export const useAuth = () => {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
};