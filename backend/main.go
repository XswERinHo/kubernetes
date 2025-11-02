package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	// Importy K8s
	// <--- DODANY IMPORT
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	// Importy Prometheusa
	"github.com/prometheus/client_golang/api"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

// WorkloadInfo - ujednolicona struktura dla wszystkich typów zasobów
type WorkloadInfo struct {
	Name            string   `json:"name"`
	Namespace       string   `json:"namespace"`
	Kind            string   `json:"kind"`
	CpuRequests     string   `json:"cpuRequests"`
	CpuLimits       string   `json:"cpuLimits"`
	MemoryRequests  string   `json:"memoryRequests"`
	MemoryLimits    string   `json:"memoryLimits"`
	AvgCpuUsage     int64    `json:"avgCpuUsage"`
	AvgMemoryUsage  int64    `json:"avgMemoryUsage"`
	Recommendations []string `json:"recommendations"`
	// --- NOWE POLA FINOPS ---
	RequestCost float64 `json:"requestCost"`
	UsageCost   float64 `json:"usageCost"`
}

// ResourceUpdateRequest - struktura do aktualizacji zasobów
type ResourceUpdateRequest struct {
	CpuRequests    *string `json:"cpuRequests,omitempty"`
	CpuLimits      *string `json:"cpuLimits,omitempty"`
	MemoryRequests *string `json:"memoryRequests,omitempty"`
	MemoryLimits   *string `json:"memoryLimits,omitempty"`
}

// Struktura dla wykresów
type MetricPoint struct {
	Timestamp int64   `json:"timestamp"`
	Value     float64 `json:"value"`
}
type MetricHistory struct {
	CpuUsage    []MetricPoint `json:"cpuUsage"`
	MemoryUsage []MetricPoint `json:"memoryUsage"`
}

// Globalne zmienne i progi
var clientset *kubernetes.Clientset
var promAPI prometheusv1.API
var minCpuRequestMilli int64 = 50
var minMemRequestBytes int64 = 64 * 1024 * 1024 // 64Mi

// --- NOWE ZMIENNE FINOPS (CENNIK BAZOWY W PLN) ---
var costPerCpuCorePerMonth float64 = 80.0 // Przykładowo 80 PLN / vCPU / miesiąc
var costPerGbRamPerMonth float64 = 40.0   // Przykładowo 40 PLN / GB RAM / miesiąc
// --- KONIEC ZMIENNYCH FINOPS ---

// Szablony zapytań PromQL (AVG bez zmian)
const avgCpuQueryTemplate = `sum(rate(container_cpu_usage_seconds_total%s[5m])) * 1000`
const avgMemQueryTemplate = `sum(container_memory_working_set_bytes%s)`

// --- POPRAWIONE ZAPYTANIA P95 (NAPRAWIA BŁĄD 'subquery') ---
const p95CpuQueryTemplate = `sum(quantile_over_time(0.95, rate(container_cpu_usage_seconds_total%s[5m])[7d:5m])) * 1000`
const p95MemQueryTemplate = `sum(quantile_over_time(0.95, container_memory_working_set_bytes%s[7d:5m]))`

// --- KONIEC POPRAWKI ---

func main() {
	// Inicjalizacja K8s
	kubeconfigPath := filepath.Join(os.Getenv("USERPROFILE"), ".kube", "config")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		log.Fatalf("Błąd budowania konfiguracji kubeconfig: %s", err.Error())
	}
	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Błąd tworzenia clientset: %s", err.Error())
	}

	// Inicjalizacja Prometheusa
	promClient, err := api.NewClient(api.Config{
		Address: "http://localhost:30090",
	})
	if err != nil {
		log.Fatalf("Błąd tworzenia klienta Prometheus: %v", err)
	}
	promAPI = prometheusv1.NewAPI(promClient)

	// Rejestracja endpointów
	http.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) { fmt.Fprintf(w, "API is healthy!") })

	// Główny endpoint do pobierania wszystkich zasobów
	http.HandleFunc("/api/workloads", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			workloadsHandler(w, r)
		} else {
			http.Error(w, "Metoda niedozwolona", http.StatusMethodNotAllowed)
		}
	})

	// Endpointy szczegółowe dla konkretnego zasobu
	http.HandleFunc("/api/workloads/", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) < 6 {
			http.NotFound(w, r)
			return
		}

		namespace := parts[2]
		kind := parts[3]
		name := parts[4]
		action := parts[5]

		if r.Method == http.MethodPatch && action == "resources" {
			updateWorkloadResourcesHandler(w, r, namespace, kind, name)
			return
		}
		if r.Method == http.MethodGet && action == "metrics" {
			metricsHandler(w, r, namespace, kind, name)
			return
		}
		http.NotFound(w, r)
	})

	// Uruchomienie serwera
	fmt.Println("Starting server on port 8080...")
	fmt.Println("Backend połączony z Kubernetesem i Prometheusem (przez http://localhost:30090).")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

// Prosta funkcja budująca selektor (bez zmian)
func buildPrometheusSelector(namespace, workloadName string) string {
	return fmt.Sprintf(`{namespace="%s", pod=~"%s-.*"}`, namespace, workloadName)
}

// Funkcja pomocnicza do odczytu zasobów (bez zmian)
func getResourceTotals(spec corev1.PodSpec) (cpuReqTotal, cpuLimTotal, memReqTotal, memLimTotal resource.Quantity, hasCpuReq, hasMemReq, hasCpuLim, hasMemLim bool) {
	for _, container := range spec.Containers {
		if reqCpu, ok := container.Resources.Requests[corev1.ResourceCPU]; ok && reqCpu.Value() > 0 {
			cpuReqTotal.Add(reqCpu)
			hasCpuReq = true
		}
		if reqMem, ok := container.Resources.Requests[corev1.ResourceMemory]; ok && reqMem.Value() > 0 {
			memReqTotal.Add(reqMem)
			hasMemReq = true
		}
		if limCpu, ok := container.Resources.Limits[corev1.ResourceCPU]; ok && limCpu.Value() > 0 {
			cpuLimTotal.Add(limCpu)
			hasCpuLim = true
		}
		if limMem, ok := container.Resources.Limits[corev1.ResourceMemory]; ok && limMem.Value() > 0 {
			memLimTotal.Add(limMem)
			hasMemLim = true
		}
	}
	return
}

// Główny handler pobierający wszystkie workloadi (ZMODYFIKOWANY)
func workloadsHandler(w http.ResponseWriter, _ *http.Request) {
	var workloadInfos []WorkloadInfo
	ctx := context.Background()

	// 1. Pobierz Deployments
	deployments, err := clientset.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("Błąd pobierania Deployments: %v", err)
	} else {
		for _, item := range deployments.Items {
			wlInfo := processWorkload(
				item.Name,
				item.Namespace,
				"Deployment",
				item.Spec.Template.Spec,
			)
			workloadInfos = append(workloadInfos, wlInfo)
		}
	}

	// 2. Pobierz StatefulSets
	statefulSets, err := clientset.AppsV1().StatefulSets("").List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("Błąd pobierania StatefulSets: %v", err)
	} else {
		for _, item := range statefulSets.Items {
			wlInfo := processWorkload(
				item.Name,
				item.Namespace,
				"StatefulSet",
				item.Spec.Template.Spec,
			)
			workloadInfos = append(workloadInfos, wlInfo)
		}
	}

	// 3. Pobierz DaemonSets
	daemonSets, err := clientset.AppsV1().DaemonSets("").List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("Błąd pobierania DaemonSets: %v", err)
	} else {
		for _, item := range daemonSets.Items {
			wlInfo := processWorkload(
				item.Name,
				item.Namespace,
				"DaemonSet",
				item.Spec.Template.Spec,
			)
			workloadInfos = append(workloadInfos, wlInfo)
		}
	}

	// 4. Pobierz CronJobs
	cronJobs, err := clientset.BatchV1().CronJobs("").List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("Błąd pobierania CronJobs: %v", err)
	} else {
		for _, item := range cronJobs.Items {
			wlInfo := processWorkload(
				item.Name,
				item.Namespace,
				"CronJob", // Nowy Kind
				// CronJob ma szablon poda w innej ścieżce
				item.Spec.JobTemplate.Spec.Template.Spec,
			)
			workloadInfos = append(workloadInfos, wlInfo)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(workloadInfos); err != nil {
		http.Error(w, fmt.Sprintf("Błąd kodowania JSON: %v", err), http.StatusInternalServerError)
	}
}

// ZMODYFIKOWANA funkcja przetwarzająca pojedynczy workload (dodane koszty)
func processWorkload(name, namespace, kind string, podSpec corev1.PodSpec) WorkloadInfo {
	// 1. Pobierz zasoby (Requests/Limits)
	cpuReqTotal, cpuLimTotal, memReqTotal, memLimTotal,
		hasCpuReq, hasMemReq, hasCpuLim, hasMemLim := getResourceTotals(podSpec)

	// 2. Pobierz metryki z Prometheusa
	selectorString := buildPrometheusSelector(namespace, name)

	avgCpuQuery := fmt.Sprintf(avgCpuQueryTemplate, selectorString)
	avgCpuUsage := queryPrometheusScalar(avgCpuQuery)
	avgMemQuery := fmt.Sprintf(avgMemQueryTemplate, selectorString)
	avgMemUsage := queryPrometheusScalar(avgMemQuery)

	p95CpuQuery := fmt.Sprintf(p95CpuQueryTemplate, selectorString)
	p95CpuUsage := queryPrometheusScalar(p95CpuQuery)
	p95MemQuery := fmt.Sprintf(p95MemQueryTemplate, selectorString)
	p95MemUsage := queryPrometheusScalar(p95MemQuery)

	// --- PRZENIESIONA LOGIKA OBLICZANIA KOSZTÓW (przed rekomendacjami) ---
	cpuReqMilli := cpuReqTotal.MilliValue()
	memReqBytes := memReqTotal.Value()
	cpuLimMilli := cpuLimTotal.MilliValue()
	memLimBytes := memLimTotal.Value()

	// Konwersja na jednostki bazowe (Core i GB)
	cpuReqCores := float64(cpuReqMilli) / 1000.0
	memReqGB := float64(memReqBytes) / (1024 * 1024 * 1024)

	avgCpuCores := float64(avgCpuUsage) / 1000.0
	avgMemGB := float64(avgMemUsage) / (1024 * 1024 * 1024)

	// Obliczenie kosztów
	reqCost := (cpuReqCores * costPerCpuCorePerMonth) + (memReqGB * costPerGbRamPerMonth)
	usageCost := (avgCpuCores * costPerCpuCorePerMonth) + (avgMemGB * costPerGbRamPerMonth)
	// --- KONIEC LOGIKI KOSZTÓW ---

	// 3. Generuj rekomendacje
	var recommendations []string

	// ... (logika rekomendacji bez zmian) ...
	if !hasCpuReq || !hasMemReq {
		recommendations = append(recommendations, "Krytyczne: Brak zdefiniowanych żądań (requests) CPU lub Pamięci!")
	}
	if !hasCpuLim && !hasMemLim {
		recommendations = append(recommendations, "Ostrzeżenie: Brak zdefiniowanych limitów (limits) CPU i Pamięci!")
	} else {
		if !hasCpuLim {
			recommendations = append(recommendations, "Ostrzeżenie: Brak zdefiniowanego limitu (limits) CPU!")
		}
		if !hasMemLim {
			recommendations = append(recommendations, "Ostrzeżenie: Brak zdefiniowanego limitu (limits) Pamięci!")
		}
	}

	// *** ZMIANA: REKOMENDACJA CPU Z FINOPS ***
	if hasCpuReq && p95CpuUsage > 0 && cpuReqMilli > 0 {
		usageRatio := float64(p95CpuUsage) / float64(cpuReqMilli)
		if usageRatio < 0.3 && cpuReqMilli > minCpuRequestMilli {
			// Oblicz sugerowane CPU
			suggestedCpuMilli := int64(math.Max(float64(minCpuRequestMilli), math.Ceil(float64(p95CpuUsage)*1.5/10.0)*10.0))
			suggestedCpuString := fmt.Sprintf("%dm", suggestedCpuMilli)

			// Oblicz oszczędności
			newCpuReqCores := float64(suggestedCpuMilli) / 1000.0
			newReqCost := (newCpuReqCores * costPerCpuCorePerMonth) + (memReqGB * costPerGbRamPerMonth)
			monthlySavings := reqCost - newReqCost

			recommendationText := fmt.Sprintf("Info (7d p95): Niskie zużycie CPU (%dm - %.0f%% żądanych %s). Rozważ zmniejszenie żądań do %s (Oszczędność: %.2f zł/mc).", p95CpuUsage, usageRatio*100, cpuReqTotal.String(), suggestedCpuString, monthlySavings)
			recommendations = append(recommendations, recommendationText)
		}
	}

	// *** ZMIANA: REKOMENDACJA PAMIĘCI Z FINOPS ***
	if hasMemReq && p95MemUsage > 0 && memReqBytes > 0 {
		usageRatio := float64(p95MemUsage) / float64(memReqBytes)
		if usageRatio < 0.3 && memReqBytes > minMemRequestBytes {
			// Oblicz sugerowaną Pamięć
			suggestedMemBytes := int64(math.Max(float64(minMemRequestBytes), float64(p95MemUsage)*1.5))
			suggestedMemMiB := int64(math.Ceil(float64(suggestedMemBytes) / (1024 * 1024)))
			suggestedMemString := fmt.Sprintf("%dMi", suggestedMemMiB)

			// Oblicz oszczędności
			newMemReqGB := float64(suggestedMemBytes) / (1024 * 1024 * 1024)
			newReqCost := (cpuReqCores * costPerCpuCorePerMonth) + (newMemReqGB * costPerGbRamPerMonth)
			monthlySavings := reqCost - newReqCost

			recommendationText := fmt.Sprintf("Info (7d p95): Niskie zużycie Pamięci (%s - %.0f%% żądanej %s). Rozważ zmniejszenie żądań do %s (Oszczędność: %.2f zł/mc).", formatBytesTrim(p95MemUsage), usageRatio*100, memReqTotal.String(), suggestedMemString, monthlySavings)
			recommendations = append(recommendations, recommendationText)
		}
	}

	if hasCpuLim && cpuLimMilli > 0 && float64(avgCpuUsage) > 0.9*float64(cpuLimMilli) {
		recommendations = append(recommendations, fmt.Sprintf("Ostrzeżenie (5m avg): Średnie zużycie CPU (%dm - %.0f%% limitu %s) bliskie limitu! Może wystąpić throttling.", avgCpuUsage, (float64(avgCpuUsage)/float64(cpuLimMilli))*100, cpuLimTotal.String()))
	}
	if hasMemLim && memLimBytes > 0 {
		usageRatio := float64(avgMemUsage) / float64(memLimBytes)
		if usageRatio > 0.9 {
			recommendations = append(recommendations, fmt.Sprintf("Krytyczne (aktualne): Średnie zużycie Pamięci (%s - %.0f%% limitu %s) bliskie limitu! Ryzyko OOMKilled!", formatBytesTrim(avgMemUsage), usageRatio*100, memLimTotal.String()))
		} else if usageRatio > 0.8 {
			recommendations = append(recommendations, fmt.Sprintf("Ostrzeżenie (aktualne): Średnie zużycie Pamięci (%s - %.0f%% limitu %s) jest wysokie.", formatBytesTrim(avgMemUsage), usageRatio*100, memLimTotal.String()))
		}
	}

	// 4. Zwróć gotową strukturę
	return WorkloadInfo{
		Name:            name,
		Namespace:       namespace,
		Kind:            kind,
		CpuRequests:     cpuReqTotal.String(),
		CpuLimits:       cpuLimTotal.String(),
		MemoryRequests:  memReqTotal.String(),
		MemoryLimits:    memLimTotal.String(),
		AvgCpuUsage:     avgCpuUsage,
		AvgMemoryUsage:  avgMemUsage,
		Recommendations: recommendations,
		RequestCost:     reqCost,   // Dodane
		UsageCost:       usageCost, // Dodane
	}
}

// Funkcja do wykonywania zapytań skalarnych do Prometheusa (bez zmian)
func queryPrometheusScalar(query string) int64 {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if strings.Contains(query, "[7d:5m]") {
		ctx, cancel = context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
	}

	result, warnings, err := promAPI.Query(ctx, query, time.Now())
	if err != nil {
		log.Printf("Błąd zapytania do Prometheus (%s): %v", query, err)
		return 0
	}
	if len(warnings) > 0 {
		log.Printf("Ostrzeżenia z Prometheus: %v", warnings)
	}
	vector, ok := result.(model.Vector)
	if !ok || vector.Len() == 0 {
		return 0
	}
	value := vector[0].Value
	if math.IsNaN(float64(value)) {
		return 0
	}
	return int64(math.Round(float64(value)))
}

// Handler do pobierania metryk historycznych dla danego workloadu (ZMODYFIKOWANY)
func metricsHandler(w http.ResponseWriter, r *http.Request, namespace, kind, name string) {
	// 1. Pobierz parametr 'range' z URL
	rangeParam := r.URL.Query().Get("range")
	if rangeParam == "" {
		rangeParam = "1h" // Domyślna wartość
	}

	endTime := time.Now()
	var startTime time.Time
	var step time.Duration

	// 2. Ustaw startTime i step na podstawie parametru
	switch rangeParam {
	case "6h":
		startTime = endTime.Add(-6 * time.Hour)
		step = 5 * time.Minute // Krok co 5 minut
	case "24h":
		startTime = endTime.Add(-24 * time.Hour)
		step = 15 * time.Minute // Krok co 15 minut
	case "7d":
		startTime = endTime.Add(-7 * 24 * time.Hour)
		step = time.Hour // Krok co 1 godzinę
	case "1h":
		fallthrough // Przejdź do domyślnego
	default:
		startTime = endTime.Add(-1 * time.Hour)
		step = time.Minute // Krok co 1 minutę
	}

	// 3. Zbuduj zakres dla Prometheusa
	promRange := prometheusv1.Range{
		Start: startTime,
		End:   endTime,
		Step:  step,
	}

	selectorString := buildPrometheusSelector(namespace, name)
	cpuQuery := fmt.Sprintf(avgCpuQueryTemplate, selectorString)
	memQuery := fmt.Sprintf(avgMemQueryTemplate, selectorString)

	history := MetricHistory{
		CpuUsage:    queryPrometheusRange(cpuQuery, promRange),
		MemoryUsage: queryPrometheusRange(memQuery, promRange),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(history); err != nil {
		http.Error(w, fmt.Sprintf("Błąd kodowania JSON: %v", err), http.StatusInternalServerError)
	}
}

// Funkcja do pobierania danych historycznych (range query) (bez zmian)
func queryPrometheusRange(query string, promRange prometheusv1.Range) []MetricPoint {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, warnings, err := promAPI.QueryRange(ctx, query, promRange)
	if err != nil {
		log.Printf("Błąd zapytania (range) do Prometheus (%s): %v", query, err)
		return nil
	}
	if len(warnings) > 0 {
		log.Printf("Ostrzeżenia z Prometheus: %v", warnings)
	}

	matrix, ok := result.(model.Matrix)
	if !ok || matrix.Len() == 0 {
		return nil
	}

	points := []MetricPoint{}
	if matrix.Len() > 0 {
		stream := matrix[0]
		for _, v := range stream.Values {
			points = append(points, MetricPoint{
				Timestamp: int64(v.Timestamp),
				Value:     float64(v.Value),
			})
		}
	}
	return points
}

// Handler do aktualizacji zasobów dla danego workloadu (ZMODYFIKOWANY)
func updateWorkloadResourcesHandler(w http.ResponseWriter, r *http.Request, namespace, kind, name string) {
	var reqUpdate ResourceUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&reqUpdate); err != nil {
		http.Error(w, fmt.Sprintf("Błąd odczytu JSON: %v", err), http.StatusBadRequest)
		return
	}

	var podSpec *corev1.PodSpec
	var updateFunc func(context.Context, metav1.UpdateOptions) (interface{}, error)

	// Pobierz odpowiedni zasób i przygotuj funkcję aktualizującą
	switch kind {
	case "Deployment":
		deployment, err := clientset.AppsV1().Deployments(namespace).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			http.Error(w, "Nie znaleziono deploymentu", http.StatusNotFound)
			return
		}
		podSpec = &deployment.Spec.Template.Spec
		updateFunc = func(ctx context.Context, opts metav1.UpdateOptions) (interface{}, error) {
			return clientset.AppsV1().Deployments(namespace).Update(ctx, deployment, opts)
		}
	case "StatefulSet":
		statefulSet, err := clientset.AppsV1().StatefulSets(namespace).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			http.Error(w, "Nie znaleziono StatefulSet", http.StatusNotFound)
			return
		}
		podSpec = &statefulSet.Spec.Template.Spec
		updateFunc = func(ctx context.Context, opts metav1.UpdateOptions) (interface{}, error) {
			return clientset.AppsV1().StatefulSets(namespace).Update(ctx, statefulSet, opts)
		}
	case "DaemonSet":
		daemonSet, err := clientset.AppsV1().DaemonSets(namespace).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			http.Error(w, "Nie znaleziono DaemonSet", http.StatusNotFound)
			return
		}
		podSpec = &daemonSet.Spec.Template.Spec
		updateFunc = func(ctx context.Context, opts metav1.UpdateOptions) (interface{}, error) {
			return clientset.AppsV1().DaemonSets(namespace).Update(ctx, daemonSet, opts)
		}
	// --- POCZĄTEK NOWEGO BLOKU ---
	case "CronJob":
		cronJob, err := clientset.BatchV1().CronJobs(namespace).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			http.Error(w, "Nie znaleziono CronJob", http.StatusNotFound)
			return
		}
		podSpec = &cronJob.Spec.JobTemplate.Spec.Template.Spec
		updateFunc = func(ctx context.Context, opts metav1.UpdateOptions) (interface{}, error) {
			return clientset.BatchV1().CronJobs(namespace).Update(ctx, cronJob, opts)
		}
	// --- KONIEC NOWEGO BLOKU ---
	default:
		http.Error(w, "Nieobsługiwany typ zasobu", http.StatusBadRequest)
		return
	}

	if len(podSpec.Containers) == 0 {
		http.Error(w, "Zasób nie ma zdefiniowanych kontenerów", http.StatusInternalServerError)
		return
	}
	container := &podSpec.Containers[0]
	if container.Resources.Requests == nil {
		container.Resources.Requests = make(corev1.ResourceList)
	}
	if container.Resources.Limits == nil {
		container.Resources.Limits = make(corev1.ResourceList)
	}

	var parseErrors []string
	applyChange := func(field **string, resourceName corev1.ResourceName, list corev1.ResourceList) {
		if *field != nil {
			if **field == "" {
				delete(list, resourceName)
			} else if qty, err := resource.ParseQuantity(**field); err == nil {
				list[resourceName] = qty
			} else {
				parseErrors = append(parseErrors, fmt.Sprintf("Nieprawidłowa wartość %s: %v", resourceName, err))
			}
		}
	}
	applyChange(&reqUpdate.CpuRequests, corev1.ResourceCPU, container.Resources.Requests)
	applyChange(&reqUpdate.CpuLimits, corev1.ResourceCPU, container.Resources.Limits)
	applyChange(&reqUpdate.MemoryRequests, corev1.ResourceMemory, container.Resources.Requests)
	applyChange(&reqUpdate.MemoryLimits, corev1.ResourceMemory, container.Resources.Limits)

	if len(parseErrors) > 0 {
		http.Error(w, strings.Join(parseErrors, "; "), http.StatusBadRequest)
		return
	}

	_, err := updateFunc(context.Background(), metav1.UpdateOptions{})
	if err != nil {
		http.Error(w, fmt.Sprintf("Błąd aktualizacji zasobu: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Zasoby dla %s/%s (%s) zaktualizowane pomyślnie", namespace, name, kind)
}

// Funkcja formatująca bajty (bez zmian)
func formatBytesTrim(bytes int64, decimals ...int) string {
	if bytes == 0 {
		return "0B"
	}
	k := int64(1024)
	dec := 1
	if len(decimals) > 0 && decimals[0] >= 0 {
		dec = decimals[0]
	}
	sizes := []string{"B", "KB", "MB", "GB", "TB"}
	i := 0
	b := float64(bytes)
	for b >= float64(k) && i < len(sizes)-1 {
		b /= float64(k)
		i++
	}
	format := fmt.Sprintf("%%.%df%%s", dec)
	if b == float64(int64(b)) {
		format = "%.0f%s"
	}
	return fmt.Sprintf(format, b, sizes[i])
}
