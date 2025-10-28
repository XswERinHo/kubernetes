package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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

// --- STRUKTURA ROZBUDOWANA O REKOMENDACJE ---
type DeploymentInfo struct {
	Name            string   `json:"name"`
	Namespace       string   `json:"namespace"`
	CpuRequests     string   `json:"cpuRequests"`
	CpuLimits       string   `json:"cpuLimits"`
	MemoryRequests  string   `json:"memoryRequests"`
	MemoryLimits    string   `json:"memoryLimits"`
	CurrentCpuUsage int64    `json:"currentCpuUsage"`    // w milicpu
	CurrentMemUsage int64    `json:"currentMemoryUsage"` // w bajtach
	Recommendations []string `json:"recommendations"`    // NOWE POLE: Lista stringów z rekomendacjami
}

var clientset *kubernetes.Clientset
var metricsClientset *metricsclientset.Clientset

func main() {
	// ... (inicjalizacja clientset i metricsClientset bez zmian)
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
	// ... (reszta bez zmian)
	fmt.Println("Backend połączony z Kubernetesem. Dostępne endpointy:")
	fmt.Println("http://localhost:8080/api/health")
	fmt.Println("http://localhost:8080/api/deployments")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

// --- deploymentsHandler Z DODANĄ LOGIKĄ REKOMENDACJI ---
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
		var recommendations []string // Lista na rekomendacje dla tego wdrożenia

		// --- Odczyt Requests/Limits ---
		hasCpuReq, hasMemReq, hasCpuLim, hasMemLim := false, false, false, false
		for _, container := range deployment.Spec.Template.Spec.Containers {
			if reqCpu, ok := container.Resources.Requests[corev1.ResourceCPU]; ok && !reqCpu.IsZero() {
				cpuReqTotal.Add(reqCpu)
				hasCpuReq = true
			}
			if reqMem, ok := container.Resources.Requests[corev1.ResourceMemory]; ok && !reqMem.IsZero() {
				memReqTotal.Add(reqMem)
				hasMemReq = true
			}
			if limCpu, ok := container.Resources.Limits[corev1.ResourceCPU]; ok && !limCpu.IsZero() {
				cpuLimTotal.Add(limCpu)
				hasCpuLim = true
			}
			if limMem, ok := container.Resources.Limits[corev1.ResourceMemory]; ok && !limMem.IsZero() {
				memLimTotal.Add(limMem)
				hasMemLim = true
			}
		}

		// --- Pobieranie metryk ---
		// ... (logika pobierania metryk bez zmian) ...
		selector := labels.Set(deployment.Spec.Selector.MatchLabels).String()
		pods, err := clientset.CoreV1().Pods(deployment.Namespace).List(context.Background(), metav1.ListOptions{LabelSelector: selector})
		if err != nil {
			log.Printf("Błąd pobierania Podów dla %s/%s: %v", deployment.Namespace, deployment.Name, err)
		} else {
			for _, pod := range pods.Items {
				// Sprawdzamy, czy Pod jest Running, bo tylko takie mają sensowne metryki
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

		// --- NOWA LOGIKA GENEROWANIA REKOMENDACJI ---
		// 1. Sprawdzenie brakujących definicji
		if !hasCpuReq || !hasMemReq {
			recommendations = append(recommendations, "Krytyczne: Brak zdefiniowanych żądań (requests) CPU lub Pamięci!")
		}
		if !hasCpuLim || !hasMemLim {
			recommendations = append(recommendations, "Ostrzeżenie: Brak zdefiniowanych limitów (limits) CPU lub Pamięci!")
		}

		// 2. Proste sprawdzenie przewymiarowania CPU (jeśli są żądania i mamy dane o zużyciu)
		// Sprawdzamy, czy zużycie CPU jest mniejsze niż 10% żądanych
		if hasCpuReq && currentCpuUsage > 0 && float64(currentCpuUsage) < 0.1*float64(cpuReqTotal.MilliValue()) {
			recommendations = append(recommendations, fmt.Sprintf("Info: Niskie zużycie CPU (%dm) w porównaniu do żądanych (%s). Rozważ zmniejszenie żądań.", currentCpuUsage, cpuReqTotal.String()))
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
			Recommendations: recommendations, // Dodajemy listę rekomendacji
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(deploymentInfos); err != nil {
		http.Error(w, fmt.Sprintf("Błąd kodowania JSON: %v", err), http.StatusInternalServerError)
	}
}
