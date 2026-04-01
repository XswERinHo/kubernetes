package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gorilla/mux"
	monitoringClient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"github.com/prometheus/client_golang/api"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"go.mongodb.org/mongo-driver/mongo"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	internalApi "kubernetes-manager/backend/internal/api"
	"kubernetes-manager/backend/internal/k8s"
	"kubernetes-manager/backend/internal/models"
	"kubernetes-manager/backend/internal/services"
)

func main() {
	inCluster := os.Getenv("IN_CLUSTER") == "true"

	clusterClients := make(map[string]*models.ClusterClients)

	if inCluster {
		log.Println("Running in in-cluster mode...")
		config, err := rest.InClusterConfig()
		if err != nil {
			log.Fatalf("Error getting in-cluster config: %v", err)
		}

		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			log.Fatalf("Error creating clientset: %v", err)
		}

		monitoringClientset, err := monitoringClient.NewForConfig(config)
		if err != nil {
			log.Fatalf("Error creating monitoringClientset: %v", err)
		}

		dynamicClient, err := dynamic.NewForConfig(config)
		if err != nil {
			log.Fatalf("Error creating dynamicClient: %v", err)
		}

		clusterClients["local"] = &models.ClusterClients{
			Clientset:           clientset,
			MonitoringClientset: monitoringClientset,
			DynamicClient:       dynamicClient,
		}
	} else {
		kubeconfigPath := filepath.Join(os.Getenv("USERPROFILE"), ".kube", "config")
		if _, err := os.Stat(kubeconfigPath); os.IsNotExist(err) {
			log.Fatalf("Kubeconfig file not found at: %s", kubeconfigPath)
		}

		config, err := clientcmd.LoadFromFile(kubeconfigPath)
		if err != nil {
			log.Fatalf("Error loading kubeconfig: %s", err.Error())
		}

		for contextName := range config.Contexts {
			log.Printf("Loading cluster context: %s", contextName)

			clientConfig := clientcmd.NewDefaultClientConfig(*config, &clientcmd.ConfigOverrides{CurrentContext: contextName})
			restConfig, err := clientConfig.ClientConfig()
			if err != nil {
				log.Printf("WARNING: Failed to load config for context %s: %v", contextName, err)
				continue
			}

			clientset, err := kubernetes.NewForConfig(restConfig)
			if err != nil {
				log.Printf("WARNING: Failed to create clientset for %s: %v", contextName, err)
				continue
			}

			monitoringClientset, err := monitoringClient.NewForConfig(restConfig)
			if err != nil {
				log.Printf("WARNING: Failed to create monitoringClientset for %s: %v", contextName, err)
				continue
			}

			dynamicClient, err := dynamic.NewForConfig(restConfig)
			if err != nil {
				log.Printf("WARNING: Failed to create dynamicClient for %s: %v", contextName, err)
				continue
			}

			clusterClients[contextName] = &models.ClusterClients{
				Clientset:           clientset,
				MonitoringClientset: monitoringClientset,
				DynamicClient:       dynamicClient,
			}
		}
	}

	if len(clusterClients) == 0 {
		log.Fatal("No valid cluster contexts loaded.")
	}

	promAddress := "http://localhost:30090"
	if inCluster {
		promAddress = os.Getenv("PROMETHEUS_URL")
		if promAddress == "" {
			promAddress = "http://prometheus-operated.monitoring.svc:9090"
		}
	}

	promClient, err := api.NewClient(api.Config{
		Address: promAddress,
	})
	if err != nil {
		log.Fatalf("Error creating Prometheus client: %v", err)
	}
	promAPI := prometheusv1.NewAPI(promClient)

	k8sService := k8s.NewService(clusterClients, promAPI)

	mongoURI := "mongodb://localhost:27017"
	if inCluster {
		mongoURI = os.Getenv("MONGO_URI")
		if mongoURI == "" {
			mongoURI = "mongodb://mongodb:27017"
		}
	}

	var db interface{}
	var mongoErr error
	maxRetries := 10
	for i := 0; i < maxRetries; i++ {
		db, mongoErr = services.InitMongoDB(mongoURI)
		if mongoErr == nil {
			log.Println("Successfully connected to MongoDB")
			if mongoDB, ok := db.(*mongo.Database); ok {
				services.InitApprovalsDB(mongoDB)
			}
			break
		}
		log.Printf("Attempt %d/%d: Failed to connect to MongoDB: %v. Retrying in 5s...", i+1, maxRetries, mongoErr)
		if i < maxRetries-1 {
			time.Sleep(5 * time.Second)
		}
	}
	if mongoErr != nil {
		log.Fatalf("FATAL: Could not connect to MongoDB after %d attempts. Exiting.", maxRetries)
	}

	go services.StartAlertEvaluator(context.Background(), k8sService)

	apiHandler := internalApi.New(k8sService)

	r := mux.NewRouter()
	r.Use(apiHandler.CORSMiddleware)
	apiHandler.SetupRoutes(r)

	port := "8080"
	fmt.Printf("Starting server on port %s...\n", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatal(err)
	}
}
