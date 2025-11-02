import { useTranslation } from 'react-i18next';
import { useCallback } from 'react';

// Zakładamy stały kurs (1 PLN = 0.25 USD, czyli 1 USD = 4 PLN)
// W przyszłości można to przenieść do pliku konfiguracyjnego.
const PLN_TO_USD_RATE = 0.2712;

/**
 * Inteligentny hook do formatowania walut.
 * Automatycznie wykrywa język (PL/EN) i konwertuje
 * wartość bazową (zawsze w PLN) na odpowiednią walutę (PLN lub USD).
 * Zwraca sformatowany string, np. "35,09 zł" lub "$8.77".
 */
export const useCurrencyFormatter = () => {
  const { i18n } = useTranslation();

  const formatCurrency = useCallback((plnValue) => {
    if (plnValue === null || typeof plnValue === 'undefined') {
      return '-';
    }

    const lang = i18n.language;
    const isPl = lang.startsWith('pl');

    // Krok 1: Konwersja wartości
    const value = isPl ? plnValue : plnValue * PLN_TO_USD_RATE;
    // Krok 2: Ustawienie waluty (PLN lub USD)
    const currency = isPl ? 'PLN' : 'USD';

    // Krok 3: Użycie natywnego API przeglądarki do formatowania
    // 'lang' (np. 'pl' lub 'en') automatycznie ustawi separatory (kropki/przecinki)
    const options = {
      style: 'currency',
      currency: currency,
    };

    return new Intl.NumberFormat(lang, options).format(value);
  }, [i18n.language]); // Funkcja przebuduje się tylko, gdy zmieni się język

  return formatCurrency; // Zwracamy gotową do użycia funkcję
};