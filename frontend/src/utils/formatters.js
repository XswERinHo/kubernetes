// Pełna zawartość pliku:
// frontend/src/utils/formatters.js

// Funkcja formatBytes
export function formatBytes(bytes, decimals = 2) { if (!bytes || bytes === 0) return '0 Bytes'; const k = 1024; const dm = decimals < 0 ? 0 : decimals; const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB']; const i = Math.floor(Math.log(bytes) / Math.log(k)); if (i < 0 || i >= sizes.length) return '0 Bytes'; return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i]; }

// Funkcja formatowania waluty została USUNIĘTA

// Funkcje parsowania zasobów
export function parseCpu(valueString) {
  // ... (bez zmian)
  if (!valueString || valueString === "0") return null;
  if (valueString.endsWith('m')) {
    return parseInt(valueString.slice(0, -1), 10);
  }
  const cores = parseFloat(valueString);
  return cores * 1000;
}

export function parseMemory(valueString) {
  // ... (bez zmian)
  if (!valueString || valueString === "0") return null;
  const units = { 'Ki': 1024, 'Mi': 1024**2, 'Gi': 1024**3, 'Ti': 1024**4 };
  const match = valueString.match(/^(\d+)(Ki|Mi|Gi|Ti)?$/);
  if (!match) return null;
  const value = parseInt(match[1], 10);
  const unit = match[2];
  if (units[unit]) {
    return value * units[unit];
  }
  return value;
}