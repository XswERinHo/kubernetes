package k8s

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	"kubernetes-manager/backend/internal/models"

	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	avgCpuQueryTemplate    = `sum(rate(container_cpu_usage_seconds_total%s[5m])) * 1000`
	avgMemQueryTemplate    = `sum(container_memory_working_set_bytes%s)`
	p95CpuQueryTemplate    = `sum(quantile_over_time(0.95, rate(container_cpu_usage_seconds_total%s[5m])[7d:5m])) * 1000`
	p95MemQueryTemplate    = `sum(quantile_over_time(0.95, container_memory_working_set_bytes%s[7d:5m]))`
	nodeCpuUsageQuery      = `sum by (node) (rate(container_cpu_usage_seconds_total{pod!=""}[5m])) * 1000`
	nodeMemUsageQuery      = `sum by (node) (container_memory_working_set_bytes{pod!=""})`
	nodeDiskCapacityQuery  = `sum by (node) (node_filesystem_size_bytes{mountpoint="/",fstype!~"tmpfs|aufs|squashfs|overlay"})`
	nodeDiskAvailableQuery = `sum by (node) (node_filesystem_avail_bytes{mountpoint="/",fstype!~"tmpfs|aufs|squashfs|overlay"})`

	workloadCpuHistoryQuery = `sum(rate(container_cpu_usage_seconds_total{namespace="%s", pod=~"%s-.*"}[5m])) * 1000`
	workloadMemHistoryQuery = `sum(container_memory_working_set_bytes{namespace="%s", pod=~"%s-.*"})`
)

func buildPrometheusSelector(namespace, workloadName string) string {
	return fmt.Sprintf(`{namespace="%s", pod=~"%s-.*"}`, namespace, workloadName)
}

func (s *Service) QueryPrometheusVectorAsMap(ctx context.Context, query string) (map[string]int64, error) {
	if s.PromAPI == nil {
		return nil, fmt.Errorf("Prometheus API not initialized")
	}
	result, warnings, err := s.PromAPI.Query(ctx, query, time.Now())
	if err != nil {
		return nil, fmt.Errorf("error querying Prometheus (%s): %v", query, err)
	}
	if len(warnings) > 0 {
		log.Printf("Prometheus warnings: %v", warnings)
	}

	vector, ok := result.(model.Vector)
	if !ok {
		return nil, fmt.Errorf("expected vector from Prometheus, got %T", result)
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

func (s *Service) QueryPrometheusRange(ctx context.Context, query string, start, end time.Time, step time.Duration) ([]models.MetricPoint, error) {
	if s.PromAPI == nil {
		return nil, fmt.Errorf("Prometheus API not initialized")
	}

	r := prometheusv1.Range{
		Start: start,
		End:   end,
		Step:  step,
	}

	result, warnings, err := s.PromAPI.QueryRange(ctx, query, r)
	if err != nil {
		return nil, fmt.Errorf("error querying Prometheus range (%s): %v", query, err)
	}
	if len(warnings) > 0 {
		log.Printf("Prometheus warnings: %v", warnings)
	}

	matrix, ok := result.(model.Matrix)
	if !ok {
		return nil, fmt.Errorf("expected matrix from Prometheus, got %T", result)
	}

	var points []models.MetricPoint
	if len(matrix) > 0 {
		// Assuming single series for the sum query
		for _, sample := range matrix[0].Values {
			points = append(points, models.MetricPoint{
				Timestamp: sample.Timestamp.Time().Unix() * 1000, // JS expects ms
				Value:     float64(sample.Value),
			})
		}
	}
	return points, nil
}

func (s *Service) GetWorkloadMetrics(ctx context.Context, namespace, workloadName, rangeStr string) (*models.MetricHistory, error) {
	var start time.Time
	var step time.Duration
	end := time.Now()

	switch rangeStr {
	case "1h":
		start = end.Add(-1 * time.Hour)
		step = 1 * time.Minute
	case "6h":
		start = end.Add(-6 * time.Hour)
		step = 5 * time.Minute
	case "24h":
		start = end.Add(-24 * time.Hour)
		step = 15 * time.Minute
	case "7d":
		start = end.Add(-7 * 24 * time.Hour)
		step = 1 * time.Hour
	default:
		start = end.Add(-1 * time.Hour)
		step = 1 * time.Minute
	}

	cpuQuery := fmt.Sprintf(workloadCpuHistoryQuery, namespace, workloadName)
	memQuery := fmt.Sprintf(workloadMemHistoryQuery, namespace, workloadName)

	cpuData, err := s.QueryPrometheusRange(ctx, cpuQuery, start, end, step)
	if err != nil {
		log.Printf("Error fetching CPU history: %v", err)
		// Return empty instead of failing completely?
		cpuData = []models.MetricPoint{}
	}

	memData, err := s.QueryPrometheusRange(ctx, memQuery, start, end, step)
	if err != nil {
		log.Printf("Error fetching Memory history: %v", err)
		memData = []models.MetricPoint{}
	}

	return &models.MetricHistory{
		CpuUsage:    cpuData,
		MemoryUsage: memData,
	}, nil
}

func (s *Service) QueryPrometheusScalarCached(query, key string, ttl time.Duration) int64 {
	cacheMutex.RLock()
	if item, found := scalarCache[key]; found {
		if time.Now().Before(item.Expiry) {
			cacheMutex.RUnlock()
			return item.Data
		}
	}
	cacheMutex.RUnlock()

	if s.PromAPI == nil {
		return 0
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, warnings, err := s.PromAPI.Query(ctx, query, time.Now())
	if err != nil {
		log.Printf("Error querying Prometheus (%s): %v", query, err)
		return 0
	}
	if len(warnings) > 0 {
		log.Printf("Prometheus warnings (%s): %v", query, warnings)
	}

	var value int64
	switch v := result.(type) {
	case *model.Scalar:
		value = int64(v.Value)
	case model.Vector:
		if len(v) > 0 {
			value = int64(v[0].Value)
		}
	}

	cacheMutex.Lock()
	scalarCache[key] = models.CacheItem{
		Data:   value,
		Expiry: time.Now().Add(ttl),
	}
	cacheMutex.Unlock()

	return value
}

func (s *Service) GetNodeMetricsFromPrometheus(ctx context.Context, clusterName string) ([]models.NodeInfo, error) {
	cpuUsageMap, err := s.QueryPrometheusVectorAsMap(ctx, nodeCpuUsageQuery)
	if err != nil {
		log.Printf("Error fetching node CPU usage: %v", err)
	}
	memUsageMap, err := s.QueryPrometheusVectorAsMap(ctx, nodeMemUsageQuery)
	if err != nil {
		log.Printf("Error fetching node Memory usage: %v", err)
	}
	diskCapacityMap, err := s.QueryPrometheusVectorAsMap(ctx, nodeDiskCapacityQuery)
	if err != nil {
		log.Printf("Error fetching node Disk capacity: %v", err)
	}
	diskAvailableMap, err := s.QueryPrometheusVectorAsMap(ctx, nodeDiskAvailableQuery)
	if err != nil {
		log.Printf("Error fetching node Disk available: %v", err)
	}

	var allNodes []models.NodeInfo

	for name, client := range s.Clients {
		if clusterName != "" && name != clusterName {
			continue
		}

		nodes, err := client.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			log.Printf("Error listing nodes for cluster %s: %v", name, err)
			continue
		}

		for _, node := range nodes.Items {
			// Count pods on this node
			fieldSelector := fmt.Sprintf("spec.nodeName=%s", node.Name)
			pods, err := client.Clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{FieldSelector: fieldSelector})
			podCount := 0
			if err == nil {
				podCount = len(pods.Items)
			} else {
				log.Printf("Error listing pods for node %s: %v", node.Name, err)
			}

			cpuCap := node.Status.Capacity.Cpu().String()
			memCap := node.Status.Capacity.Memory().String()
			cpuAlloc := node.Status.Allocatable.Cpu().String()
			memAlloc := node.Status.Allocatable.Memory().String()

			cpuAllocMilli := node.Status.Allocatable.Cpu().MilliValue()
			memAllocBytes := node.Status.Allocatable.Memory().Value()

			ephemeralCap := node.Status.Capacity.StorageEphemeral().String()
			ephemeralAlloc := node.Status.Allocatable.StorageEphemeral().String()
			ephemeralAllocBytes := node.Status.Allocatable.StorageEphemeral().Value()
			ephemeralCapBytes := node.Status.Capacity.StorageEphemeral().Value()

			info := models.NodeInfo{
				Name:                             node.Name,
				Status:                           string(node.Status.Conditions[len(node.Status.Conditions)-1].Type),
				PodCount:                         podCount,
				CpuCapacity:                      cpuCap,
				MemoryCapacity:                   memCap,
				CpuAllocatable:                   cpuAlloc,
				MemoryAllocatable:                memAlloc,
				CpuAllocatableMilli:              cpuAllocMilli,
				MemoryAllocatableBytes:           memAllocBytes,
				CpuUsage:                         cpuUsageMap[node.Name],
				MemoryUsage:                      memUsageMap[node.Name],
				Labels:                           node.Labels,
				Taints:                           []string{},
				EphemeralStorageCapacity:         ephemeralCap,
				EphemeralStorageAllocatable:      ephemeralAlloc,
				EphemeralStorageAllocatableBytes: ephemeralAllocBytes,
				EphemeralStorageCapacityBytes:    ephemeralCapBytes,
				DiskCapacityBytes:                diskCapacityMap[node.Name],
				DiskAvailableBytes:               diskAvailableMap[node.Name],
				DiskUsageBytes:                   diskCapacityMap[node.Name] - diskAvailableMap[node.Name],
			}

			for _, t := range node.Spec.Taints {
				info.Taints = append(info.Taints, fmt.Sprintf("%s=%s:%s", t.Key, t.Value, t.Effect))
			}

			allNodes = append(allNodes, info)
		}
	}
	return allNodes, nil
}
