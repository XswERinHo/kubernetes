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
	"sync"
	"time"

	// --- NOWE IMPORTY DLA OPERATORA ---

	// POPRAWIONA ŚCIEŻKA:
	monitoringClient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	// --- KONIEC NOWYCH IMPORTÓW ---

	// Importy K8s
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	// Importy Prometheusa
	"github.com/prometheus/client_golang/api"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

// --- Sekcje Cache (bez zmian) ---
type CacheItem struct {
	Data   int64
	Expiry time.Time
}

var (
	promCache  = make(map[string]CacheItem)
	cacheMutex = &sync.RWMutex{}
)

type RangeCacheItem struct {
	Data   []MetricPoint
	Expiry time.Time
}

var (
	promRangeCache  = make(map[string]RangeCacheItem)
	rangeCacheMutex = &sync.RWMutex{}
)

const (
	shortCacheTTL = 1 * time.Minute
	longCacheTTL  = 1 * time.Hour
)

// Struktury (bez zmian)
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
	RequestCost     float64  `json:"requestCost"`
	UsageCost       float64  `json:"usageCost"`
}
type ResourceUpdateRequest struct {
	CpuRequests    *string `json:"cpuRequests,omitempty"`
	CpuLimits      *string `json:"cpuLimits,omitempty"`
	MemoryRequests *string `json:"memoryRequests,omitempty"`
	MemoryLimits   *string `json:"memoryLimits,omitempty"`
}
type MetricPoint struct {
	Timestamp int64   `json:"timestamp"`
	Value     float64 `json:"value"`
}
type MetricHistory struct {
	CpuUsage    []MetricPoint `json:"cpuUsage"`
	MemoryUsage []MetricPoint `json:"memoryUsage"`
}

// Zmienne globalne (z nowymi klientami)
var clientset *kubernetes.Clientset
var promAPI prometheusv1.API
var monitoringClientset *monitoringClient.Clientset
var dynamicClient dynamic.Interface

var minCpuRequestMilli int64 = 50
var minMemRequestBytes int64 = 64 * 1024 * 1024
var costPerCpuCorePerMonth float64 = 80.0
var costPerGbRamPerMonth float64 = 40.0

// Szablony PromQL (bez zmian)
const avgCpuQueryTemplate = `sum(rate(container_cpu_usage_seconds_total%s[5m])) * 1000`
const avgMemQueryTemplate = `sum(container_memory_working_set_bytes%s)`
const p95CpuQueryTemplate = `sum(quantile_over_time(0.95, rate(container_cpu_usage_seconds_total%s[5m])[7d:5m])) * 1000`
const p95MemQueryTemplate = `sum(quantile_over_time(0.95, container_memory_working_set_bytes%s[7d:5m]))`

func main() {
	kubeconfigPath := filepath.Join(os.Getenv("USERPROFILE"), ".kube", "config")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		log.Fatalf("Błąd budowania konfiguracji kubeconfig: %s", err.Error())
	}

	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Błąd tworzenia clientset: %s", err.Error())
	}

	// Inicjalizacja nowych klientów
	monitoringClientset, err = monitoringClient.NewForConfig(config)
	if err != nil {
		log.Fatalf("Błąd tworzenia monitoring clientset: %s", err.Error())
	}
	dynamicClient, err = dynamic.NewForConfig(config)
	if err != nil {
		log.Fatalf("Błąd tworzenia dynamic clientset: %s", err.Error())
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
	http.HandleFunc("/api/workloads", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			workloadsHandler(w, r)
		} else {
			http.Error(w, "Metoda niedozwolona", http.StatusMethodNotAllowed)
		}
	})
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

	fmt.Println("Starting server on port 8080...")
	fmt.Println("Backend połączony z Kubernetesem i Prometheusem (przez http://localhost:30090).")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

// Główny handler pobierający wszystkie workloadi
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
				"CronJob",
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

// Prosta funkcja budująca selektor
func buildPrometheusSelector(namespace, workloadName string) string {
	return fmt.Sprintf(`{namespace="%s", pod=~"%s-.*"}`, namespace, workloadName)
}

// Funkcja pomocnicza do odczytu zasobów
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

// Sprawdza, czy zasób jest zarządzany przez znanego Operatora
func getOwnerCRD(ctx context.Context, namespace, kind, name string) (ownerKind string, ownerName string, isOperatorManaged bool) {
	var gvr schema.GroupVersionResource
	switch kind {
	case "Deployment":
		gvr = appsv1.SchemeGroupVersion.WithResource("deployments")
	case "StatefulSet":
		gvr = appsv1.SchemeGroupVersion.WithResource("statefulsets")
	case "DaemonSet":
		gvr = appsv1.SchemeGroupVersion.WithResource("daemonsets")
	default:
		return "", "", false
	}

	unstructuredObj, err := dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		log.Printf("Błąd dynamicznego pobierania zasobu %s/%s: %v", namespace, name, err)
		return "", "", false
	}

	owners := unstructuredObj.GetOwnerReferences()
	if len(owners) == 0 {
		return "", "", false
	}

	for _, owner := range owners {
		if strings.EqualFold(owner.APIVersion, "monitoring.coreos.com/v1") {
			if strings.EqualFold(owner.Kind, "Prometheus") || strings.EqualFold(owner.Kind, "Alertmanager") {
				log.Printf("Zasób %s/%s jest zarządzany przez Operatora! Właściciel: %s", namespace, name, owner.Name)
				return owner.Kind, owner.Name, true
			}
		}
	}

	return "", "", false
}

// Handler aktualizacji
func updateWorkloadResourcesHandler(w http.ResponseWriter, r *http.Request, namespace, kind, name string) {
	ctx := context.Background()
	var reqUpdate ResourceUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&reqUpdate); err != nil {
		http.Error(w, fmt.Sprintf("Błąd odczytu JSON: %v", err), http.StatusBadRequest)
		return
	}

	ownerKind, ownerName, isOperatorManaged := getOwnerCRD(ctx, namespace, kind, name)

	var err error
	if isOperatorManaged {
		log.Printf("Wykryto zasób zarządzany przez Operatora. Przekierowuję żądanie do %s/%s...", ownerKind, ownerName)
		err = updateOperatorResource(ctx, namespace, ownerKind, ownerName, &reqUpdate)
	} else {
		log.Printf("Wykryto zwykły zasób. Aktualizuję bezpośrednio...")
		err = updateStandardResource(ctx, namespace, kind, name, &reqUpdate)
	}

	if err != nil {
		http.Error(w, fmt.Sprintf("Błąd aktualizacji zasobu: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("Zaktualizowano zasób: %s/%s. Czyszczenie cache'a...", namespace, name)
	clearAllCaches(namespace, kind, name)

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Zasoby dla %s/%s (%s) zaktualizowane pomyśmie", namespace, name, kind)
}

// Aktualizacja zasobów zarządzanych przez Operatora
func updateOperatorResource(ctx context.Context, namespace, ownerKind, ownerName string, reqUpdate *ResourceUpdateRequest) error {

	newResources, err := parseResourceRequirements(reqUpdate)
	if err != nil {
		return err
	}

	switch ownerKind {
	case "Prometheus":
		prometheusCR, err := monitoringClientset.MonitoringV1().Prometheuses(namespace).Get(ctx, ownerName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("nie znaleziono zasobu CR 'Prometheus' %s: %v", ownerName, err)
		}
		prometheusCR.Spec.Resources = newResources

		_, err = monitoringClientset.MonitoringV1().Prometheuses(namespace).Update(ctx, prometheusCR, metav1.UpdateOptions{})
		return err

	case "Alertmanager":
		alertmanagerCR, err := monitoringClientset.MonitoringV1().Alertmanagers(namespace).Get(ctx, ownerName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("nie znaleziono zasobu CR 'Alertmanager' %s: %v", ownerName, err)
		}
		alertmanagerCR.Spec.Resources = newResources

		_, err = monitoringClientset.MonitoringV1().Alertmanagers(namespace).Update(ctx, alertmanagerCR, metav1.UpdateOptions{})
		return err
	}

	return fmt.Errorf("nieobsługiwany rodzaj zasobu operatora: %s", ownerKind)
}

// Stara logika aktualizacji (dla zwykłych zasobów)
func updateStandardResource(ctx context.Context, namespace, kind, name string, reqUpdate *ResourceUpdateRequest) error {

	var podSpec *corev1.PodSpec
	var updateFunc func(context.Context, metav1.UpdateOptions) (runtime.Object, error)

	switch kind {
	case "Deployment":
		obj, err := clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		podSpec = &obj.Spec.Template.Spec
		updateFunc = func(ctx context.Context, opts metav1.UpdateOptions) (runtime.Object, error) {
			return clientset.AppsV1().Deployments(namespace).Update(ctx, obj, opts)
		}
	case "StatefulSet":
		obj, err := clientset.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		podSpec = &obj.Spec.Template.Spec
		updateFunc = func(ctx context.Context, opts metav1.UpdateOptions) (runtime.Object, error) {
			return clientset.AppsV1().StatefulSets(namespace).Update(ctx, obj, opts)
		}
	case "DaemonSet":
		obj, err := clientset.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		podSpec = &obj.Spec.Template.Spec
		updateFunc = func(ctx context.Context, opts metav1.UpdateOptions) (runtime.Object, error) {
			return clientset.AppsV1().DaemonSets(namespace).Update(ctx, obj, opts)
		}
	case "CronJob":
		obj, err := clientset.BatchV1().CronJobs(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		podSpec = &obj.Spec.JobTemplate.Spec.Template.Spec
		updateFunc = func(ctx context.Context, opts metav1.UpdateOptions) (runtime.Object, error) {
			return clientset.BatchV1().CronJobs(namespace).Update(ctx, obj, opts)
		}
	default:
		return fmt.Errorf("nieobsługiwany typ zasobu: %s", kind)
	}

	if len(podSpec.Containers) == 0 {
		return fmt.Errorf("zasób nie ma zdefiniowanych kontenerów")
	}

	newResources, err := parseResourceRequirements(reqUpdate)
	if err != nil {
		return err
	}
	for i := range podSpec.Containers {
		podSpec.Containers[i].Resources = newResources
	}

	_, err = updateFunc(ctx, metav1.UpdateOptions{})
	return err
}

// Funkcja pomocnicza: Parsuje request na zasoby K8s
func parseResourceRequirements(reqUpdate *ResourceUpdateRequest) (corev1.ResourceRequirements, error) {
	requests := corev1.ResourceList{}
	limits := corev1.ResourceList{}
	var parseErrors []string

	parseField := func(field *string, resName corev1.ResourceName, list corev1.ResourceList) {
		if field != nil {
			if *field == "" {
				// Użytkownik chce usunąć
			} else if qty, err := resource.ParseQuantity(*field); err == nil {
				list[resName] = qty
			} else {
				parseErrors = append(parseErrors, fmt.Sprintf("Nieprawidłowa wartość %s: %v", resName, err))
			}
		}
	}

	parseField(reqUpdate.CpuRequests, corev1.ResourceCPU, requests)
	parseField(reqUpdate.CpuLimits, corev1.ResourceCPU, limits)
	parseField(reqUpdate.MemoryRequests, corev1.ResourceMemory, requests)
	parseField(reqUpdate.MemoryLimits, corev1.ResourceMemory, limits)

	if len(parseErrors) > 0 {
		// --- POPRAWKA LINTERA ---
		// Zamiast fmt.Errorf(strings.Join(...))
		return corev1.ResourceRequirements{}, fmt.Errorf("%s", strings.Join(parseErrors, "; "))
		// --- KONIEC POPRAWKI ---
	}

	return corev1.ResourceRequirements{Requests: requests, Limits: limits}, nil
}

// Funkcja pomocnicza: Czyści wszystkie cache
func clearAllCaches(namespace, kind, name string) {
	cacheMutex.Lock()
	delete(promCache, fmt.Sprintf("%s-%s-%s-avg-cpu", namespace, kind, name))
	delete(promCache, fmt.Sprintf("%s-%s-%s-avg-mem", namespace, kind, name))
	delete(promCache, fmt.Sprintf("%s-%s-%s-p95-cpu", namespace, kind, name))
	delete(promCache, fmt.Sprintf("%s-%s-%s-p95-mem", namespace, kind, name))
	cacheMutex.Unlock()

	rangeCacheMutex.Lock()
	for _, aRange := range []string{"1h", "6h", "24h", "7d"} {
		delete(promRangeCache, fmt.Sprintf("%s-%s-%s-range-cpu-%s", namespace, kind, name, aRange))
		delete(promRangeCache, fmt.Sprintf("%s-%s-%s-range-mem-%s", namespace, kind, name, aRange))
	}
	rangeCacheMutex.Unlock()
}

// Funkcja processWorkload (bez zmian)
func processWorkload(name, namespace, kind string, podSpec corev1.PodSpec) WorkloadInfo {
	cpuReqTotal, cpuLimTotal, memReqTotal, memLimTotal,
		hasCpuReq, hasMemReq, hasCpuLim, hasMemLim := getResourceTotals(podSpec)

	selectorString := buildPrometheusSelector(namespace, name) // Błąd był tutaj (funkcja brakowała)

	avgCpuKey := fmt.Sprintf("%s-%s-%s-avg-cpu", namespace, kind, name)
	avgMemKey := fmt.Sprintf("%s-%s-%s-avg-mem", namespace, kind, name)
	p95CpuKey := fmt.Sprintf("%s-%s-%s-p95-cpu", namespace, kind, name)
	p95MemKey := fmt.Sprintf("%s-%s-%s-p95-mem", namespace, kind, name)

	avgCpuQuery := fmt.Sprintf(avgCpuQueryTemplate, selectorString)
	avgMemQuery := fmt.Sprintf(avgMemQueryTemplate, selectorString)
	p95CpuQuery := fmt.Sprintf(p95CpuQueryTemplate, selectorString)
	p95MemQuery := fmt.Sprintf(p95MemQueryTemplate, selectorString)

	avgCpuUsage := queryPrometheusScalarCached(avgCpuQuery, avgCpuKey, shortCacheTTL)
	avgMemUsage := queryPrometheusScalarCached(avgMemQuery, avgMemKey, shortCacheTTL)
	p95CpuUsage := queryPrometheusScalarCached(p95CpuQuery, p95CpuKey, longCacheTTL)
	p95MemUsage := queryPrometheusScalarCached(p95MemQuery, p95MemKey, longCacheTTL)

	cpuReqMilli := cpuReqTotal.MilliValue()
	memReqBytes := memReqTotal.Value()
	cpuLimMilli := cpuLimTotal.MilliValue()
	memLimBytes := memLimTotal.Value()
	cpuReqCores := float64(cpuReqMilli) / 1000.0
	memReqGB := float64(memReqBytes) / (1024 * 1024 * 1024)
	avgCpuCores := float64(avgCpuUsage) / 1000.0
	avgMemGB := float64(avgMemUsage) / (1024 * 1024 * 1024)
	reqCost := (cpuReqCores * costPerCpuCorePerMonth) + (memReqGB * costPerGbRamPerMonth)
	usageCost := (avgCpuCores * costPerCpuCorePerMonth) + (avgMemGB * costPerGbRamPerMonth)

	var recommendations []string
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

	if hasCpuReq && p95CpuUsage > 0 && cpuReqMilli > 0 {
		usageRatio := float64(p95CpuUsage) / float64(cpuReqMilli)
		if usageRatio < 0.3 && cpuReqMilli > minCpuRequestMilli {
			suggestedCpuMilli := int64(math.Max(float64(minCpuRequestMilli), math.Ceil(float64(p95CpuUsage)*1.5/10.0)*10.0))
			suggestedCpuString := fmt.Sprintf("%dm", suggestedCpuMilli)
			newCpuReqCores := float64(suggestedCpuMilli) / 1000.0
			newReqCost := (newCpuReqCores * costPerCpuCorePerMonth) + (memReqGB * costPerGbRamPerMonth)
			monthlySavings := reqCost - newReqCost
			recommendationText := fmt.Sprintf("Info (7d p95): Niskie zużycie CPU (%dm - %.0f%% żądanych %s). Rozważ zmniejszenie żądań do %s (Oszczędność: %.2f zł/mc).", p95CpuUsage, usageRatio*100, cpuReqTotal.String(), suggestedCpuString, monthlySavings)
			recommendations = append(recommendations, recommendationText)
		}
	}

	if hasMemReq && p95MemUsage > 0 && memReqBytes > 0 {
		usageRatio := float64(p95MemUsage) / float64(memReqBytes)
		if usageRatio < 0.3 && memReqBytes > minMemRequestBytes {
			suggestedMemBytes := int64(math.Max(float64(minMemRequestBytes), float64(p95MemUsage)*1.5))
			suggestedMemMiB := int64(math.Ceil(float64(suggestedMemBytes) / (1024 * 1024)))
			suggestedMemString := fmt.Sprintf("%dMi", suggestedMemMiB)
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
		RequestCost:     reqCost,
		UsageCost:       usageCost,
	}
}

// Funkcje Cache (bez zmian)
func queryPrometheusScalarCached(query string, cacheKey string, ttl time.Duration) int64 {
	cacheMutex.RLock()
	item, found := promCache[cacheKey]
	if found && time.Now().Before(item.Expiry) {
		cacheMutex.RUnlock()
		return item.Data
	}
	cacheMutex.RUnlock()

	result, err := queryPrometheusScalar(query)
	if err != nil {
		log.Printf("BŁĄD ZAPYTANIA (nie zapisano w cache): %s", cacheKey)
		if found {
			return item.Data
		}
		return 0
	}

	cacheMutex.Lock()
	promCache[cacheKey] = CacheItem{
		Data:   result,
		Expiry: time.Now().Add(ttl),
	}
	cacheMutex.Unlock()

	return result
}

func queryPrometheusScalar(query string) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if strings.Contains(query, "[7d:5m]") {
		ctx, cancel = context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
	}

	result, warnings, err := promAPI.Query(ctx, query, time.Now())
	if err != nil {
		log.Printf("Błąd zapytania do Prometheus (%s): %v", query, err)
		return 0, err
	}
	if len(warnings) > 0 {
		log.Printf("Ostrzeżenia z Prometheus: %v", warnings)
	}
	vector, ok := result.(model.Vector)
	if !ok || vector.Len() == 0 {
		return 0, nil
	}
	value := vector[0].Value
	if math.IsNaN(float64(value)) {
		return 0, nil
	}
	return int64(math.Round(float64(value))), nil
}

// Funkcje Wykresów (bez zmian)
func metricsHandler(w http.ResponseWriter, r *http.Request, namespace, kind, name string) {
	rangeParam := r.URL.Query().Get("range")
	if rangeParam == "" {
		rangeParam = "1h"
	}
	endTime := time.Now()
	var startTime time.Time
	var step time.Duration
	var cacheTTL time.Duration

	switch rangeParam {
	case "6h":
		startTime = endTime.Add(-6 * time.Hour)
		step = 5 * time.Minute
		cacheTTL = 15 * time.Minute
	case "24h":
		startTime = endTime.Add(-24 * time.Hour)
		step = 15 * time.Minute
		cacheTTL = 30 * time.Minute
	case "7d":
		startTime = endTime.Add(-7 * 24 * time.Hour)
		step = time.Hour
		cacheTTL = longCacheTTL
	case "1h":
		fallthrough
	default:
		startTime = endTime.Add(-1 * time.Minute)
		step = time.Minute
		cacheTTL = shortCacheTTL
	}

	promRange := prometheusv1.Range{
		Start: startTime,
		End:   endTime,
		Step:  step,
	}

	selectorString := buildPrometheusSelector(namespace, name)
	cpuQuery := fmt.Sprintf(avgCpuQueryTemplate, selectorString)
	memQuery := fmt.Sprintf(avgMemQueryTemplate, selectorString)

	cpuCacheKey := fmt.Sprintf("%s-%s-%s-range-cpu-%s", namespace, kind, name, rangeParam)
	memCacheKey := fmt.Sprintf("%s-%s-%s-range-mem-%s", namespace, kind, name, rangeParam)

	cpuHistory := queryPrometheusRangeCached(cpuQuery, promRange, cpuCacheKey, cacheTTL)
	memHistory := queryPrometheusRangeCached(memQuery, promRange, memCacheKey, cacheTTL)

	history := MetricHistory{
		CpuUsage:    cpuHistory,
		MemoryUsage: memHistory,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(history); err != nil {
		http.Error(w, fmt.Sprintf("Błąd kodowania JSON: %v", err), http.StatusInternalServerError)
	}
}

func queryPrometheusRangeCached(query string, promRange prometheusv1.Range, cacheKey string, ttl time.Duration) []MetricPoint {
	rangeCacheMutex.RLock()
	item, found := promRangeCache[cacheKey]
	if found && time.Now().Before(item.Expiry) {
		rangeCacheMutex.RUnlock()
		return item.Data
	}
	rangeCacheMutex.RUnlock()

	result, err := queryPrometheusRange(query, promRange)

	if err != nil {
		log.Printf("BŁĄD ZAPYTANIA (Range) (nie zapisano w cache): %s", cacheKey)
		if found {
			return item.Data
		}
		return nil
	}

	rangeCacheMutex.Lock()
	promRangeCache[cacheKey] = RangeCacheItem{
		Data:   result,
		Expiry: time.Now().Add(ttl),
	}
	rangeCacheMutex.Unlock()

	return result
}

func queryPrometheusRange(query string, promRange prometheusv1.Range) ([]MetricPoint, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	if promRange.End.Sub(promRange.Start) > (24 * time.Hour) {
		ctx, cancel = context.WithTimeout(context.Background(), 90*time.Second)
	}
	defer cancel()

	result, warnings, err := promAPI.QueryRange(ctx, query, promRange)
	if err != nil {
		log.Printf("Błąd zapytania (range) do Prometheus (%s): %v", query, err)
		return nil, err
	}
	if len(warnings) > 0 {
		log.Printf("Ostrzeżenia z Prometheus: %v", warnings)
	}

	matrix, ok := result.(model.Matrix)
	if !ok || matrix.Len() == 0 {
		return nil, nil
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
	return points, nil
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
