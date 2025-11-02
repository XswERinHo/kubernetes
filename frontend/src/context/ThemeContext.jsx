import { createContext, useContext } from 'react';

// Tworzymy kontekst, który będzie przechowywał funkcję I aktualny tryb
export const ThemeModeContext = createContext({
  mode: 'dark', // Domyślna wartość
  toggleThemeMode: () => {},
});

// Hook pomocniczy
export const useThemeMode = () => useContext(ThemeModeContext);