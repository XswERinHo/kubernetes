package models

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	monitoringClient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// --- Cache ---
type CacheItem struct {
	Data   int64
	Expiry time.Time
}

type RangeCacheItem struct {
	Data   []MetricPoint
	Expiry time.Time
}

// --- Node & Pod Info ---
type NodeInfo struct {
	Name                             string            `json:"name"`
	Status                           string            `json:"status"`
	CpuCapacity                      string            `json:"cpuCapacity"`
	MemoryCapacity                   string            `json:"memoryCapacity"`
	CpuAllocatable                   string            `json:"cpuAllocatable"`
	MemoryAllocatable                string            `json:"memoryAllocatable"`
	CpuUsage                         int64             `json:"cpuUsage"`
	MemoryUsage                      int64             `json:"memoryUsage"`
	CpuAllocatableMilli              int64             `json:"cpuAllocatableMilli"`
	MemoryAllocatableBytes           int64             `json:"memoryAllocatableBytes"`
	PodCount                         int               `json:"podCount"`
	Labels                           map[string]string `json:"labels"`
	Taints                           []string          `json:"taints"`
	EphemeralStorageCapacity         string            `json:"ephemeralStorageCapacity"`
	EphemeralStorageAllocatable      string            `json:"ephemeralStorageAllocatable"`
	EphemeralStorageAllocatableBytes int64             `json:"ephemeralStorageAllocatableBytes"`
	EphemeralStorageCapacityBytes    int64             `json:"ephemeralStorageCapacityBytes"`
	DiskCapacityBytes                int64             `json:"diskCapacityBytes"`
	DiskUsageBytes                   int64             `json:"diskUsageBytes"`
	DiskAvailableBytes               int64             `json:"diskAvailableBytes"`
}

type PodInfo struct {
	Name           string          `json:"name"`
	Namespace      string          `json:"namespace"`
	Status         string          `json:"status"`
	Reason         string          `json:"reason,omitempty"`
	ContainerCount int             `json:"containerCount"`
	RestartCount   int32           `json:"restartCount"`
	Containers     []ContainerInfo `json:"containers"`
}

type ContainerInfo struct {
	Name            string   `json:"name"`
	Image           string   `json:"image"`
	Ready           bool     `json:"ready"`
	RestartCount    int32    `json:"restartCount"`
	State           string   `json:"state"`
	Reason          string   `json:"reason,omitempty"`
	Message         string   `json:"message,omitempty"`
	CpuRequests     string   `json:"cpuRequests,omitempty"`
	CpuLimits       string   `json:"cpuLimits,omitempty"`
	MemoryRequests  string   `json:"memoryRequests,omitempty"`
	MemoryLimits    string   `json:"memoryLimits,omitempty"`
	AvgCpuUsage     int64    `json:"avgCpuUsage"`
	AvgMemoryUsage  int64    `json:"avgMemoryUsage"`
	Recommendations []string `json:"recommendations,omitempty"`
}

// --- Events ---
type Event struct {
	Type           string          `json:"type"`
	Reason         string          `json:"reason"`
	Message        string          `json:"message"`
	LastTimestamp  time.Time       `json:"lastTimestamp"`
	InvolvedObject ObjectReference `json:"involvedObject"`
}

type ObjectReference struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

// --- System & Auth ---
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

// --- Workloads ---
type WorkloadInfo struct {
	Name            string          `json:"name"`
	Namespace       string          `json:"namespace"`
	Kind            string          `json:"kind"`
	CpuRequests     string          `json:"cpuRequests"`
	CpuLimits       string          `json:"cpuLimits"`
	MemoryRequests  string          `json:"memoryRequests"`
	MemoryLimits    string          `json:"memoryLimits"`
	AvgCpuUsage     int64           `json:"avgCpuUsage"`
	AvgMemoryUsage  int64           `json:"avgMemoryUsage"`
	Recommendations []string        `json:"recommendations"`
	Containers      []ContainerInfo `json:"containers"`
}

type ResourceUpdateRequest struct {
	Containers     []ContainerResourceUpdate `json:"containers,omitempty"`
	CpuRequests    *string                   `json:"cpuRequests,omitempty"`
	CpuLimits      *string                   `json:"cpuLimits,omitempty"`
	MemoryRequests *string                   `json:"memoryRequests,omitempty"`
	MemoryLimits   *string                   `json:"memoryLimits,omitempty"`
}

type ContainerResourceUpdate struct {
	Name           string  `json:"name"`
	CpuRequests    *string `json:"cpuRequests,omitempty"`
	CpuLimits      *string `json:"cpuLimits,omitempty"`
	MemoryRequests *string `json:"memoryRequests,omitempty"`
	MemoryLimits   *string `json:"memoryLimits,omitempty"`
}

// --- Metrics ---
type MetricPoint struct {
	Timestamp int64   `json:"timestamp"`
	Value     float64 `json:"value"`
}

type MetricHistory struct {
	CpuUsage    []MetricPoint `json:"cpuUsage"`
	MemoryUsage []MetricPoint `json:"memoryUsage"`
}

type K8sContainerMetrics struct {
	CpuUsage    int64 // millicores
	MemoryUsage int64 // bytes
}

type K8sNodeMetrics struct {
	CpuUsage    int64 // millicores
	MemoryUsage int64 // bytes
}

// --- Clients ---
type ClusterClients struct {
	Clientset           *kubernetes.Clientset
	MonitoringClientset *monitoringClient.Clientset
	DynamicClient       dynamic.Interface
}

// --- Approvals ---
type PendingChange struct {
	ID          string            `json:"id"`
	Cluster     string            `json:"cluster"`
	Namespace   string            `json:"namespace"`
	Kind        string            `json:"kind"`
	Name        string            `json:"name"`
	RequestedBy string            `json:"requestedBy"`
	Role        string            `json:"role"`
	RequestedAt time.Time         `json:"requestedAt"`
	Payload     map[string]string `json:"payload"`
	Status      string            `json:"status"`
	DecisionBy  string            `json:"decisionBy,omitempty"`
	DecisionAt  *time.Time        `json:"decisionAt,omitempty"`
	Reason      string            `json:"reason,omitempty"`
}

// --- Auth ---
type User struct {
	Username string `bson:"username" json:"username"`
	Password string `bson:"password" json:"-"` // Password hash
	Role     string `bson:"role" json:"role"`
}

// --- Alerts ---
type AlertRule struct {
	ID         string    `json:"id" bson:"id"`
	Name       string    `json:"name" bson:"name"`
	Metric     string    `json:"metric" bson:"metric"`
	Comparison string    `json:"comparison" bson:"comparison"`
	Threshold  float64   `json:"threshold" bson:"threshold"`
	Severity   string    `json:"severity" bson:"severity"`
	Window     string    `json:"window" bson:"window"`
	Channels   []string  `json:"channels" bson:"channels"`
	Enabled    bool      `json:"enabled" bson:"enabled"`
	CreatedAt  time.Time `json:"createdAt" bson:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt" bson:"updatedAt"`
}

type AlertEvent struct {
	ID          string    `json:"id" bson:"id"`
	RuleID      string    `json:"ruleId" bson:"ruleId"`
	RuleName    string    `json:"ruleName" bson:"ruleName"`
	Severity    string    `json:"severity" bson:"severity"`
	Message     string    `json:"message" bson:"message"`
	TriggeredAt time.Time `json:"triggeredAt" bson:"triggeredAt"`
	Status      string    `json:"status" bson:"status"`
	Value       float64   `json:"value" bson:"value"`
	Threshold   float64   `json:"threshold" bson:"threshold"`
	Comparison  string    `json:"comparison" bson:"comparison"`
}

type AlertsOverview struct {
	Rules   []AlertRule  `json:"rules"`
	Active  []AlertEvent `json:"active"`
	History []AlertEvent `json:"history"`
	Stats   AlertStats   `json:"stats"`
}

type AlertStats struct {
	CriticalRecommendations int     `json:"criticalRecommendations"`
	ClusterCpuUsage         float64 `json:"clusterCpuUsage"`
	ClusterMemoryUsage      float64 `json:"clusterMemoryUsage"`
}

type AlertRuleInput struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Metric     string   `json:"metric"`
	Comparison string   `json:"comparison"`
	Threshold  float64  `json:"threshold"`
	Severity   string   `json:"severity"`
	Window     string   `json:"window"`
	Channels   []string `json:"channels"`
	Enabled    bool     `json:"enabled"`
}
