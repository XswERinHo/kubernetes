// Funkcja parsowania rekomendacji
export function parseActionableRecommendation(recText) {
  const matchCpu = recText.match(/Niskie zużycie CPU.*?Rozważ zmniejszenie żądań do (\d+m)/i);
  if (matchCpu && matchCpu[1]) {
    return { type: 'apply', resource: 'cpuRequests', value: matchCpu[1], text: recText };
  }
  const matchMem = recText.match(/Niskie zużycie Pamięci.*?Rozważ zmniejszenie żądań do (\d+(Mi|Gi))/i);
  if (matchMem && matchMem[1]) {
    return { type: 'apply', resource: 'memoryRequests', value: matchMem[1], text: recText };
  }
  return { type: 'info', text: recText };
}