package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math" // Upewnij się, że ten import jest
	"net/http"
	"os"
	"path/filepath"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
)

// Struktura DeploymentInfo (bez zmian)
type DeploymentInfo struct {
	Name            string   `json:"name"`
	Namespace       string   `json:"namespace"`
	CpuRequests     string   `json:"cpuRequests"`
	CpuLimits       string   `json:"cpuLimits"`
	MemoryRequests  string   `json:"memoryRequests"`
	MemoryLimits    string   `json:"memoryLimits"`
	CurrentCpuUsage int64    `json:"currentCpuUsage"`
	CurrentMemUsage int64    `json:"currentMemoryUsage"`
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
var metricsClientset *metricsclientset.Clientset
var minCpuRequestMilli int64 = 50
var minMemRequestBytes int64 = 64 * 1024 * 1024 // 64Mi

func main() {
	// Inicjalizacja i rejestracja endpointów (bez zmian)
	kubeconfigPath := filepath.Join(os.Getenv("USERPROFILE"), ".kube", "config")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		log.Fatalf("Błąd budowania konfiguracji kubeconfig: %s", err.Error())
	}
	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Błąd tworzenia clientset: %s", err.Error())
	}
	metricsClientset, err = metricsclientset.NewForConfig(config)
	if err != nil {
		log.Fatalf("Błąd tworzenia metrics clientset: %s", err.Error())
	}

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

	fmt.Println("Starting server on port 8080...")
	fmt.Println("Backend połączony z Kubernetesem. Dostępne endpointy:")
	fmt.Println("GET http://localhost:8080/api/health")
	fmt.Println("GET http://localhost:8080/api/deployments")
	fmt.Println("PATCH http://localhost:8080/api/deployments/{namespace}/{name}/resources")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

// deploymentsHandler - Zmodyfikowano tekst rekomendacji dla Pamięci
func deploymentsHandler(w http.ResponseWriter, _ *http.Request) {
	deployments, err := clientset.AppsV1().Deployments("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		http.Error(w, fmt.Sprintf("Błąd pobierania wdrożeń: %v", err), http.StatusInternalServerError)
		return
	}

	var deploymentInfos []DeploymentInfo

	for _, deployment := range deployments.Items {
		var cpuReqTotal, cpuLimTotal, memReqTotal, memLimTotal resource.Quantity
		var currentCpuUsage int64
		var currentMemUsage int64
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

		// Pobieranie metryk (bez zmian)
		selector := labels.Set(deployment.Spec.Selector.MatchLabels).String()
		pods, err := clientset.CoreV1().Pods(deployment.Namespace).List(context.Background(), metav1.ListOptions{LabelSelector: selector})
		if err != nil {
			log.Printf("Błąd pobierania Podów dla %s/%s: %v", deployment.Namespace, deployment.Name, err)
		} else {
			for _, pod := range pods.Items {
				if pod.Status.Phase != corev1.PodRunning {
					continue
				}
				podMetrics, err := metricsClientset.MetricsV1beta1().PodMetricses(pod.Namespace).Get(context.Background(), pod.Name, metav1.GetOptions{})
				if err != nil {
					log.Printf("Błąd pobierania metryk dla Poda %s/%s: %v", pod.Namespace, pod.Name, err)
					continue
				}
				for _, containerMetrics := range podMetrics.Containers {
					currentCpuUsage += containerMetrics.Usage.Cpu().MilliValue()
					currentMemUsage += containerMetrics.Usage.Memory().Value()
				}
			}
		}

		// Logika Rekomendacji (Zmodyfikowany tekst dla Pamięci)
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

		// Rekomendacja dla CPU request (bez zmian)
		if hasCpuReq && currentCpuUsage > 0 && cpuReqMilli > 0 && float64(currentCpuUsage) < 0.3*float64(cpuReqMilli) && cpuReqMilli > minCpuRequestMilli {
			suggestedCpuMilli := int64(math.Max(float64(minCpuRequestMilli), math.Ceil(float64(currentCpuUsage)*1.5/10.0)*10.0))
			suggestedCpuString := fmt.Sprintf("%dm", suggestedCpuMilli)
			recommendations = append(recommendations, fmt.Sprintf("Info: Niskie zużycie CPU (%dm - %.0f%% żądanych %s). Rozważ zmniejszenie żądań do %s.", currentCpuUsage, (float64(currentCpuUsage)/float64(cpuReqMilli))*100, cpuReqTotal.String(), suggestedCpuString))
		}

		// Rekomendacja dla Pamięci request (ZMIANA TUTAJ)
		if hasMemReq && currentMemUsage > 0 && memReqBytes > 0 && float64(currentMemUsage) < 0.3*float64(memReqBytes) && memReqBytes > minMemRequestBytes {
			// Oblicz sugerowaną wartość: 150% zużycia, zaokrąglone do najbliższego MiB, nie mniej niż minimum
			suggestedMemBytes := int64(math.Max(float64(minMemRequestBytes), float64(currentMemUsage)*1.5))
			suggestedMemMiB := int64(math.Ceil(float64(suggestedMemBytes) / (1024 * 1024))) // Zaokrąglij w górę do najbliższego MiB
			suggestedMemString := fmt.Sprintf("%dMi", suggestedMemMiB)                      // Formatuj jako "XMi"
			recommendations = append(recommendations, fmt.Sprintf("Info: Niskie zużycie Pamięci (%s - %.0f%% żądanej %s). Rozważ zmniejszenie żądań do %s.", formatBytesTrim(currentMemUsage), (float64(currentMemUsage)/float64(memReqBytes))*100, memReqTotal.String(), suggestedMemString))
		}

		// Rekomendacje dla wysokiego zużycia (bez zmian)
		if hasCpuLim && cpuLimMilli > 0 && float64(currentCpuUsage) > 0.9*float64(cpuLimMilli) {
			recommendations = append(recommendations, fmt.Sprintf("Ostrzeżenie: Wysokie zużycie CPU (%dm - %.0f%% limitu %s)! Może wystąpić throttling.", currentCpuUsage, (float64(currentCpuUsage)/float64(cpuLimMilli))*100, cpuLimTotal.String()))
		}
		if hasMemLim && memLimBytes > 0 {
			usageRatio := float64(currentMemUsage) / float64(memLimBytes)
			if usageRatio > 0.9 {
				recommendations = append(recommendations, fmt.Sprintf("Krytyczne: Wysokie zużycie Pamięci (%s - %.0f%% limitu %s)! Ryzyko OOMKilled!", formatBytesTrim(currentMemUsage), usageRatio*100, memLimTotal.String()))
			} else if usageRatio > 0.8 {
				recommendations = append(recommendations, fmt.Sprintf("Ostrzeżenie: Zużycie Pamięci (%s - %.0f%% limitu %s) jest wysokie.", formatBytesTrim(currentMemUsage), usageRatio*100, memLimTotal.String()))
			}
		}

		deploymentInfos = append(deploymentInfos, DeploymentInfo{Name: deployment.Name, Namespace: deployment.Namespace, CpuRequests: cpuReqTotal.String(), CpuLimits: cpuLimTotal.String(), MemoryRequests: memReqTotal.String(), MemoryLimits: memLimTotal.String(), CurrentCpuUsage: currentCpuUsage, CurrentMemUsage: currentMemUsage, Recommendations: recommendations})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(deploymentInfos); err != nil {
		http.Error(w, fmt.Sprintf("Błąd kodowania JSON: %v", err), http.StatusInternalServerError)
	}
}

// updateDeploymentResourcesHandler (bez zmian)
func updateDeploymentResourcesHandler(w http.ResponseWriter, r *http.Request) {
	// ... (cały kod tej funkcji pozostaje taki sam jak w poprzedniej odpowiedzi) ...
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

// formatBytesTrim (bez zmian)
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
