package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log" // Potrzebne do zaokrąglania procentów
	"net/http"
	"os"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
)

// Definicja struktury bez zmian
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

var clientset *kubernetes.Clientset
var metricsClientset *metricsclientset.Clientset

// Definicja minimalnych progów dla rekomendacji zmniejszenia
var minCpuRequestMilli int64 = 50               // 50m
var minMemRequestBytes int64 = 64 * 1024 * 1024 // 64Mi

func main() {
	// ... (inicjalizacja clientset i metricsClientset bez zmian) ...
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
	http.HandleFunc("/api/deployments", deploymentsHandler)

	fmt.Println("Starting server on port 8080...")
	// ... (reszta bez zmian) ...
	fmt.Println("Backend połączony z Kubernetesem. Dostępne endpointy:")
	fmt.Println("http://localhost:8080/api/health")
	fmt.Println("http://localhost:8080/api/deployments")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

func deploymentsHandler(w http.ResponseWriter, r *http.Request) {
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

		// --- Odczyt Requests/Limits ---
		// (Logika odczytu bez zmian, tylko używa Value() > 0)
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

		// --- Pobieranie metryk ---
		// (Logika pobierania metryk bez zmian)
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

		// --- BARDZIEJ INTELIGENTNA LOGIKA REKOMENDACJI ---

		// 1. Sprawdzenie brakujących definicji (logika bez zmian)
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

		// Konwersja na wartości liczbowe dla łatwiejszych obliczeń
		cpuReqMilli := cpuReqTotal.MilliValue()
		memReqBytes := memReqTotal.Value()
		cpuLimMilli := cpuLimTotal.MilliValue()
		memLimBytes := memLimTotal.Value()

		// 2. Sprawdzenie przewymiarowania (Over-provisioning) z procentami i progiem minimalnym
		if hasCpuReq && cpuReqMilli > 0 {
			usageRatio := float64(currentCpuUsage) / float64(cpuReqMilli)
			// Rekomenduj zmniejszenie tylko jeśli zużycie < 30% ORAZ żądanie > minimum
			if currentCpuUsage > 0 && usageRatio < 0.3 && cpuReqMilli > minCpuRequestMilli {
				recommendations = append(recommendations, fmt.Sprintf("Info: Niskie zużycie CPU (%dm - %.0f%% żądanych %s). Rozważ zmniejszenie żądań.", currentCpuUsage, usageRatio*100, cpuReqTotal.String()))
			}
		}
		if hasMemReq && memReqBytes > 0 {
			usageRatio := float64(currentMemUsage) / float64(memReqBytes)
			// Rekomenduj zmniejszenie tylko jeśli zużycie < 30% ORAZ żądanie > minimum
			if currentMemUsage > 0 && usageRatio < 0.3 && memReqBytes > minMemRequestBytes {
				recommendations = append(recommendations, fmt.Sprintf("Info: Niskie zużycie Pamięci (%s - %.0f%% żądanej %s). Rozważ zmniejszenie żądań.", formatBytesTrim(currentMemUsage), usageRatio*100, memReqTotal.String()))
			}
		}

		// 3. Sprawdzenie wysokiego zużycia (Under-provisioning) w stosunku do limitów z procentami
		if hasCpuLim && cpuLimMilli > 0 {
			usageRatio := float64(currentCpuUsage) / float64(cpuLimMilli)
			// Ostrzegaj, gdy zużycie > 90% limitu
			if usageRatio > 0.9 {
				recommendations = append(recommendations, fmt.Sprintf("Ostrzeżenie: Wysokie zużycie CPU (%dm - %.0f%% limitu %s)! Może wystąpić throttling.", currentCpuUsage, usageRatio*100, cpuLimTotal.String()))
			}
		}
		if hasMemLim && memLimBytes > 0 {
			usageRatio := float64(currentMemUsage) / float64(memLimBytes)
			// Ostrzegaj (krytycznie), gdy zużycie > 90% limitu
			if usageRatio > 0.9 {
				recommendations = append(recommendations, fmt.Sprintf("Krytyczne: Wysokie zużycie Pamięci (%s - %.0f%% limitu %s)! Ryzyko OOMKilled!", formatBytesTrim(currentMemUsage), usageRatio*100, memLimTotal.String()))
			} else if usageRatio > 0.8 { // Dodatkowe ostrzeżenie dla > 80%
				recommendations = append(recommendations, fmt.Sprintf("Ostrzeżenie: Zużycie Pamięci (%s - %.0f%% limitu %s) jest wysokie.", formatBytesTrim(currentMemUsage), usageRatio*100, memLimTotal.String()))
			}
		}
		// --- KONIEC LOGIKI REKOMENDACJI ---

		deploymentInfos = append(deploymentInfos, DeploymentInfo{
			Name:            deployment.Name,
			Namespace:       deployment.Namespace,
			CpuRequests:     cpuReqTotal.String(),
			CpuLimits:       cpuLimTotal.String(),
			MemoryRequests:  memReqTotal.String(),
			MemoryLimits:    memLimTotal.String(),
			CurrentCpuUsage: currentCpuUsage,
			CurrentMemUsage: currentMemUsage,
			Recommendations: recommendations,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(deploymentInfos); err != nil {
		http.Error(w, fmt.Sprintf("Błąd kodowania JSON: %v", err), http.StatusInternalServerError)
	}
}

// Funkcja formatBytesTrim bez zmian
func formatBytesTrim(bytes int64, decimals ...int) string {
	// ... (kod funkcji bez zmian) ...
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
