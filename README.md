# K8s Resource Manager (Sentinel)

Aplikacja webowa (Go + React) do monitorowania klastra Kubernetes, optymalizacji zasobow CPU/RAM oraz bezpiecznego wprowadzania zmian przez approval workflow.

## Co to robi

- Dashboard operacyjny dla wybranego klastra
- Monitoring workloads: Deployments, StatefulSets, DaemonSets, CronJobs
- Rekomendacje optymalizacji zasobow na podstawie metryk
- Edycja requests/limits z mechanizmem zatwierdzen
- Alerty i historia alertow
- RBAC aplikacyjny (Admin, Editor, Viewer)
- i18n (PL/EN) i tryb jasny/ciemny

## Architektura

Aplikacja dziala in-cluster jako trzy glowne komponenty:

- Frontend (React + Nginx)
- Backend (Go)
- MongoDB

Backend integruje sie z:

- Kubernetes API (odczyt i patch zasobow)
- Prometheus (metryki historyczne)
- SMTP (powiadomienia email, opcjonalnie)

## Szybki Start (lokalnie, Docker Desktop Kubernetes)

### Wymagania

- Git
- Docker Desktop z wlaczonym Kubernetes
- kubectl
- PowerShell (Windows)
- Helm (opcjonalnie, dla pelnych metryk Prometheus)

### 1. Klonowanie

- git clone <twoj-repo-url>
- cd kubernetes-main

### 2. Opcjonalnie: Prometheus przez Helm

- helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
- helm install prometheus prometheus-community/kube-prometheus-stack -n monitoring --create-namespace

### 3. Weryfikacja sekretow

Plik k8s/secrets.yaml zawiera przykladowe wartosci. Przed produkcja podmien:

- smtp-user
- smtp-pass
- jwt-secret

Wartosci musza byc zapisane jako Base64.

### 4. Build i deploy (automatyzacja)

- .\update_k8s.ps1

Skrypt:

- buduje obraz backendu
- buduje obraz frontendu
- aplikuje manifesty z katalogu k8s
- restartuje deploymenty backend/frontend

### 5. Dostep

- Frontend: http://localhost:30080
- Domyslne konto przy pustej bazie: admin / password123

## Wdrozenie i pliki Kubernetes

Projekt korzysta z natywnych manifestow YAML (bez chartu Helm dla samej aplikacji):

- k8s/secrets.yaml
- k8s/mongodb.yaml
- k8s/backend.yaml
- k8s/frontend.yaml

Uwaga: manifesty maja imagePullPolicy: Never, wiec domyslnie sa przygotowane pod lokalny klaster i lokalnie zbudowane obrazy.

Dla klastra chmurowego:

- wypchnij obrazy do registry (np. GHCR/Docker Hub)
- zmien image w deploymentach
- zmien imagePullPolicy na IfNotPresent lub Always

## Zakres funkcjonalny

- Zarzadzanie zasobami workloads przez modyfikacje:
  - resources.requests.cpu
  - resources.requests.memory
  - resources.limits.cpu
  - resources.limits.memory
- Workflow zatwierdzen:
  - Editor/Operator inicjuje zmiane
  - Admin zatwierdza lub odrzuca
  - Backend wykonuje patch przez Kubernetes API

## Stos technologiczny

- Backend: Go, client-go, gorilla/mux, mongo-driver, prom client, JWT, bcrypt
- Frontend: React, Vite, MUI, React Router, i18next
- Baza danych: MongoDB
- Monitoring: Prometheus
- Orkiestracja: Kubernetes

## Bezpieczenstwo

- Hasla przechowywane jako hash bcrypt
- Tokeny JWT
- Role i autoryzacja endpointow
- Sekrety przez Kubernetes Secret

## Rozwoj

Najblizsze kierunki rozwoju:

- dalsza rozbudowa alertow i kanalow powiadomien
- rozszerzenia multi-cluster
- integracje SSO/LDAP
- bardziej zaawansowane rekomendacje
