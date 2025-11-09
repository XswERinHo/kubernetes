// frontend/src/components/ProtectedRoute.jsx

import { Navigate, Outlet } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';

/**
 * Ten komponent sprawdza, czy użytkownik jest zalogowany.
 * Jeśli tak, renderuje komponenty-dzieci (np. MainLayout).
 * Jeśli nie, przekierowuje na stronę logowania.
 */
export default function ProtectedRoute() {
  const { isAuthenticated } = useAuth();

  if (!isAuthenticated) {
    // Przekieruj na /login, jeśli nie ma tokenu
    return <Navigate to="/login" replace />;
  }

  // Jeśli jest token, renderuj zawartość (np. MainLayout z jego <Outlet />)
  return <Outlet />;
}