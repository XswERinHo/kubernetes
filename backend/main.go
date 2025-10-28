package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	// Będziemy potrzebować do manipulacji selektorami
	// Kubernetes API Core
	corev1 "k8s.io/api/core/v1"
	// Quantity do pracy z wartościami zasobów (np. "100m", "70Mi")
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels" // Do konwersji selektorów etykiet
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	// --- NOWY IMPORT DLA METRICS ---
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
	// --- KONIEC NOWEGO IMPORTU ---
)

// --- STRUKTURA ROZBUDOWANA O AKTUALNE ZUŻYCIE ---
// Używamy int64 do przechowywania wartości w milicpu i bajtach dla łatwiejszego sortowania/porównywania
type DeploymentInfo struct {
	Name            string `json:"name"`
	Namespace       string `json:"namespace"`
	CpuRequests     string `json:"cpuRequests"`
	CpuLimits       string `json:"cpuLimits"`
	MemoryRequests  string `json:"memoryRequests"`
	MemoryLimits    string `json:"memoryLimits"`
	CurrentCpuUsage int64  `json:"currentCpuUsage"`    // w milicpu
	CurrentMemUsage int64  `json:"currentMemoryUsage"` // w bajtach
}

// Globalne zmienne dla klientów Kubernetes i Metrics
var clientset *kubernetes.Clientset
var metricsClientset *metricsclientset.Clientset // Nowy klient

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

	// --- INICJALIZACJA KLIENTA METRICS ---
	metricsClientset, err = metricsclientset.NewForConfig(config)
	if err != nil {
		log.Fatalf("Błąd tworzenia metrics clientset: %s", err.Error())
	}
	// --- KONIEC INICJALIZACJI ---

	http.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) { fmt.Fprintf(w, "API is healthy!") })
	http.HandleFunc("/api/deployments", deploymentsHandler)

	fmt.Println("Starting server on port 8080...")
	fmt.Println("Backend połączony z Kubernetesem. Dostępne endpointy:")
	fmt.Println("http://localhost:8080/api/health")
	fmt.Println("http://localhost:8080/api/deployments")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

// --- BARDZO ZMODYFIKOWANA FUNKCJA deploymentsHandler ---
func deploymentsHandler(w http.ResponseWriter, r *http.Request) {
	deployments, err := clientset.AppsV1().Deployments("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		http.Error(w, fmt.Sprintf("Błąd pobierania wdrożeń: %v", err), http.StatusInternalServerError)
		return
	}

	var deploymentInfos []DeploymentInfo

	for _, deployment := range deployments.Items {
		var cpuReqTotal, cpuLimTotal, memReqTotal, memLimTotal resource.Quantity // Używamy Quantity do sumowania
		var currentCpuUsage int64
		var currentMemUsage int64

		// --- Odczyt Requests/Limits z szablonu Poda ---
		for _, container := range deployment.Spec.Template.Spec.Containers {
			cpuReqTotal.Add(container.Resources.Requests[corev1.ResourceCPU])
			cpuLimTotal.Add(container.Resources.Limits[corev1.ResourceCPU])
			memReqTotal.Add(container.Resources.Requests[corev1.ResourceMemory])
			memLimTotal.Add(container.Resources.Limits[corev1.ResourceMemory])
		}

		// --- Pobieranie metryk dla Podów danego Deploymentu ---
		// Musimy znaleźć Pody, które należą do tego wdrożenia. Używamy etykiet (labels).
		selector := labels.Set(deployment.Spec.Selector.MatchLabels).String()
		pods, err := clientset.CoreV1().Pods(deployment.Namespace).List(context.Background(), metav1.ListOptions{LabelSelector: selector})
		if err != nil {
			log.Printf("Błąd pobierania Podów dla %s/%s: %v", deployment.Namespace, deployment.Name, err)
			// Kontynuujemy, ale zużycie będzie 0
		} else {
			// Iterujemy po znalezionych Podach
			for _, pod := range pods.Items {
				// Odpytujemy Metrics Server o metryki dla konkretnego Poda
				podMetrics, err := metricsClientset.MetricsV1beta1().PodMetricses(pod.Namespace).Get(context.Background(), pod.Name, metav1.GetOptions{})
				if err != nil {
					log.Printf("Błąd pobierania metryk dla Poda %s/%s: %v", pod.Namespace, pod.Name, err)
					continue // Przejdź do następnego Poda
				}

				// Sumujemy zużycie ze wszystkich kontenerów w Podzie
				for _, containerMetrics := range podMetrics.Containers {
					currentCpuUsage += containerMetrics.Usage.Cpu().MilliValue() // Sumujemy w milicpu
					currentMemUsage += containerMetrics.Usage.Memory().Value()   // Sumujemy w bajtach
				}
			}
		}

		// Dodajemy wdrożenie do listy
		deploymentInfos = append(deploymentInfos, DeploymentInfo{
			Name:            deployment.Name,
			Namespace:       deployment.Namespace,
			CpuRequests:     cpuReqTotal.String(),
			CpuLimits:       cpuLimTotal.String(),
			MemoryRequests:  memReqTotal.String(),
			MemoryLimits:    memLimTotal.String(),
			CurrentCpuUsage: currentCpuUsage, // Dodajemy zużycie
			CurrentMemUsage: currentMemUsage, // Dodajemy zużycie
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(deploymentInfos); err != nil {
		http.Error(w, fmt.Sprintf("Błąd kodowania JSON: %v", err), http.StatusInternalServerError)
	}
}
