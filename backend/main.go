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
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	// Importy Prometheusa
	"github.com/prometheus/client_golang/api"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

// Struktura DeploymentInfo (bez zmian)
type DeploymentInfo struct {
	Name            string   `json:"name"`
	Namespace       string   `json:"namespace"`
	CpuRequests     string   `json:"cpuRequests"`
	CpuLimits       string   `json:"cpuLimits"`
	MemoryRequests  string   `json:"memoryRequests"`
	MemoryLimits    string   `json:"memoryLimits"`
	CurrentCpuUsage int64    `json:"avgCpuUsage"`
	CurrentMemUsage int64    `json:"avgMemoryUsage"`
	Recommendations []string `json:"recommendations"`
}

// ResourceUpdateRequest (bez zmian)
type ResourceUpdateRequest struct {
	CpuRequests    *string `json:"cpuRequests,omitempty"`
	CpuLimits      *string `json:"cpuLimits,omitempty"`
	MemoryRequests *string `json:"memoryRequests,omitempty"`
	MemoryLimits   *string `json:"memoryLimits,omitempty"`
}

// Globalne zmienne i progi (bez zmian)
var clientset *kubernetes.Clientset
var promAPI prometheusv1.API
var minCpuRequestMilli int64 = 50
var minMemRequestBytes int64 = 64 * 1024 * 1024 // 64Mi

func main() {
	// Inicjalizacja K8s (bez zmian)
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

	// Rejestracja endpointów (bez zmian)
	http.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) { fmt.Fprintf(w, "API is healthy!") })
	http.HandleFunc("/api/deployments", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			deploymentsHandler(w, r)
		} else {
			http.Error(w, "Metoda niedozwolona", http.StatusMethodNotAllowed)
		}
	})
	http.HandleFunc("/api/deployments/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch && strings.HasSuffix(r.URL.Path, "/resources") {
			updateDeploymentResourcesHandler(w, r)
		} else {
			http.NotFound(w, r)
		}
	})

	// Uruchomienie serwera
	fmt.Println("Starting server on port 8080...")
	fmt.Println("Backend połączony z Kubernetesem I Prometheusem (przez http://localhost:30090).")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

// --- OSTATECZNIE POPRAWIONA FUNKCJA buildPrometheusSelector ---
// Usunięto filtr `container!=""`, ponieważ w tej konfiguracji powoduje on odfiltrowanie wszystkich wyników.
func buildPrometheusSelector(namespace, deploymentName string) string {
	return fmt.Sprintf(`{namespace="%s", pod=~"%s-.*"}`, namespace, deploymentName)
}

// deploymentsHandler - ZOSTATECZNIE POPRAWIONA WERSJA Z DZIAŁAJĄCYMI ZAPYTANIAMI
func deploymentsHandler(w http.ResponseWriter, _ *http.Request) {
	deployments, err := clientset.AppsV1().Deployments("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		http.Error(w, fmt.Sprintf("Błąd pobierania wdrożeń: %v", err), http.StatusInternalServerError)
		return
	}

	// --- OSTATECZNIE POPRAWIONE, DZIAŁAJĄCE SZABLONY ZAPYTAŃ ---
	const cpuQueryTemplate = `sum(rate(container_cpu_usage_seconds_total%s[5m])) * 1000`
	const memQueryTemplate = `sum(container_memory_working_set_bytes%s)`
	// --- KONIEC ZMIAN ---

	var deploymentInfos []DeploymentInfo

	for _, deployment := range deployments.Items {
		var cpuReqTotal, cpuLimTotal, memReqTotal, memLimTotal resource.Quantity
		var avgCpuUsage int64
		var avgMemUsage int64
		var recommendations []string
		hasCpuReq, hasMemReq, hasCpuLim, hasMemLim := false, false, false, false

		// Odczyt Requests/Limits (bez zmian)
		for _, container := range deployment.Spec.Template.Spec.Containers {
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

		// --- LOGIKA POBIERANIA METRYK ---
		selectorString := buildPrometheusSelector(deployment.Namespace, deployment.Name)
		// log.Printf("Selektor dla %s/%s: %s", deployment.Namespace, deployment.Name, selectorString) // ZAKOMENTOWANO

		// Zapytanie o CPU
		cpuQuery := fmt.Sprintf(cpuQueryTemplate, selectorString)
		avgCpuUsage = queryPrometheusScalar(cpuQuery)

		// Zapytanie o Pamięć
		memQuery := fmt.Sprintf(memQueryTemplate, selectorString)
		avgMemUsage = queryPrometheusScalar(memQuery)

		// log.Printf("Wynik końcowy dla %s/%s -> CPU: %d, Mem: %d", deployment.Namespace, deployment.Name, avgCpuUsage, avgMemUsage) // ZAKOMENTOWANO
		// --- KONIEC LOGIKI PROMETHEUSA ---

		// Logika Rekomendacji (bez zmian)
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
		cpuReqMilli := cpuReqTotal.MilliValue()
		memReqBytes := memReqTotal.Value()
		cpuLimMilli := cpuLimTotal.MilliValue()
		memLimBytes := memLimTotal.Value()
		if hasCpuReq && avgCpuUsage > 0 && cpuReqMilli > 0 {
			usageRatio := float64(avgCpuUsage) / float64(cpuReqMilli)
			if usageRatio < 0.3 && cpuReqMilli > minCpuRequestMilli {
				suggestedCpuMilli := int64(math.Max(float64(minCpuRequestMilli), math.Ceil(float64(avgCpuUsage)*1.5/10.0)*10.0))
				suggestedCpuString := fmt.Sprintf("%dm", suggestedCpuMilli)
				recommendations = append(recommendations, fmt.Sprintf("Info (5m avg): Niskie zużycie CPU (%dm - %.0f%% żądanych %s). Rozważ zmniejszenie żądań do %s.", avgCpuUsage, usageRatio*100, cpuReqTotal.String(), suggestedCpuString))
			}
		}
		if hasMemReq && avgMemUsage > 0 && memReqBytes > 0 {
			usageRatio := float64(avgMemUsage) / float64(memReqBytes)
			if usageRatio < 0.3 && memReqBytes > minMemRequestBytes {
				suggestedMemBytes := int64(math.Max(float64(minMemRequestBytes), float64(avgMemUsage)*1.5))
				suggestedMemMiB := int64(math.Ceil(float64(suggestedMemBytes) / (1024 * 1024)))
				suggestedMemString := fmt.Sprintf("%dMi", suggestedMemMiB)
				recommendations = append(recommendations, fmt.Sprintf("Info (aktualne): Niskie zużycie Pamięci (%s - %.0f%% żądanej %s). Rozważ zmniejszenie żądań do %s.", formatBytesTrim(avgMemUsage), usageRatio*100, memReqTotal.String(), suggestedMemString))
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

		deploymentInfos = append(deploymentInfos, DeploymentInfo{Name: deployment.Name, Namespace: deployment.Namespace, CpuRequests: cpuReqTotal.String(), CpuLimits: cpuLimTotal.String(), MemoryRequests: memReqTotal.String(), MemoryLimits: memLimTotal.String(), CurrentCpuUsage: avgCpuUsage, CurrentMemUsage: avgMemUsage, Recommendations: recommendations})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(deploymentInfos); err != nil {
		http.Error(w, fmt.Sprintf("Błąd kodowania JSON: %v", err), http.StatusInternalServerError)
	}
}

// queryPrometheusScalar (bez zmian)
func queryPrometheusScalar(query string) int64 {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
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
	return int64(math.Round(float64(value)))
}

// updateDeploymentResourcesHandler (pełna implementacja)
func updateDeploymentResourcesHandler(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/deployments/"), "/")
	if len(parts) != 3 || parts[2] != "resources" {
		http.Error(w, "Nieprawidłowy format URL", http.StatusBadRequest)
		return
	}
	namespace := parts[0]
	name := parts[1]

	var reqUpdate ResourceUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&reqUpdate); err != nil {
		http.Error(w, fmt.Sprintf("Błąd odczytu JSON: %v", err), http.StatusBadRequest)
		return
	}

	deployment, err := clientset.AppsV1().Deployments(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			http.Error(w, fmt.Sprintf("Wdrożenie %s/%s nie znalezione", namespace, name), http.StatusNotFound)
		} else {
			http.Error(w, fmt.Sprintf("Błąd pobierania wdrożenia: %v", err), http.StatusInternalServerError)
		}
		return
	}

	if len(deployment.Spec.Template.Spec.Containers) == 0 {
		http.Error(w, "Wdrożenie nie ma zdefiniowanych kontenerów", http.StatusInternalServerError)
		return
	}

	container := &deployment.Spec.Template.Spec.Containers[0]
	if container.Resources.Requests == nil {
		container.Resources.Requests = make(corev1.ResourceList)
	}
	if container.Resources.Limits == nil {
		container.Resources.Limits = make(corev1.ResourceList)
	}

	var parseErrors []string
	applyChange := func(field **string, resourceName corev1.ResourceName, list corev1.ResourceList) {
		if *field != nil {
			if qty, err := resource.ParseQuantity(**field); err == nil {
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

	_, err = clientset.AppsV1().Deployments(namespace).Update(context.Background(), deployment, metav1.UpdateOptions{})
	if err != nil {
		http.Error(w, fmt.Sprintf("Błąd aktualizacji wdrożenia: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Zasoby dla %s/%s zaktualizowane pomyślnie", namespace, name)
}

// formatBytesTrim (pełna implementacja)
func formatBytesTrim(bytes int64, decimals ...int) string {
	if bytes == 0 {
		return "0B"
	}
	k := int64(1024)
	dm := float64(0)
	if len(decimals) > 0 {
		dm = float64(decimals[0])
	}
	sizes := []string{"B", "KB", "MB", "GB", "TB"}
	i := 0
	b := float64(bytes)
	for b >= float64(k) && i < len(sizes)-1 {
		b /= float64(k)
		i++
	}
	format := fmt.Sprintf("%%.%df%%s", int(dm))
	if dm == 0 {
		format = "%.0f%s"
	}
	return fmt.Sprintf(format, b, sizes[i])
}
