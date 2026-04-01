package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"kubernetes-manager/backend/internal/k8s"
	"kubernetes-manager/backend/internal/models"
	"kubernetes-manager/backend/internal/services"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
)

var jwtSecretKey = []byte(getJWTSecret())

func getJWTSecret() string {
	if secret := os.Getenv("JWT_SECRET"); secret != "" {
		return secret
	}
	return "change-me-in-production"
}

type API struct {
	K8s *k8s.Service
}

func New(k8sService *k8s.Service) *API {
	return &API{K8s: k8sService}
}

type contextKey string

const (
	userRoleContextKey contextKey = "userRole"
	userNameContextKey contextKey = "userName"
	tokenContextKey    contextKey = "token"
	claimsContextKey   contextKey = "claims"
)

var (
	revokedTokens = make(map[string]time.Time)
	revokedMutex  = &sync.RWMutex{}
)

func (a *API) SetupRoutes(r *mux.Router) {
	r.HandleFunc("/api/login", a.LoginHandler).Methods("POST", "OPTIONS")
	r.HandleFunc("/api/health", a.HealthHandler).Methods("GET")

	api := r.PathPrefix("/api").Subrouter()
	api.Use(a.AuthMiddleware)

	api.HandleFunc("/clusters/{cluster}/health", a.HealthHandler).Methods("GET")
	api.HandleFunc("/users", a.UsersHandler).Methods("GET", "POST", "OPTIONS")
	api.HandleFunc("/users/{username}", a.DeleteUserHandler).Methods("DELETE")
	api.HandleFunc("/clusters", a.ClustersHandler).Methods("GET")
	api.HandleFunc("/clusters/{cluster}/workloads", a.WorkloadsHandler).Methods("GET")
	api.HandleFunc("/clusters/{cluster}/nodes", a.NodesHandler).Methods("GET")
	api.HandleFunc("/clusters/{cluster}/nodes/{node}/pods", a.NodePodsHandler).Methods("GET")
	api.HandleFunc("/clusters/{cluster}/events", a.EventsHandler).Methods("GET")
	api.HandleFunc("/clusters/{cluster}/approvals", a.ApprovalsHandler).Methods("GET")
	api.HandleFunc("/approvals/{id}/approve", a.ApproveChangeHandler).Methods("POST")
	api.HandleFunc("/approvals/{id}/reject", a.RejectChangeHandler).Methods("POST")
	api.HandleFunc("/clusters/{cluster}/workloads/{namespace}/{kind}/{name}/resources", a.UpdateWorkloadResourcesHandler).Methods("PATCH")
	api.HandleFunc("/clusters/{cluster}/workloads/{namespace}/{kind}/{name}/metrics", a.WorkloadMetricsHandler).Methods("GET")
	api.HandleFunc("/clusters/{cluster}/namespaces/{namespace}/pods/{pod}/logs", a.PodLogsHandler).Methods("GET")
	api.HandleFunc("/alerts", a.AlertsHandler).Methods("GET", "POST")
}

func (a *API) LoginHandler(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	role, ok := services.AuthenticateUser(req.Username, req.Password)
	if !ok {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &models.Claims{
		Username: req.Username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtSecretKey)
	if err != nil {
		http.Error(w, "Error generating token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": tokenString, "role": role})
}

func (a *API) UsersHandler(w http.ResponseWriter, r *http.Request) {
	role := getRoleFromContext(r.Context())
	if role != "Admin" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if r.Method == http.MethodGet {
		users, err := services.ListUsers()
		if err != nil {
			http.Error(w, "Failed to list users", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(users)
	} else if r.Method == http.MethodPost {
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
			Role     string `json:"role"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		if err := services.CreateUser(req.Username, req.Password, req.Role); err != nil {
			http.Error(w, fmt.Sprintf("Failed to create user: %v", err), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}
}

func (a *API) DeleteUserHandler(w http.ResponseWriter, r *http.Request) {
	role := getRoleFromContext(r.Context())
	if role != "Admin" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	vars := mux.Vars(r)
	username := vars["username"]
	if username == "" {
		http.Error(w, "Username required", http.StatusBadRequest)
		return
	}

	if err := services.DeleteUser(username); err != nil {
		http.Error(w, "Failed to delete user", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (a *API) ClustersHandler(w http.ResponseWriter, r *http.Request) {
	var clusters []string
	for name := range a.K8s.Clients {
		clusters = append(clusters, name)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(clusters)
}

func (a *API) WorkloadsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clusterName := vars["cluster"]

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	var infos []models.WorkloadInfo
	if clusterName != "" {
		if client, ok := a.K8s.Clients[clusterName]; ok {
			infos = a.K8s.CollectFromCluster(ctx, clusterName, client)
		} else {
			// If cluster not found, return empty list or error?
			// For now empty list to avoid breaking frontend
			infos = []models.WorkloadInfo{}
		}
	} else {
		// Fallback for legacy or global call
		infos = a.K8s.CollectClusterWorkloads(ctx)
	}

	if infos == nil {
		infos = []models.WorkloadInfo{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(infos)
}

func (a *API) NodesHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clusterName := vars["cluster"]

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	nodes, err := a.K8s.GetNodeMetricsFromPrometheus(ctx, clusterName)
	if err != nil {
		log.Printf("Error fetching nodes: %v", err)
		http.Error(w, "Failed to fetch nodes", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(nodes)
}

func (a *API) NodePodsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	nodeName := vars["node"]
	clusterName := vars["cluster"]

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	pods, err := a.K8s.GetNodePods(ctx, clusterName, nodeName)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error fetching pods for node %s: %v", nodeName, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pods)
}

func (a *API) HealthHandler(w http.ResponseWriter, r *http.Request) {
	health := models.SystemHealth{
		KubernetesStatus: "ok",
		PrometheusStatus: "ok",
	}

	var errors []string
	vars := mux.Vars(r)
	clusterName := vars["cluster"]

	if clusterName != "" {
		client, ok := a.K8s.Clients[clusterName]
		if !ok {
			health.KubernetesStatus = "error"
			errors = append(errors, fmt.Sprintf("cluster '%s' not found", clusterName))
		} else {
			if _, err := client.Clientset.Discovery().ServerVersion(); err != nil {
				health.KubernetesStatus = "error"
				errors = append(errors, fmt.Sprintf("kubernetes api: %v", err))
			}
		}
	}

	if a.K8s.PromAPI == nil {
		health.PrometheusStatus = "error"
		errors = append(errors, "prometheus api not initialized")
	} else {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		if _, _, err := a.K8s.PromAPI.Query(ctx, "up", time.Now()); err != nil {
			health.PrometheusStatus = "error"
			errors = append(errors, fmt.Sprintf("prometheus api: %v", err))
		}
	}

	if len(errors) > 0 {
		health.ErrorMessage = strings.Join(errors, "; ")
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(health)
}

func getRoleFromContext(ctx context.Context) string {
	if role, ok := ctx.Value(userRoleContextKey).(string); ok {
		return role
	}
	return "Viewer"
}

func getUsernameFromContext(ctx context.Context) string {
	if username, ok := ctx.Value(userNameContextKey).(string); ok {
		return username
	}
	return "unknown"
}

func (a *API) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		bearerToken := strings.Split(authHeader, " ")
		if len(bearerToken) != 2 {
			http.Error(w, "Invalid token format", http.StatusUnauthorized)
			return
		}

		tokenStr := bearerToken[1]
		claims := &models.Claims{}

		token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
			return jwtSecretKey, nil
		})

		if err != nil || !token.Valid {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), userRoleContextKey, claims.Role)
		ctx = context.WithValue(ctx, userNameContextKey, claims.Username)
		ctx = context.WithValue(ctx, tokenContextKey, tokenStr)
		ctx = context.WithValue(ctx, claimsContextKey, claims)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *API) CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (a *API) ApprovalsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	cluster := vars["cluster"]
	status := r.URL.Query().Get("status")

	log.Printf("ApprovalsHandler: cluster=%s, status=%s", cluster, status)

	changes := services.ListPendingChanges(cluster, status)

	log.Printf("ApprovalsHandler: found %d changes", len(changes))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(changes)
}

func (a *API) ApproveChangeHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	change, ok := services.GetPendingChange(id)
	if !ok {
		http.Error(w, "Change not found", http.StatusNotFound)
		return
	}

	if change.Status != "pending" {
		http.Error(w, "Change is not pending", http.StatusBadRequest)
		return
	}

	// Execute the change
	req := services.ToResourceUpdateRequest(change)
	err := a.K8s.ExecuteResourceUpdate(r.Context(), change.Cluster, change.Namespace, change.Kind, change.Name, req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to execute update: %v", err), http.StatusInternalServerError)
		return
	}

	// Extract username from context
	username, _ := r.Context().Value(userNameContextKey).(string)
	if username == "" {
		username = "unknown"
	}

	// Update status and decision fields
	now := time.Now()
	change.Status = "approved"
	change.DecisionBy = username
	change.DecisionAt = &now
	if err := services.UpdatePendingChange(change); err != nil {
		log.Printf("Failed to update change status: %v", err)
	}

	w.WriteHeader(http.StatusOK)
}

func (a *API) RejectChangeHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	change, ok := services.GetPendingChange(id)
	if !ok {
		http.Error(w, "Change not found", http.StatusNotFound)
		return
	}

	if change.Status != "pending" {
		http.Error(w, "Change is not pending", http.StatusBadRequest)
		return
	}

	// Extract username from context
	username, _ := r.Context().Value(userNameContextKey).(string)
	if username == "" {
		username = "unknown"
	}

	// Update status and decision fields
	now := time.Now()
	change.Status = "rejected"
	change.DecisionBy = username
	change.DecisionAt = &now
	if err := services.UpdatePendingChange(change); err != nil {
		log.Printf("Failed to update change status: %v", err)
		http.Error(w, "Failed to update change status", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (a *API) AlertsHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("AlertsHandler called: method=%s", r.Method)

	if r.Method == http.MethodGet {
		overview, err := services.GetAlertsOverview()
		if err != nil {
			log.Printf("Error getting alerts overview: %v", err)
			http.Error(w, fmt.Sprintf("Failed to get alerts: %v", err), http.StatusInternalServerError)
			return
		}
		log.Printf("Successfully retrieved alerts: %d rules, %d history items", len(overview.Rules), len(overview.History))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(overview)
		return
	}

	if r.Method == http.MethodPost {
		role := getRoleFromContext(r.Context())
		if role != "Admin" && role != "Editor" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		var payload struct {
			Rules []models.AlertRuleInput `json:"rules"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		if err := services.SaveAlertRules(payload.Rules); err != nil {
			http.Error(w, fmt.Sprintf("Failed to save rules: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Rules saved successfully"))
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (a *API) EventsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	cluster := vars["cluster"]

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	events, err := a.K8s.GetEvents(ctx, cluster)
	if err != nil {
		// Log error but return empty list to avoid breaking frontend
		log.Printf("Error fetching events: %v", err)
		events = []models.Event{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(events)
}

func (a *API) UpdateWorkloadResourcesHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clusterName := vars["cluster"]
	namespace := vars["namespace"]
	kind := vars["kind"]
	name := vars["name"]

	var req models.ResourceUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	role := getRoleFromContext(r.Context())
	username := getUsernameFromContext(r.Context())

	// If user is not Admin, create a pending change request
	if role != "Admin" {
		payload := make(map[string]string)
		if req.CpuRequests != nil {
			payload["cpuRequests"] = *req.CpuRequests
		}
		if req.CpuLimits != nil {
			payload["cpuLimits"] = *req.CpuLimits
		}
		if req.MemoryRequests != nil {
			payload["memoryRequests"] = *req.MemoryRequests
		}
		if req.MemoryLimits != nil {
			payload["memoryLimits"] = *req.MemoryLimits
		}

		if len(req.Containers) > 0 {
			containersJson, _ := json.Marshal(req.Containers)
			payload["containers_json"] = string(containersJson)
		}

		_, err := services.AddPendingChange(clusterName, namespace, kind, name, username, role, payload)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to create pending change: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("Change request submitted for approval"))
		return
	}

	// If Admin, execute immediately
	err := a.K8s.ExecuteResourceUpdate(r.Context(), clusterName, namespace, kind, name, &req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to update resources: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Resources updated successfully"))
}

func (a *API) PodLogsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clusterName := vars["cluster"]
	namespace := vars["namespace"]
	podName := vars["pod"]

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	logs, err := a.K8s.GetPodLogs(ctx, clusterName, namespace, podName)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error fetching logs: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(logs))
}

func (a *API) WorkloadMetricsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	// clusterName := vars["cluster"] // Not used yet, assuming single prometheus
	namespace := vars["namespace"]
	name := vars["name"]
	rangeStr := r.URL.Query().Get("range")

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	metrics, err := a.K8s.GetWorkloadMetrics(ctx, namespace, name, rangeStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error fetching metrics: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}
