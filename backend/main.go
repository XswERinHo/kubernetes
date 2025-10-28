package main

import (
	"context"
	"encoding/json" // Będziemy potrzebować tego do konwersji danych na JSON
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	// --- NOWE IMPORTY DLA KUBERNETESA ---
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	// --- KONIEC NOWYCH IMPORTÓW ---
)

// Definiujemy prostą strukturę, aby przechowywać tylko te dane,
// których na razie potrzebujemy w frontendzie.
type DeploymentInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// Globalna zmienna dla naszego klienta Kubernetes
var clientset *kubernetes.Clientset

func main() {
	// --- KONFIGURACJA POŁĄCZENIA Z KUBERNETES ---
	// Musimy znaleźć plik 'kubeconfig', którego 'kubectl' używa do łączenia się z klastrem.
	// Zazwyczaj znajduje się on w katalogu domowym użytkownika.
	kubeconfigPath := filepath.Join(os.Getenv("USERPROFILE"), ".kube", "config")

	// Budujemy konfigurację na podstawie tego pliku
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		log.Fatalf("Błąd budowania konfiguracji kubeconfig: %s", err.Error())
	}

	// Tworzymy "clientset" - to jest nasz główny obiekt do interakcji z API KubernetESA
	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Błąd tworzenia clientset: %s", err.Error())
	}
	// --- KONIEC KONFIGURACJI ---

	// --- DEFINICJA NASZYCH ENDPOINTÓW API ---
	// Zostawiamy /api/health do testów
	http.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "API is healthy!")
	})

	// NOWY ENDPOINT: Zwraca listę wdrożeń
	http.HandleFunc("/api/deployments", deploymentsHandler)
	// --- KONIEC DEFINICJI ---

	// --- URUCHOMIENIE SERWERA ---
	fmt.Println("Starting server on port 8080...")
	fmt.Println("Backend połączony z Kubernetesem. Dostępne endpointy:")
	fmt.Println("http://localhost:8080/api/health")
	fmt.Println("http://localhost:8080/api/deployments")

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

// deploymentsHandler to nowa funkcja, która obsługuje logikę dla /api/deployments
func deploymentsHandler(w http.ResponseWriter, r *http.Request) {
	// Używamy naszego globalnego clientset, aby pobrać listę wdrożeń
	// ze wszystkich przestrzeni nazw ("")
	deployments, err := clientset.AppsV1().Deployments("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		// Jeśli coś pójdzie nie tak, zwróć błąd
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Tworzymy pustą listę (slice) do przechowania naszych uproszczonych danych
	var deploymentInfos []DeploymentInfo

	// Przechodzimy pętlą po wszystkich znalezionych wdrożeniach
	for _, item := range deployments.Items {
		// i dodajemy tylko nazwę i namespace do naszej listy
		deploymentInfos = append(deploymentInfos, DeploymentInfo{
			Name:      item.Name,
			Namespace: item.Namespace,
		})
	}

	// Ustawiamy nagłówek, aby przeglądarka wiedziała, że odpowiedź to JSON
	w.Header().Set("Content-Type", "application/json")
	// Konwertujemy naszą listę na format JSON i wysyłamy ją jako odpowiedź
	json.NewEncoder(w).Encode(deploymentInfos)
}
