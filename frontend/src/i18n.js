// Pełna zawartość pliku:
// frontend/src/i18n.js

import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import LanguageDetector from 'i18next-browser-languagedetector';

// --- POPRAWKA ŚCIEŻKI ---
// Importujemy pliki z katalogu 'src/locales'
import translationEN from './locales/en/translation.json';
import translationPL from './locales/pl/translation.json';
// --- KONIEC POPRAWKI ---

// Definicja zasobów (tłumaczeń)
const resources = {
  en: {
    translation: translationEN
  },
  pl: {
    translation: translationPL
  }
};

i18n
  .use(LanguageDetector) // Wykrywa język użytkownika
  .use(initReactI18next) // Łączy i18n z Reactem
  .init({
    resources,
    fallbackLng: 'en', // Język używany, gdy tłumaczenie nie jest dostępne
    debug: true, // Włącz logowanie w konsoli (przydatne przy dewelopmencie)
    
    interpolation: {
      escapeValue: false, // React już chroni przed XSS
    },
  });

export default i18n;