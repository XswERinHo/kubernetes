package services

import (
	"context"
	"fmt"
	"log"
	"net/smtp"
	"os"
	"strings"
	"time"

	"kubernetes-manager/backend/internal/k8s"
	"kubernetes-manager/backend/internal/models"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	alertEvaluationCooldown = 5 * time.Minute
)

func EnsureDefaultAlerts() error {
	if alertsCollection == nil {
		return fmt.Errorf("alerts collection not initialized")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	count, err := alertsCollection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return err
	}

	if count == 0 {
		defaultRules := []models.AlertRule{
			{
				ID:         uuid.NewString(),
				Name:       "High CPU Usage",
				Metric:     "cpu_usage",
				Comparison: ">",
				Threshold:  80,
				Severity:   "warning",
				Window:     "5m",
				Enabled:    true,
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			},
			{
				ID:         uuid.NewString(),
				Name:       "High Memory Usage",
				Metric:     "memory_usage",
				Comparison: ">",
				Threshold:  85,
				Severity:   "critical",
				Window:     "5m",
				Enabled:    true,
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			},
		}

		for _, rule := range defaultRules {
			_, err := alertsCollection.InsertOne(ctx, rule)
			if err != nil {
				log.Printf("Failed to insert default rule: %v", err)
			}
		}
	}
	return nil
}

func StartAlertEvaluator(ctx context.Context, k8sService *k8s.Service) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	log.Println("Starting background alert evaluator...")

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			evaluateAlertRules(ctx, k8sService)
		}
	}
}

func evaluateAlertRules(ctx context.Context, k8sService *k8s.Service) {
	stats, err := computeAlertStats(ctx, k8sService)
	if err != nil {
		log.Printf("Error computing alert stats: %v", err)
		return
	}

	if alertsCollection == nil || historyCollection == nil {
		log.Println("MongoDB collections not initialized, skipping alert evaluation")
		return
	}

	// Get Rules
	cursor, err := alertsCollection.Find(ctx, bson.M{})
	if err != nil {
		log.Printf("Error fetching alert rules: %v", err)
		return
	}
	var rules []models.AlertRule
	if err = cursor.All(ctx, &rules); err != nil {
		log.Printf("Error decoding alert rules: %v", err)
		return
	}

	now := time.Now()

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		value, ok := metricValue(stats, rule.Metric)
		if !ok {
			continue
		}
		if !compareRule(value, rule.Comparison, rule.Threshold) {
			continue
		}

		// Check cooldown by finding the last event for this rule
		var lastEvent models.AlertEvent
		err := historyCollection.FindOne(ctx, bson.M{"ruleId": rule.ID}, options.FindOne().SetSort(bson.D{{Key: "triggeredAt", Value: -1}})).Decode(&lastEvent)

		shouldTrigger := false
		if err == mongo.ErrNoDocuments {
			shouldTrigger = true
		} else if err == nil {
			if now.Sub(lastEvent.TriggeredAt) >= alertEvaluationCooldown {
				shouldTrigger = true
			}
		}

		event := models.AlertEvent{
			ID:          uuid.NewString(),
			RuleID:      rule.ID,
			RuleName:    rule.Name,
			Severity:    rule.Severity,
			Message:     fmt.Sprintf("%s: %.2f %s %.2f", rule.Name, value, comparisonSymbol(rule.Comparison), rule.Threshold),
			TriggeredAt: now,
			Status:      "firing",
			Value:       value,
			Threshold:   rule.Threshold,
			Comparison:  rule.Comparison,
		}

		if shouldTrigger {
			_, err := historyCollection.InsertOne(ctx, event)
			if err != nil {
				log.Printf("Error saving alert history: %v", err)
			}

			// Send notifications
			for _, channel := range rule.Channels {
				if channel != "" {
					if strings.Contains(channel, "@") {
						go sendEmailNotification(channel, event)
					} else {
						// Webhook implementation can be added here
						log.Printf("Webhook notification not implemented yet for %s", channel)
					}
				}
			}
		}
	}
}

func computeAlertStats(ctx context.Context, k8sService *k8s.Service) (models.AlertStats, error) {
	if len(k8sService.Clients) == 0 {
		return models.AlertStats{}, fmt.Errorf("no clusters configured")
	}

	stats := models.AlertStats{}
	var totalCpuUsage, totalMemUsage int64
	var totalCpuCapacity, totalMemCapacity int64

	for _, clients := range k8sService.Clients {
		// Collect recommendations stats
		workloads := k8sService.CollectFromCluster(ctx, "", clients) // Cluster name empty if not needed for this
		for _, wl := range workloads {
			for _, rec := range wl.Recommendations {
				if strings.HasPrefix(rec, "Krytyczne:") {
					stats.CriticalRecommendations++
					break
				}
			}
		}

		// Collect cluster usage stats
		nodes, err := clients.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err == nil {
			for _, node := range nodes.Items {
				cpuCap := node.Status.Capacity.Cpu().MilliValue()
				memCap := node.Status.Capacity.Memory().Value()
				totalCpuCapacity += cpuCap
				totalMemCapacity += memCap
			}

			// For usage, we would ideally query Prometheus.
			// Since we don't have easy access to Prometheus query here without duplicating logic,
			// we will sum up workload usage as an approximation or use 0 if not available.
			for _, wl := range workloads {
				totalCpuUsage += int64(wl.AvgCpuUsage)
				totalMemUsage += int64(wl.AvgMemoryUsage)
			}
		}
	}

	if totalCpuCapacity > 0 {
		stats.ClusterCpuUsage = float64(totalCpuUsage) / float64(totalCpuCapacity) * 100
	}
	if totalMemCapacity > 0 {
		stats.ClusterMemoryUsage = float64(totalMemUsage) / float64(totalMemCapacity) * 100
	}

	return stats, nil
}

func metricValue(s models.AlertStats, metric string) (float64, bool) {
	switch metric {
	case "criticalRecommendations":
		return float64(s.CriticalRecommendations), true
	case "clusterCpuUsage":
		return s.ClusterCpuUsage, true
	case "clusterMemoryUsage":
		return s.ClusterMemoryUsage, true
	default:
		return 0, false
	}
}

func compareRule(value float64, comparison string, threshold float64) bool {
	switch comparison {
	case "gt":
		return value > threshold
	case "gte":
		return value >= threshold
	case "lt":
		return value < threshold
	case "lte":
		return value <= threshold
	case "eq":
		return value == threshold
	default:
		return false
	}
}

func comparisonSymbol(op string) string {
	switch op {
	case "gt":
		return ">"
	case "gte":
		return ">="
	case "lt":
		return "<"
	case "lte":
		return "<="
	case "eq":
		return "="
	default:
		return op
	}
}

func sendEmailNotification(to string, alert models.AlertEvent) {
	smtpHost := os.Getenv("SMTP_HOST")
	smtpPort := os.Getenv("SMTP_PORT")
	smtpUser := os.Getenv("SMTP_USER")
	smtpPass := os.Getenv("SMTP_PASS")

	if smtpHost == "" || smtpPort == "" {
		log.Printf("SMTP not configured. Skipping email to %s", to)
		return
	}

	from := smtpUser
	if from == "" {
		from = "alerts@kubernetes-dashboard.local"
	}

	subject := fmt.Sprintf("Alert: %s - %s", alert.Severity, alert.RuleName)
	body := fmt.Sprintf("Rule: %s\nSeverity: %s\nMessage: %s\nValue: %.2f\nThreshold: %.2f\nTime: %s",
		alert.RuleName, alert.Severity, alert.Message, alert.Value, alert.Threshold, alert.TriggeredAt.Format(time.RFC3339))

	msg := []byte(fmt.Sprintf("To: %s\r\n"+
		"Subject: %s\r\n"+
		"\r\n"+
		"%s\r\n", to, subject, body))

	var auth smtp.Auth
	if smtpUser != "" {
		auth = smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)
	}

	err := smtp.SendMail(smtpHost+":"+smtpPort, auth, from, []string{to}, msg)
	if err != nil {
		log.Printf("Error sending email to %s: %v", to, err)
		return
	}

	log.Printf("Email sent to %s for alert %s", to, alert.RuleName)
}

func GetAlertsOverview() (*models.AlertsOverview, error) {
	if alertsCollection == nil || historyCollection == nil {
		return nil, fmt.Errorf("MongoDB collections not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var rules []models.AlertRule
	cursor, err := alertsCollection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	if err = cursor.All(ctx, &rules); err != nil {
		return nil, err
	}

	// Get History (last 20)
	opts := options.Find().SetSort(bson.D{{Key: "triggeredAt", Value: -1}}).SetLimit(20)
	histCursor, err := historyCollection.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	var history []models.AlertEvent
	if err = histCursor.All(ctx, &history); err != nil {
		return nil, err
	}

	// Get Active alerts (fired within last 10 minutes and matching enabled rules)
	tenMinutesAgo := time.Now().Add(-10 * time.Minute)
	activeOpts := options.Find().SetSort(bson.D{{Key: "triggeredAt", Value: -1}})
	activeCursor, err := historyCollection.Find(ctx, bson.M{
		"triggeredAt": bson.M{"$gte": tenMinutesAgo},
		"status":      "firing",
	}, activeOpts)
	if err != nil {
		return nil, err
	}
	var activeEvents []models.AlertEvent
	if err = activeCursor.All(ctx, &activeEvents); err != nil {
		return nil, err
	}

	// Filter active events to only include those from enabled rules
	enabledRuleIDs := make(map[string]bool)
	for _, rule := range rules {
		if rule.Enabled {
			enabledRuleIDs[rule.ID] = true
		}
	}

	var active []models.AlertEvent
	for _, event := range activeEvents {
		if enabledRuleIDs[event.RuleID] {
			active = append(active, event)
		}
	}

	// Calculate basic stats from history
	criticalCount := 0
	for _, event := range history {
		if event.Severity == "critical" && time.Since(event.TriggeredAt) < 1*time.Hour {
			criticalCount++
		}
	}

	return &models.AlertsOverview{
		Rules:   rules,
		Active:  active,
		History: history,
		Stats: models.AlertStats{
			CriticalRecommendations: criticalCount,
			ClusterCpuUsage:         0,
			ClusterMemoryUsage:      0,
		},
	}, nil
}

func SaveAlertRules(rules []models.AlertRuleInput) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Delete all existing rules
	if _, err := alertsCollection.DeleteMany(ctx, bson.M{}); err != nil {
		return fmt.Errorf("failed to delete existing rules: %w", err)
	}

	if len(rules) == 0 {
		return nil
	}

	var documents []interface{}
	now := time.Now()
	for _, r := range rules {
		rule := models.AlertRule{
			ID:         r.ID,
			Name:       r.Name,
			Metric:     r.Metric,
			Comparison: r.Comparison,
			Threshold:  r.Threshold,
			Severity:   r.Severity,
			Window:     r.Window,
			Channels:   r.Channels,
			Enabled:    r.Enabled,
			CreatedAt:  now, // Ideally we should preserve this if it exists
			UpdatedAt:  now,
		}
		if rule.ID == "" {
			rule.ID = uuid.NewString()
		}
		documents = append(documents, rule)
	}

	_, err := alertsCollection.InsertMany(ctx, documents)
	return err
}
