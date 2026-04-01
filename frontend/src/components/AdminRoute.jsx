import { Navigate } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';

/**
 * Renders children only for Admin role. Assumes user is already authenticated.
 */
export default function AdminRoute({ children }) {
  const { userRole } = useAuth();

  if (userRole !== 'Admin') {
    return <Navigate to="/dashboard" replace />;
  }

  return children;
}
