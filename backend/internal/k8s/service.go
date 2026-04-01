package k8s

import (
	"context"
	"fmt"
	"io"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"kubernetes-manager/backend/internal/models"

	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

type Service struct {
	Clients map[string]*models.ClusterClients
	PromAPI prometheusv1.API
}

func NewService(clients map[string]*models.ClusterClients, promAPI prometheusv1.API) *Service {
	return &Service{Clients: clients, PromAPI: promAPI}
}

// --- Cache ---
var (
	cacheMutex    sync.RWMutex
	scalarCache   = make(map[string]models.CacheItem)
	rangeCache    = make(map[string]models.RangeCacheItem)
	shortCacheTTL = 30 * time.Second
	longCacheTTL  = 15 * time.Minute
)

// --- Workloads (Optimized) ---

func (s *Service) CollectClusterWorkloads(ctx context.Context) []models.WorkloadInfo {
	var allInfos []models.WorkloadInfo

	// Bulk fetch metrics (Metrics API) - assuming global or per cluster?
	// If we have multiple clusters, we might need to fetch per cluster if DynamicClient is per cluster.
	// getAllPodMetrics uses s.Clients.DynamicClient.
	// So we need to iterate.

	for clusterName, client := range s.Clients {
		infos := s.CollectFromCluster(ctx, clusterName, client)
		allInfos = append(allInfos, infos...)
	}
	return allInfos
}

func (s *Service) CollectFromCluster(ctx context.Context, clusterName string, client *models.ClusterClients) []models.WorkloadInfo {
	var infos []models.WorkloadInfo

	// 1. Bulk fetch all resources across all namespaces
	deployments, err := client.Clientset.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("[%s] Error listing deployments: %v", clusterName, err)
		return nil
	}

	statefulsets, err := client.Clientset.AppsV1().StatefulSets("").List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("[%s] Error listing statefulsets: %v", clusterName, err)
	}

	daemonsets, err := client.Clientset.AppsV1().DaemonSets("").List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("[%s] Error listing daemonsets: %v", clusterName, err)
	}

	cronjobs, err := client.Clientset.BatchV1().CronJobs("").List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("[%s] Error listing cronjobs: %v", clusterName, err)
	}

	// 2. Bulk fetch all pods
	pods, err := client.Clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Printf("[%s] Error listing pods: %v", clusterName, err)
		return nil
	}

	// 3. Bulk fetch metrics (Metrics API)
	podMetrics := s.getAllPodMetrics(ctx, client)

	// 4. Index pods by namespace for faster lookup
	podsByNs := make(map[string][]corev1.Pod)
	for _, pod := range pods.Items {
		podsByNs[pod.Namespace] = append(podsByNs[pod.Namespace], pod)
	}

	// 5. Process Workloads
	// Deployments
	for _, d := range deployments.Items {
		selector, err := metav1.LabelSelectorAsSelector(d.Spec.Selector)
		if err != nil {
			continue
		}
		info := s.processWorkloadOptimized(d.Name, d.Namespace, "Deployment", d.Spec.Template.Spec, selector, podsByNs[d.Namespace], podMetrics)
		// Add cluster name to info? Model doesn't have Cluster field?
		// WorkloadInfo doesn't have Cluster field.
		// I should add it to models.WorkloadInfo if I want to distinguish.
		// But for now I'll just append.
		infos = append(infos, info)
	}

	// StatefulSets
	if statefulsets != nil {
		for _, ss := range statefulsets.Items {
			selector, err := metav1.LabelSelectorAsSelector(ss.Spec.Selector)
			if err != nil {
				continue
			}
			info := s.processWorkloadOptimized(ss.Name, ss.Namespace, "StatefulSet", ss.Spec.Template.Spec, selector, podsByNs[ss.Namespace], podMetrics)
			infos = append(infos, info)
		}
	}

	// DaemonSets
	if daemonsets != nil {
		for _, ds := range daemonsets.Items {
			selector, err := metav1.LabelSelectorAsSelector(ds.Spec.Selector)
			if err != nil {
				continue
			}
			info := s.processWorkloadOptimized(ds.Name, ds.Namespace, "DaemonSet", ds.Spec.Template.Spec, selector, podsByNs[ds.Namespace], podMetrics)
			infos = append(infos, info)
		}
	}

	// CronJobs
	if cronjobs != nil {
		for _, cj := range cronjobs.Items {
			selector, err := metav1.LabelSelectorAsSelector(cj.Spec.JobTemplate.Spec.Selector)
			if err != nil {
				continue
			}
			info := s.processWorkloadOptimized(cj.Name, cj.Namespace, "CronJob", cj.Spec.JobTemplate.Spec.Template.Spec, selector, podsByNs[cj.Namespace], podMetrics)
			infos = append(infos, info)
		}
	}

	return infos
}

func (s *Service) processWorkloadOptimized(name, namespace, kind string, podSpec corev1.PodSpec, selector labels.Selector, nsPods []corev1.Pod, allMetrics map[string]models.K8sContainerMetrics) models.WorkloadInfo {
	cpuReqTotal, cpuLimTotal, memReqTotal, memLimTotal, _, _, _, _ := getResourceTotals(podSpec)

	// Filter pods belonging to this workload
	var workloadPods []corev1.Pod
	for _, pod := range nsPods {
		if selector.Matches(labels.Set(pod.Labels)) {
			workloadPods = append(workloadPods, pod)
		}
	}

	// Aggregate metrics from pods
	var totalCpuUsage, totalMemUsage int64
	var podCount int64
	oomKilledByContainer := make(map[string]int)

	containerMetricsSum := make(map[string]models.K8sContainerMetrics)
	containerCounts := make(map[string]int64)

	for _, pod := range workloadPods {
		podCount++
		// Check OOM
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.LastTerminationState.Terminated != nil && cs.LastTerminationState.Terminated.Reason == "OOMKilled" {
				oomKilledByContainer[cs.Name]++
			}
		}

		// Sum metrics
		for _, container := range pod.Spec.Containers {
			// Key for metrics map: namespace/podName/containerName
			key := fmt.Sprintf("%s/%s/%s", namespace, pod.Name, container.Name)
			if m, ok := allMetrics[key]; ok {
				totalCpuUsage += m.CpuUsage
				totalMemUsage += m.MemoryUsage

				cSum := containerMetricsSum[container.Name]
				cSum.CpuUsage += m.CpuUsage
				cSum.MemoryUsage += m.MemoryUsage
				containerMetricsSum[container.Name] = cSum
				containerCounts[container.Name]++
			}
		}
	}

	var avgCpuUsage, avgMemUsage int64
	if podCount > 0 {
		avgCpuUsage = totalCpuUsage / podCount
		avgMemUsage = totalMemUsage / podCount
	}

	// Build Container Infos
	var containerInfos []models.ContainerInfo
	for _, c := range podSpec.Containers {
		cInfo := models.ContainerInfo{
			Name:            c.Name,
			Image:           c.Image,
			CpuRequests:     "0",
			CpuLimits:       "0",
			MemoryRequests:  "0",
			MemoryLimits:    "0",
			Recommendations: []string{},
		}

		if reqCpu, ok := c.Resources.Requests[corev1.ResourceCPU]; ok {
			cInfo.CpuRequests = reqCpu.String()
		}
		if limCpu, ok := c.Resources.Limits[corev1.ResourceCPU]; ok {
			cInfo.CpuLimits = limCpu.String()
		}
		if reqMem, ok := c.Resources.Requests[corev1.ResourceMemory]; ok {
			cInfo.MemoryRequests = reqMem.String()
		}
		if limMem, ok := c.Resources.Limits[corev1.ResourceMemory]; ok {
			cInfo.MemoryLimits = limMem.String()
		}

		// Calculate average for this container
		if count := containerCounts[c.Name]; count > 0 {
			sum := containerMetricsSum[c.Name]
			cInfo.AvgCpuUsage = sum.CpuUsage / count
			cInfo.AvgMemoryUsage = sum.MemoryUsage / count
		}

		// Recommendations
		if count := oomKilledByContainer[c.Name]; count > 0 {
			cInfo.Recommendations = append(cInfo.Recommendations, fmt.Sprintf("Krytyczne: Kontener został zatrzymany %d razy z powodu OOMKilled!", count))
		}

		// Check limits vs current usage
		var cCpuLimMilli int64
		if limCpu, ok := c.Resources.Limits[corev1.ResourceCPU]; ok {
			cCpuLimMilli = limCpu.MilliValue()
		}
		var cMemLimBytes int64
		if limMem, ok := c.Resources.Limits[corev1.ResourceMemory]; ok {
			cMemLimBytes = limMem.Value()
		}

		if cCpuLimMilli > 0 && cInfo.AvgCpuUsage > 0 {
			usageRatio := float64(cInfo.AvgCpuUsage) / float64(cCpuLimMilli)
			if usageRatio > 0.9 {
				cInfo.Recommendations = append(cInfo.Recommendations, fmt.Sprintf("Ostrzeżenie (aktualne): Średnie zużycie CPU (%dm - %.0f%% limitu %s) bliskie limitu!", cInfo.AvgCpuUsage, usageRatio*100, cInfo.CpuLimits))
			}
		}

		if cMemLimBytes > 0 && cInfo.AvgMemoryUsage > 0 {
			usageRatio := float64(cInfo.AvgMemoryUsage) / float64(cMemLimBytes)
			if usageRatio > 0.9 {
				cInfo.Recommendations = append(cInfo.Recommendations, fmt.Sprintf("Krytyczne (aktualne): Średnie zużycie Pamięci (%s - %.0f%% limitu %s) bliskie limitu!", formatBytesTrim(cInfo.AvgMemoryUsage), usageRatio*100, cInfo.MemoryLimits))
			} else if usageRatio > 0.8 {
				cInfo.Recommendations = append(cInfo.Recommendations, fmt.Sprintf("Ostrzeżenie (aktualne): Średnie zużycie Pamięci (%s - %.0f%% limitu %s) jest wysokie.", formatBytesTrim(cInfo.AvgMemoryUsage), usageRatio*100, cInfo.MemoryLimits))
			}
		}

		containerInfos = append(containerInfos, cInfo)
	}

	return models.WorkloadInfo{
		Name:            name,
		Namespace:       namespace,
		Kind:            kind,
		CpuRequests:     cpuReqTotal.String(),
		CpuLimits:       cpuLimTotal.String(),
		MemoryRequests:  memReqTotal.String(),
		MemoryLimits:    memLimTotal.String(),
		AvgCpuUsage:     avgCpuUsage,
		AvgMemoryUsage:  avgMemUsage,
		Recommendations: []string{},
		Containers:      containerInfos,
	}
}

// --- Helpers ---

// getAllPodMetrics fetches metrics for ALL pods in the cluster
func (s *Service) getAllPodMetrics(ctx context.Context, client *models.ClusterClients) map[string]models.K8sContainerMetrics {
	results := make(map[string]models.K8sContainerMetrics)

	gvr := schema.GroupVersionResource{
		Group:    "metrics.k8s.io",
		Version:  "v1beta1",
		Resource: "pods",
	}

	unstructuredList, err := client.DynamicClient.Resource(gvr).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil
	}

	for _, item := range unstructuredList.Items {
		podName := item.GetName()
		namespace := item.GetNamespace()

		containers, found, err := unstructured.NestedSlice(item.Object, "containers")
		if !found || err != nil {
			continue
		}

		for _, c := range containers {
			cMap, ok := c.(map[string]interface{})
			if !ok {
				continue
			}

			containerName, _ := cMap["name"].(string)
			usage, ok := cMap["usage"].(map[string]interface{})
			if !ok {
				continue
			}

			cpuStr, _ := usage["cpu"].(string)
			memStr, _ := usage["memory"].(string)

			cpuQty, _ := resource.ParseQuantity(cpuStr)
			memQty, _ := resource.ParseQuantity(memStr)

			key := fmt.Sprintf("%s/%s/%s", namespace, podName, containerName)
			results[key] = models.K8sContainerMetrics{
				CpuUsage:    cpuQty.MilliValue(),
				MemoryUsage: memQty.Value(),
			}
		}
	}
	return results
}

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

// --- Resource Updates ---

func (s *Service) ExecuteResourceUpdate(ctx context.Context, clusterName, namespace, kind, name string, reqUpdate *models.ResourceUpdateRequest) error {
	client, ok := s.Clients[clusterName]
	if !ok {
		// Try to find the resource in any cluster if clusterName is empty
		if clusterName == "" {
			for cName, c := range s.Clients {
				// Check if resource exists
				// This is expensive and risky.
				// For now, let's assume we need clusterName.
				// Or we can try to get it.
				_, err := s.getResource(ctx, c, namespace, kind, name)
				if err == nil {
					client = c
					clusterName = cName
					break
				}
			}
		}
	}

	if client == nil {
		return fmt.Errorf("cluster not found or resource not found in any cluster")
	}

	clientset := client.Clientset
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
		return fmt.Errorf("unsupported kind: %s", kind)
	}

	// Update logic
	if len(reqUpdate.Containers) > 0 {
		for _, cUpdate := range reqUpdate.Containers {
			for i := range podSpec.Containers {
				if podSpec.Containers[i].Name == cUpdate.Name {
					newRes, err := parseResourceRequirements(&models.ResourceUpdateRequest{
						CpuRequests:    cUpdate.CpuRequests,
						CpuLimits:      cUpdate.CpuLimits,
						MemoryRequests: cUpdate.MemoryRequests,
						MemoryLimits:   cUpdate.MemoryLimits,
					})
					if err != nil {
						return err
					}
					podSpec.Containers[i].Resources = newRes
				}
			}
		}
	}

	_, err := updateFunc(ctx, metav1.UpdateOptions{})
	return err
}

func parseResourceRequirements(reqUpdate *models.ResourceUpdateRequest) (corev1.ResourceRequirements, error) {
	requests := corev1.ResourceList{}
	limits := corev1.ResourceList{}

	parseField := func(field *string, resName corev1.ResourceName, list corev1.ResourceList) {
		if field != nil && *field != "" {
			if qty, err := resource.ParseQuantity(*field); err == nil {
				list[resName] = qty
			}
		}
	}

	parseField(reqUpdate.CpuRequests, corev1.ResourceCPU, requests)
	parseField(reqUpdate.CpuLimits, corev1.ResourceCPU, limits)
	parseField(reqUpdate.MemoryRequests, corev1.ResourceMemory, requests)
	parseField(reqUpdate.MemoryLimits, corev1.ResourceMemory, limits)

	return corev1.ResourceRequirements{Requests: requests, Limits: limits}, nil
}

func (s *Service) getResource(ctx context.Context, client *models.ClusterClients, namespace, kind, name string) (runtime.Object, error) {
	switch kind {
	case "Deployment":
		return client.Clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	case "StatefulSet":
		return client.Clientset.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
	case "DaemonSet":
		return client.Clientset.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
	default:
		return nil, fmt.Errorf("unsupported kind: %s", kind)
	}
}

func formatBytesTrim(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func (s *Service) GetNodePods(ctx context.Context, clusterName, nodeName string) ([]models.PodInfo, error) {
	// Helper function to fetch pods from a specific client
	fetchPods := func(client *models.ClusterClients) ([]models.PodInfo, error) {
		// Check if node exists in this cluster
		_, err := client.Clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		fieldSelector := fmt.Sprintf("spec.nodeName=%s", nodeName)
		pods, err := client.Clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{FieldSelector: fieldSelector})
		if err != nil {
			return nil, err
		}

		var podsInfo []models.PodInfo
		for _, pod := range pods.Items {
			status, reason := getPodStatus(pod)
			containers, totalRestarts := buildContainerInfos(pod)
			info := models.PodInfo{
				Name:           pod.Name,
				Namespace:      pod.Namespace,
				Status:         status,
				Reason:         reason,
				ContainerCount: len(containers),
				RestartCount:   totalRestarts,
				Containers:     containers,
			}
			podsInfo = append(podsInfo, info)
		}
		return podsInfo, nil
	}

	if clusterName != "" {
		if client, ok := s.Clients[clusterName]; ok {
			pods, err := fetchPods(client)
			if err == nil {
				return pods, nil
			}
			return nil, fmt.Errorf("node %s not found in cluster %s: %v", nodeName, clusterName, err)
		}
		return nil, fmt.Errorf("cluster %s not found", clusterName)
	}

	// Fallback: iterate all clusters
	for _, client := range s.Clients {
		pods, err := fetchPods(client)
		if err == nil {
			return pods, nil
		}
	}
	return nil, fmt.Errorf("node %s not found in any configured cluster", nodeName)
}

func getPodStatus(pod corev1.Pod) (string, string) {
	if pod.Status.Phase == corev1.PodPending {
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
		allReady := true
		for _, cs := range pod.Status.ContainerStatuses {
			if !cs.Ready {
				allReady = false
				if cs.State.Waiting != nil && cs.State.Waiting.Reason == "CrashLoopBackOff" {
					return "CrashLoopBackOff", cs.LastTerminationState.Terminated.Reason
				}
				if cs.State.Terminated != nil {
					return "Terminated", cs.State.Terminated.Reason
				}
			}
		}
		if allReady {
			return string(pod.Status.Phase), ""
		}
		return "Running", "NotReady"
	}

	return string(pod.Status.Phase), pod.Status.Reason
}

func buildContainerInfos(pod corev1.Pod) ([]models.ContainerInfo, int32) {
	statusMap := make(map[string]corev1.ContainerStatus)
	for _, cs := range pod.Status.ContainerStatuses {
		statusMap[cs.Name] = cs
	}

	var containers []models.ContainerInfo
	var totalRestarts int32
	for _, container := range pod.Spec.Containers {
		cs, ok := statusMap[container.Name]
		ready := false
		restartCount := int32(0)
		state := "Unknown"
		reason := ""
		message := ""
		if ok {
			ready = cs.Ready
			restartCount = cs.RestartCount
			totalRestarts += cs.RestartCount
			if cs.State.Waiting != nil {
				state = "Waiting"
				reason = cs.State.Waiting.Reason
				message = cs.State.Waiting.Message
			} else if cs.State.Running != nil {
				state = "Running"
			} else if cs.State.Terminated != nil {
				state = "Terminated"
				reason = cs.State.Terminated.Reason
				message = cs.State.Terminated.Message
			}
		}

		containers = append(containers, models.ContainerInfo{
			Name:           container.Name,
			Image:          container.Image,
			Ready:          ready,
			RestartCount:   restartCount,
			State:          state,
			Reason:         reason,
			Message:        message,
			CpuRequests:    getResourceValue(container.Resources.Requests, corev1.ResourceCPU),
			CpuLimits:      getResourceValue(container.Resources.Limits, corev1.ResourceCPU),
			MemoryRequests: getResourceValue(container.Resources.Requests, corev1.ResourceMemory),
			MemoryLimits:   getResourceValue(container.Resources.Limits, corev1.ResourceMemory),
		})
	}

	return containers, totalRestarts
}

func getResourceValue(resources corev1.ResourceList, name corev1.ResourceName) string {
	if quantity, ok := resources[name]; ok {
		if quantity.IsZero() {
			return ""
		}
		return quantity.String()
	}
	return ""
}

func (s *Service) GetEvents(ctx context.Context, clusterName string) ([]models.Event, error) {
	client, ok := s.Clients[clusterName]
	if !ok {
		// Fallback to first cluster if not found (or handle error)
		// For now, return error
		return nil, fmt.Errorf("cluster %s not found", clusterName)
	}

	events, err := client.Clientset.CoreV1().Events("").List(ctx, metav1.ListOptions{
		Limit: 50,
	})
	if err != nil {
		return nil, err
	}

	var result []models.Event
	for _, e := range events.Items {
		result = append(result, models.Event{
			Type:          e.Type,
			Reason:        e.Reason,
			Message:       e.Message,
			LastTimestamp: e.LastTimestamp.Time,
			InvolvedObject: models.ObjectReference{
				Kind: e.InvolvedObject.Kind,
				Name: e.InvolvedObject.Name,
			},
		})
	}

	// Sort by LastTimestamp descending
	sort.Slice(result, func(i, j int) bool {
		return result[i].LastTimestamp.After(result[j].LastTimestamp)
	})

	return result, nil
}

func (s *Service) GetPodLogs(ctx context.Context, clusterName, namespace, podName string) (string, error) {
	client, ok := s.Clients[clusterName]
	if !ok {
		return "", fmt.Errorf("cluster %s not found", clusterName)
	}

	// Get pod to check container count
	pod, err := client.Clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("error getting pod: %v", err)
	}

	var allLogs strings.Builder

	// Get logs from all containers
	containers := pod.Spec.Containers
	for i, container := range containers {
		if i > 0 {
			allLogs.WriteString("\n\n" + strings.Repeat("=", 80) + "\n")
		}

		if len(containers) > 1 {
			allLogs.WriteString(fmt.Sprintf("=== Container: %s ===\n\n", container.Name))
		}

		podLogOpts := corev1.PodLogOptions{
			Container: container.Name,
			TailLines: func(i int64) *int64 { return &i }(100),
		}
		req := client.Clientset.CoreV1().Pods(namespace).GetLogs(podName, &podLogOpts)
		podLogs, err := req.Stream(ctx)
		if err != nil {
			allLogs.WriteString(fmt.Sprintf("Error fetching logs for container %s: %v\n", container.Name, err))
			continue
		}

		buf := new(strings.Builder)
		_, err = io.Copy(buf, podLogs)
		podLogs.Close()

		if err != nil {
			allLogs.WriteString(fmt.Sprintf("Error reading logs for container %s: %v\n", container.Name, err))
		} else {
			allLogs.WriteString(buf.String())
		}
	}

	return allLogs.String(), nil
}
