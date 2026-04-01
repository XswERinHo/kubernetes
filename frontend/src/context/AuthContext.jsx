import { createContext, useContext, useState, useMemo, useCallback } from 'react';

const AuthContext = createContext(null);

export function AuthProvider({ children }) {
  const [token, setToken] = useState(localStorage.getItem('authToken'));
  const [userRole, setUserRole] = useState(localStorage.getItem('userRole'));

  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);

  const login = async (username, password) => {
    setLoading(true);
    setError(null);
    try {
      const response = await fetch('/api/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, password }),
      });

      if (!response.ok) {
        const errorText = await response.text();
        throw new Error(errorText || 'Błąd logowania');
      }

      const { token, role } = await response.json();

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

  const logout = useCallback(() => {
    // if (token) {
    //   fetch('/api/auth/logout', {
    //     method: 'POST',
    //     headers: {
    //       'Content-Type': 'application/json',
    //       Authorization: `Bearer ${token}`,
    //     },
    //   }).catch((err) => console.error('Logout request failed', err));
    // }

    localStorage.removeItem('authToken');
    localStorage.removeItem('userRole');
    setToken(null);
    setUserRole(null);
  }, [token]);

  const value = useMemo(() => ({
    token,
    userRole,
    isAuthenticated: !!token,
    login,
    logout,
    loading,
    error,
    getAuthHeader: () => (token ? { 'Authorization': `Bearer ${token}` } : {}),
  }), [token, userRole, loading, error, logout]);

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