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

	"github.com/golang-jwt/jwt/v5"
	monitoringClient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

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

// --- ZMODYFIKOWANA STRUKTURA WĘZŁA ---
type NodeInfo struct {
	Name                   string            `json:"name"`
	Status                 string            `json:"status"`
	CpuCapacity            string            `json:"cpuCapacity"`
	MemoryCapacity         string            `json:"memoryCapacity"`
	CpuAllocatable         string            `json:"cpuAllocatable"`
	MemoryAllocatable      string            `json:"memoryAllocatable"`
	CpuUsage               int64             `json:"cpuUsage"`
	MemoryUsage            int64             `json:"memoryUsage"`
	CpuAllocatableMilli    int64             `json:"cpuAllocatableMilli"`
	MemoryAllocatableBytes int64             `json:"memoryAllocatableBytes"`
	PodCount               int               `json:"podCount"` // <-- NOWE
	Labels                 map[string]string `json:"labels"`   // <-- NOWE
	Taints                 []string          `json:"taints"`   // <-- NOWE
}

// --- NOWA STRUKTURA DLA LISTY PODÓW ---
type PodInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Status    string `json:"status"`
	Reason    string `json:"reason,omitempty"`
}

// ---------------------------------

type SystemHealth struct {
	KubernetesStatus string `json:"kubernetesStatus"`
	PrometheusStatus string `json:"prometheusStatus"`
	ErrorMessage     string `json:"errorMessage,omitempty"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Claims struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

type contextKey string

const userRoleContextKey contextKey = "userRole"

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

type ClusterClients struct {
	Clientset           *kubernetes.Clientset
	MonitoringClientset *monitoringClient.Clientset
	DynamicClient       dynamic.Interface
}

var (
	clusterClients map[string]*ClusterClients
	promAPI        prometheusv1.API
)

var jwtSecretKey = []byte("super-secret-key-that-should-be-in-env")

var minCpuRequestMilli int64 = 50
var minMemRequestBytes int64 = 64 * 1024 * 1024
var costPerCpuCorePerMonth float64 = 80.0
var costPerGbRamPerMonth float64 = 40.0

// Zapytania PromQL (bez zmian)
const avgCpuQueryTemplate = `sum(rate(container_cpu_usage_seconds_total%s[5m])) * 1000`
const avgMemQueryTemplate = `sum(container_memory_working_set_bytes%s)`
const p95CpuQueryTemplate = `sum(quantile_over_time(0.95, rate(container_cpu_usage_seconds_total%s[5m])[7d:5m])) * 1000`
const p95MemQueryTemplate = `sum(quantile_over_time(0.95, container_memory_working_set_bytes%s[7d:5m]))`
const nodeCpuUsageQuery = `sum by (node) (rate(container_cpu_usage_seconds_total{image!=""}[5m])) * 1000`
const nodeMemUsageQuery = `sum by (node) (container_memory_working_set_bytes{image!=""})`

func main() {
	kubeconfigPath := filepath.Join(os.Getenv("USERPROFILE"), ".kube", "config")
	if _, err := os.Stat(kubeconfigPath); os.IsNotExist(err) {
		log.Fatalf("Plik kubeconfig nie został znaleziony w: %s", kubeconfigPath)
	}

	config, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		log.Fatalf("Błąd ładowania pliku kubeconfig: %s", err.Error())
	}

	clusterClients = make(map[string]*ClusterClients)

	for contextName := range config.Contexts {
		log.Printf("Ładowanie kontekstu klastra: %s", contextName)

		restConfig, err := clientcmd.NewDefaultClientConfig(*config, &clientcmd.ConfigOverrides{CurrentContext: contextName}).ClientConfig()
		if err != nil {
			log.Printf("OSTRZEŻENIE: Nie można załadować konfiguracji dla kontekstu '%s': %v. Pomijanie.", contextName, err)
			continue
		}

		clientset, err := kubernetes.NewForConfig(restConfig)
		if err != nil {
			log.Printf("OSTRZEŻENIE: Nie można utworzyć clientset dla '%s': %v. Pomijanie.", contextName, err)
			continue
		}

		monitoringClientset, err := monitoringClient.NewForConfig(restConfig)
		if err != nil {
			log.Printf("OSTRZEŻENIE: Nie można utworzyć monitoringClientset dla '%s': %v. Pomijanie.", contextName, err)
			continue
		}

		dynamicClient, err := dynamic.NewForConfig(restConfig)
		if err != nil {
			log.Printf("OSTRZEŻENIE: Nie można utworzyć dynamicClient dla '%s': %v. Pomijanie.", contextName, err)
			continue
		}

		clusterClients[contextName] = &ClusterClients{
			Clientset:           clientset,
			MonitoringClientset: monitoringClientset,
			DynamicClient:       dynamicClient,
		}
	}

	if len(clusterClients) == 0 {
		log.Fatalf("Nie załadowano żadnych poprawnych kontekstów klastra z pliku kubeconfig.")
	}
	log.Printf("Załadowano pomyślnie %d kontekstów klastra.", len(clusterClients))

	promClient, err := api.NewClient(api.Config{
		Address: "http://localhost:30090",
	})
	if err != nil {
		log.Fatalf("Błąd tworzenia klienta Prometheus: %v", err)
	}
	promAPI = prometheusv1.NewAPI(promClient)

	http.HandleFunc("/api/auth/login", loginHandler)
	http.HandleFunc("/api/clusters", jwtAuthenticationMiddleware(clustersHandler))
	http.HandleFunc("/api/clusters/", jwtAuthenticationMiddleware(clusterApiHandler))

	fmt.Println("Starting server on port 8080...")
	fmt.Println("Backend połączony z Kubernetesem i Prometheusem (przez http://localhost:30090).")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Metoda niedozwolona", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Nieprawidłowy format JSON", http.StatusBadRequest)
		return
	}

	var userRole string
	if req.Username == "admin" && req.Password == "password123" {
		userRole = "Admin"
	} else if req.Username == "editor" && req.Password == "password123" {
		userRole = "Editor"
	} else if req.Username == "viewer" && req.Password == "password123" {
		userRole = "Viewer"
	} else {
		http.Error(w, "Nieprawidłowa nazwa użytkownika lub hasło", http.StatusUnauthorized)
		return
	}

	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		Username: req.Username,
		Role:     userRole,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtSecretKey)
	if err != nil {
		http.Error(w, "Nie można wygenerować tokenu", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"token": tokenString,
		"role":  userRole,
	})
}

func jwtAuthenticationMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Brak nagłówka autoryzacji", http.StatusUnauthorized)
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			http.Error(w, "Nieprawidłowy format tokenu", http.StatusUnauthorized)
			return
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return jwtSecretKey, nil
		})

		if err != nil {
			if err == jwt.ErrSignatureInvalid {
				http.Error(w, "Nieprawidłowy podpis tokenu", http.StatusUnauthorized)
				return
			}
			http.Error(w, "Nieprawidłowy token", http.StatusBadRequest)
			return
		}
		if !token.Valid {
			http.Error(w, "Token jest nieważny", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), userRoleContextKey, claims.Role)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

func clustersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Metoda niedozwolona", http.StatusMethodNotAllowed)
		return
	}

	var clusterNames []string
	for name := range clusterClients {
		clusterNames = append(clusterNames, name)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(clusterNames); err != nil {
		http.Error(w, fmt.Sprintf("Błąd kodowania JSON: %v", err), http.StatusInternalServerError)
	}
}

// --- ZMODYFIKOWANY ROUTER API KLASTRA ---
func clusterApiHandler(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 {
		http.NotFound(w, r)
		return
	}

	clusterName := parts[2]
	clients, ok := clusterClients[clusterName]
	if !ok {
		http.Error(w, fmt.Sprintf("Nie znaleziono klastra o nazwie: %s", clusterName), http.StatusNotFound)
		return
	}

	actionPath := "/" + strings.Join(parts[3:], "/")

	role, ok := r.Context().Value(userRoleContextKey).(string)
	if !ok {
		role = "Viewer"
		log.Println("OSTRZEŻENIE: Nie można odczytać roli użytkownika z kontekstu.")
	}

	if strings.HasPrefix(actionPath, "/health") {
		systemHealthHandler(w, r, clients)
		return
	}

	// --- ZMODYFIKOWANY ROUTING DLA WĘZŁÓW ---
	if strings.HasPrefix(actionPath, "/nodes") {
		// Ścieżka: /api/clusters/{clusterName}/nodes/{nodeName}/pods
		// parts: [api, clusters, clusterName, nodes, nodeName, pods] (len 6)
		if len(parts) == 6 && parts[3] == "nodes" && parts[5] == "pods" {
			nodeName := parts[4]
			nodePodsHandler(w, r, clients, nodeName)
			return
		}

		// Ścieżka: /api/clusters/{clusterName}/nodes
		if actionPath == "/nodes" && r.Method == http.MethodGet {
			nodesHandler(w, r, clients)
			return
		}
	}
	// ------------------------------------

	if strings.HasPrefix(actionPath, "/workloads") {
		if actionPath == "/workloads" && r.Method == http.MethodGet {
			workloadsHandler(w, r, clients)
			return
		}

		if strings.HasSuffix(actionPath, "/resources") && r.Method == http.MethodPatch {
			if role != "Admin" && role != "Editor" {
				http.Error(w, "Brak uprawnień. Wymagana rola 'Admin' lub 'Editor'.", http.StatusForbidden)
				return
			}
			resourceParts := strings.Split(strings.Trim(actionPath, "/"), "/")
			if len(resourceParts) == 5 {
				namespace := resourceParts[1]
				kind := resourceParts[2]
				name := resourceParts[3]
				updateWorkloadResourcesHandler(w, r, clients, namespace, kind, name)
				return
			}
		}

		if strings.HasSuffix(actionPath, "/metrics") && r.Method == http.MethodGet {
			metricParts := strings.Split(strings.Trim(actionPath, "/"), "/")
			if len(metricParts) == 5 {
				namespace := metricParts[1]
				kind := metricParts[2]
				name := metricParts[3]
				metricsHandler(w, r, clients, namespace, kind, name)
				return
			}
		}
	}

	http.NotFound(w, r)
}

// --- NOWA FUNKCJA POMOCNICZA: Pobieranie statusu Poda ---
func getPodStatus(pod corev1.Pod) (string, string) {
	if pod.Status.Phase == corev1.PodPending {
		// Sprawdzanie, czy kontener czeka
		if len(pod.Status.ContainerStatuses) > 0 {
			if pod.Status.ContainerStatuses[0].State.Waiting != nil {
				return string(pod.Status.Phase), pod.Status.ContainerStatuses[0].State.Waiting.Reason
			}
		}
		return string(pod.Status.Phase), "Initializing"
	}

	if pod.Status.Phase == corev1.PodFailed {
		return string(pod.Status.Phase), pod.Status.Reason
	}

	if pod.Status.Phase == corev1.PodRunning {
		// Sprawdzanie, czy kontenery są gotowe
		allReady := true
		for _, cs := range pod.Status.ContainerStatuses {
			if !cs.Ready {
				allReady = false
				// Sprawdzanie, czy któryś jest w CrashLoopBackOff
				if cs.State.Waiting != nil && cs.State.Waiting.Reason == "CrashLoopBackOff" {
					return "CrashLoopBackOff", cs.LastTerminationState.Terminated.Reason
				}
				// Sprawdzanie, czy któryś jest w Terminated
				if cs.State.Terminated != nil {
					return "Terminated", cs.State.Terminated.Reason
				}
			}
		}
		if allReady {
			return string(pod.Status.Phase), ""
		}
		return "Running", "NotReady" // Działa, ale nie wszystkie kontenery są 'Ready'
	}

	return string(pod.Status.Phase), pod.Status.Reason
}

// ----------------------------------------------------

// --- NOWY HANDLER DLA LISTY PODÓW NA WĘŹLE ---
func nodePodsHandler(w http.ResponseWriter, r *http.Request, clients *ClusterClients, nodeName string) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	var podsInfo []PodInfo

	// Pobierz pody używając fieldSelector
	fieldSelector := fmt.Sprintf("spec.nodeName=%s", nodeName)
	pods, err := clients.Clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{FieldSelector: fieldSelector})
	if err != nil {
		http.Error(w, fmt.Sprintf("Błąd pobierania podów dla węzła %s: %v", nodeName, err), http.StatusInternalServerError)
		return
	}

	for _, pod := range pods.Items {
		status, reason := getPodStatus(pod)
		info := PodInfo{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Status:    status,
			Reason:    reason,
		}
		podsInfo = append(podsInfo, info)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(podsInfo); err != nil {
		http.Error(w, fmt.Sprintf("Błąd kodowania JSON: %v", err), http.StatusInternalServerError)
	}
}

// --------------------------------------------

// --- ZMODYFIKOWANY HANDLER DLA WĘZŁÓW ---
func nodesHandler(w http.ResponseWriter, r *http.Request, clients *ClusterClients) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	var nodesInfo []NodeInfo

	cpuUsageMap, err := queryPrometheusVectorAsMap(ctx, nodeCpuUsageQuery)
	if err != nil {
		log.Printf("Błąd pobierania metryk CPU węzłów: %v", err)
	}
	memUsageMap, err := queryPrometheusVectorAsMap(ctx, nodeMemUsageQuery)
	if err != nil {
		log.Printf("Błąd pobierania metryk Pamięci węzłów: %v", err)
	}

	nodes, err := clients.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		http.Error(w, fmt.Sprintf("Błąd pobierania węzłów: %v", err), http.StatusInternalServerError)
		return
	}

	for _, node := range nodes.Items {
		status := "NotReady"
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
				status = "Ready"
				break
			}
		}

		cpuCap := node.Status.Capacity[corev1.ResourceCPU]
		memCap := node.Status.Capacity[corev1.ResourceMemory]
		cpuAlloc := node.Status.Allocatable[corev1.ResourceCPU]
		memAlloc := node.Status.Allocatable[corev1.ResourceMemory]

		// --- NOWA LOGIKA: Zliczanie podów dla węzła ---
		fieldSelector := fmt.Sprintf("spec.nodeName=%s", node.Name)
		pods, err := clients.Clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{FieldSelector: fieldSelector})
		podCount := 0
		if err != nil {
			log.Printf("Błąd pobierania podów dla węzła %s: %v", node.Name, err)
		} else {
			podCount = len(pods.Items)
		}
		// ----------------------------------------

		// --- NOWA LOGIKA: Formatowanie Taintów ---
		var formattedTaints []string
		for _, taint := range node.Spec.Taints {
			formattedTaints = append(formattedTaints, fmt.Sprintf("%s=%s:%s", taint.Key, taint.Value, taint.Effect))
		}
		// ------------------------------------

		info := NodeInfo{
			Name:                   node.Name,
			Status:                 status,
			CpuCapacity:            cpuCap.String(),
			MemoryCapacity:         memCap.String(),
			CpuAllocatable:         cpuAlloc.String(),
			MemoryAllocatable:      memAlloc.String(),
			CpuUsage:               cpuUsageMap[node.Name],
			MemoryUsage:            memUsageMap[node.Name],
			CpuAllocatableMilli:    cpuAlloc.MilliValue(),
			MemoryAllocatableBytes: memAlloc.Value(),
			PodCount:               podCount,        // <-- NOWE
			Labels:                 node.Labels,     // <-- NOWE
			Taints:                 formattedTaints, // <-- NOWE
		}
		nodesInfo = append(nodesInfo, info)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(nodesInfo); err != nil {
		http.Error(w, fmt.Sprintf("Błąd kodowania JSON: %v", err), http.StatusInternalServerError)
	}
}

func queryPrometheusVectorAsMap(ctx context.Context, query string) (map[string]int64, error) {
	result, warnings, err := promAPI.Query(ctx, query, time.Now())
	if err != nil {
		return nil, fmt.Errorf("błąd zapytania Prometheus (%s): %v", query, err)
	}
	if len(warnings) > 0 {
		log.Printf("Ostrzeżenia z Prometheus: %v", warnings)
	}

	vector, ok := result.(model.Vector)
	if !ok {
		return nil, fmt.Errorf("oczekiwano wektora z Prometheus, otrzymano %T", result)
	}

	resultMap := make(map[string]int64)
	for _, sample := range vector {
		nodeName := string(sample.Metric["node"])
		if nodeName != "" {
			value := int64(math.Round(float64(sample.Value)))
			resultMap[nodeName] = value
		}
	}
	return resultMap, nil
}

func systemHealthHandler(w http.ResponseWriter, r *http.Request, clients *ClusterClients) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	health := SystemHealth{
		KubernetesStatus: "ok",
		PrometheusStatus: "ok",
	}
	var errorMessages []string

	_, err := clients.Clientset.Discovery().ServerVersion()
	if err != nil {
		health.KubernetesStatus = "error"
		errorMessages = append(errorMessages, fmt.Sprintf("Kubernetes API error: %v", err))
		log.Printf("Błąd połączenia z Kubernetes API: %v", err)
	}

	_, err = promAPI.Buildinfo(ctx)
	if err != nil {
		health.PrometheusStatus = "error"
		errorMessages = append(errorMessages, fmt.Sprintf("Prometheus API error: %v", err))
		log.Printf("Błąd połączenia z Prometheus: %v", err)
	}

	if len(errorMessages) > 0 {
		health.ErrorMessage = strings.Join(errorMessages, "; ")
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(health); err != nil {
		http.Error(w, fmt.Sprintf("Błąd kodowania JSON: %v", err), http.StatusInternalServerError)
	}
}

func workloadsHandler(w http.ResponseWriter, _ *http.Request, clients *ClusterClients) {
	var workloadInfos []WorkloadInfo
	ctx := context.Background()

	deployments, err := clients.Clientset.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("Błąd pobierania Deployments: %v", err)
	} else {
		for _, item := range deployments.Items {
			labelSelector := metav1.FormatLabelSelector(item.Spec.Selector)
			wlInfo := processWorkload(
				ctx,
				clients,
				item.Name,
				item.Namespace,
				"Deployment",
				item.Spec.Template.Spec,
				labelSelector,
			)
			workloadInfos = append(workloadInfos, wlInfo)
		}
	}

	statefulSets, err := clients.Clientset.AppsV1().StatefulSets("").List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("Błąd pobierania StatefulSets: %v", err)
	} else {
		for _, item := range statefulSets.Items {
			labelSelector := metav1.FormatLabelSelector(item.Spec.Selector)
			wlInfo := processWorkload(
				ctx,
				clients,
				item.Name,
				item.Namespace,
				"StatefulSet",
				item.Spec.Template.Spec,
				labelSelector,
			)
			workloadInfos = append(workloadInfos, wlInfo)
		}
	}

	daemonSets, err := clients.Clientset.AppsV1().DaemonSets("").List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("Błąd pobierania DaemonSets: %v", err)
	} else {
		for _, item := range daemonSets.Items {
			labelSelector := metav1.FormatLabelSelector(item.Spec.Selector)
			wlInfo := processWorkload(
				ctx,
				clients,
				item.Name,
				item.Namespace,
				"DaemonSet",
				item.Spec.Template.Spec,
				labelSelector,
			)
			workloadInfos = append(workloadInfos, wlInfo)
		}
	}

	cronJobs, err := clients.Clientset.BatchV1().CronJobs("").List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("Błąd pobierania CronJobs: %v", err)
	} else {
		for _, item := range cronJobs.Items {
			wlInfo := processWorkload(
				ctx,
				clients,
				item.Name,
				item.Namespace,
				"CronJob",
				item.Spec.JobTemplate.Spec.Template.Spec,
				"",
			)
			workloadInfos = append(workloadInfos, wlInfo)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(workloadInfos); err != nil {
		http.Error(w, fmt.Sprintf("Błąd kodowania JSON: %v", err), http.StatusInternalServerError)
	}
}

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

func getOwnerCRD(ctx context.Context, clients *ClusterClients, namespace, kind, name string) (ownerKind string, ownerName string, isOperatorManaged bool) {
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

	unstructuredObj, err := clients.DynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
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

func updateWorkloadResourcesHandler(w http.ResponseWriter, r *http.Request, clients *ClusterClients, namespace, kind, name string) {
	ctx := context.Background()
	var reqUpdate ResourceUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&reqUpdate); err != nil {
		http.Error(w, fmt.Sprintf("Błąd odczytu JSON: %v", err), http.StatusBadRequest)
		return
	}

	ownerKind, ownerName, isOperatorManaged := getOwnerCRD(ctx, clients, namespace, kind, name)

	var err error
	if isOperatorManaged {
		log.Printf("Wykryto zasób zarządzany przez Operatora. Przekierowuję żądanie do %s/%s...", ownerKind, ownerName)
		err = updateOperatorResource(ctx, clients, namespace, ownerKind, ownerName, &reqUpdate)
	} else {
		log.Printf("Wykryto zwykły zasób. Aktualizuję bezpośrednio...")
		err = updateStandardResource(ctx, clients, namespace, kind, name, &reqUpdate)
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

func updateOperatorResource(ctx context.Context, clients *ClusterClients, namespace, ownerKind, ownerName string, reqUpdate *ResourceUpdateRequest) error {

	newResources, err := parseResourceRequirements(reqUpdate)
	if err != nil {
		return err
	}

	monitoringClientset := clients.MonitoringClientset

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

func updateStandardResource(ctx context.Context, clients *ClusterClients, namespace, kind, name string, reqUpdate *ResourceUpdateRequest) error {

	var podSpec *corev1.PodSpec
	var updateFunc func(context.Context, metav1.UpdateOptions) (runtime.Object, error)

	clientset := clients.Clientset

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

func parseResourceRequirements(reqUpdate *ResourceUpdateRequest) (corev1.ResourceRequirements, error) {
	requests := corev1.ResourceList{}
	limits := corev1.ResourceList{}
	var parseErrors []string

	parseField := func(field *string, resName corev1.ResourceName, list corev1.ResourceList) {
		if field != nil {
			if *field == "" {
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
		return corev1.ResourceRequirements{}, fmt.Errorf("%s", strings.Join(parseErrors, "; "))
	}

	return corev1.ResourceRequirements{Requests: requests, Limits: limits}, nil
}

func clearAllCaches(namespace, kind, name string) {
	log.Println("CZYSZCZENIE CACHE: (Uwaga: cache nie jest jeszcze świadomy klastrów)")
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

func processWorkload(ctx context.Context, clients *ClusterClients, name, namespace, kind string, podSpec corev1.PodSpec, labelSelector string) WorkloadInfo {
	cpuReqTotal, cpuLimTotal, memReqTotal, memLimTotal,
		hasCpuReq, hasMemReq, hasCpuLim, hasMemLim := getResourceTotals(podSpec)

	selectorString := buildPrometheusSelector(namespace, name)

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

	oomKilledCount := 0
	if labelSelector != "" {
		pods, err := clients.Clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
		if err != nil {
			log.Printf("Błąd pobierania podów dla %s/%s: %v", namespace, name, err)
		} else {
			for _, pod := range pods.Items {
				for _, cs := range pod.Status.ContainerStatuses {
					if cs.LastTerminationState.Terminated != nil && cs.LastTerminationState.Terminated.Reason == "OOMKilled" {
						oomKilledCount++
						break
					}
				}
				for _, cs := range pod.Status.InitContainerStatuses {
					if cs.LastTerminationState.Terminated != nil && cs.LastTerminationState.Terminated.Reason == "OOMKilled" {
						oomKilledCount++
						break
					}
				}
			}
		}
	}
	if oomKilledCount > 0 {
		recommendations = append(recommendations, fmt.Sprintf("Krytyczne: Wykryto %d pody, które zostały zatrzymane z powodu OOMKilled! Rozważ natychmiastowe zwiększenie limitów pamięci.", oomKilledCount))
	}

	if kind != "CronJob" && p95CpuUsage == 0 && p95MemUsage < (10*1024*1024) {
		recommendations = append(recommendations, "Ostrzeżenie: Ten zasób (niebędący CronJobem) nie wykazuje niemal żadnego zużycia CPU i Pamięci w ciągu ostatnich 7 dni (p95). Może być kandydatem do usunięcia (zombie workload).")
	}

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
		if oomKilledCount == 0 && usageRatio > 0.9 {
			recommendations = append(recommendations, fmt.Sprintf("Krytyczne (aktualne): Średnie zużycie Pamięci (%s - %.0f%% limitu %s) bliskie limitu! Ryzyko OOMKilled!", formatBytesTrim(avgMemUsage), usageRatio*100, memLimTotal.String()))
		} else if oomKilledCount == 0 && usageRatio > 0.8 {
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

func metricsHandler(w http.ResponseWriter, r *http.Request, _ *ClusterClients, namespace, kind, name string) {
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
		startTime = endTime.Add(-1 * time.Hour)
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
