package services

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"kubernetes-manager/backend/internal/models"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

var pendingChangesCollection *mongo.Collection

func InitApprovalsDB(db *mongo.Database) {
	pendingChangesCollection = db.Collection("pending_changes")
	log.Println("Initialized approvals DB collection: pending_changes")
}

func AddPendingChange(cluster, namespace, kind, name, username, role string, payload map[string]string) (models.PendingChange, error) {
	payloadCopy := make(map[string]string, len(payload))
	for k, v := range payload {
		payloadCopy[k] = v
	}

	change := models.PendingChange{
		ID:          uuid.NewString(),
		Cluster:     cluster,
		Namespace:   namespace,
		Kind:        kind,
		Name:        name,
		RequestedBy: username,
		Role:        role,
		RequestedAt: time.Now(),
		Payload:     payloadCopy,
		Status:      "pending",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := pendingChangesCollection.InsertOne(ctx, change)
	if err != nil {
		return models.PendingChange{}, err
	}

	return change, nil
}

func ListPendingChanges(cluster string, status string) []models.PendingChange {
	log.Printf("ListPendingChanges called: cluster=%s, status=%s, collection=%v", cluster, status, pendingChangesCollection)
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{}
	if cluster != "" {
		filter["cluster"] = cluster
	}
	if status != "" {
		filter["status"] = status
	}

	log.Printf("ListPendingChanges filter: %+v", filter)

	cursor, err := pendingChangesCollection.Find(ctx, filter)
	if err != nil {
		log.Printf("ListPendingChanges error: %v", err)
		return []models.PendingChange{}
	}
	defer cursor.Close(ctx)

	var results []models.PendingChange
	if err := cursor.All(ctx, &results); err != nil {
		log.Printf("ListPendingChanges decode error: %v", err)
		return []models.PendingChange{}
	}

	log.Printf("ListPendingChanges found %d results", len(results))
	return results
}

func GetPendingChange(id string) (models.PendingChange, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var change models.PendingChange
	err := pendingChangesCollection.FindOne(ctx, bson.M{"id": id}).Decode(&change)
	if err != nil {
		return models.PendingChange{}, false
	}

	return change, true
}

func UpdatePendingChange(change models.PendingChange) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := pendingChangesCollection.ReplaceOne(
		ctx,
		bson.M{"id": change.ID},
		change,
	)

	return err
}

func ToResourceUpdateRequest(pc models.PendingChange) *models.ResourceUpdateRequest {
	req := &models.ResourceUpdateRequest{}

	if val, ok := pc.Payload["cpuRequests"]; ok {
		v := val
		req.CpuRequests = &v
	}
	if val, ok := pc.Payload["cpuLimits"]; ok {
		v := val
		req.CpuLimits = &v
	}
	if val, ok := pc.Payload["memoryRequests"]; ok {
		v := val
		req.MemoryRequests = &v
	}
	if val, ok := pc.Payload["memoryLimits"]; ok {
		v := val
		req.MemoryLimits = &v
	}

	if val, ok := pc.Payload["containers_json"]; ok {
		var containers []models.ContainerResourceUpdate
		if err := json.Unmarshal([]byte(val), &containers); err == nil {
			req.Containers = containers
		}
	}

	return req
}
