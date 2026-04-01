# K8s Resource Manager

Aplikacja webowa do monitorowania, optymalizacji i zarządzania zasobami (CPU, RAM) oraz kosztami w klastrach Kubernetes.

## Szybki Start (GitHub)

### Wymagania
- Git
- Docker Desktop (z włączonym Kubernetes)
- kubectl
- PowerShell (Windows)
- Helm (opcjonalnie, dla pełnych metryk Prometheus)

### 1. Klonowanie repo
```powershell
git clone <twoj-repo-url>
cd kubernetes-main
```

### 2. (Opcjonalnie) Instalacja Prometheus przez Helm
```powershell
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm install prometheus prometheus-community/kube-prometheus-stack -n monitoring --create-namespace
```

### 3. Weryfikacja sekretów
Plik `k8s/secrets.yaml` zawiera tylko przykładowe wartości. Przed użyciem produkcyjnym podmień:
- `smtp-user`
- `smtp-pass`
- `jwt-secret`

Wartości muszą być zakodowane Base64.

### 4. Build i deploy (automatyzacja)
```powershell
.\update_k8s.ps1
```

Skrypt automatycznie:
- buduje obraz backendu,
- buduje obraz frontendu,
- aplikuje manifesty Kubernetes z `k8s/`,
- restartuje deploymenty backend/frontend.

### 5. Dostęp do aplikacji
- Frontend: `http://localhost:30080`
- Domyślne konto (przy pustej bazie): `admin / password123`

### Uwaga o środowisku
Manifesty używają `imagePullPolicy: Never`, więc ta konfiguracja jest przygotowana pod lokalny klaster (np. Docker Desktop Kubernetes). Dla klastra chmurowego trzeba użyć registry obrazów i zmienić `image` + `imagePullPolicy`.

## Wizja Produktu (Docelowe Funkcje)

Celem jest stworzenie inteligentnego asystenta, który aktywnie pomaga w optymalizacji klastra. Kluczowe obszary rozwoju:

### 1. Pełny Wgląd i Kontekst (Widoczność) 📊
* Holistyczny dashboard klastra(ów).
* Możliwość drążenia danych (Klaster -> Namespace -> Zasób -> Pod -> Kontener).
* Dedykowany widok węzłów (Nodes).
* Wsparcie dla `Deployments`, `StatefulSets`, `DaemonSets`, `CronJobs`.

### 2. Inteligencja i Proaktywność (Rekomendacje) 🧠
* Rekomendacje `requests`/`limits` oparte na **danych historycznych** (integracja z Prometheus, np. 95. percentyl).
* Identyfikacja marnotrawstwa (zasoby "zombie", przewymiarowanie, niedowymiarowanie).
* Sugestie dotyczące konfiguracji autoskalowania (HPA/VPA).
* Wykrywanie anomalii w zużyciu zasobów.

### 3. Koncentracja na Kosztach (FinOps) 💲
* (Sekcja usunięta - projekt zmienił kierunek na zarządzanie operacyjne)

### 4. Ułatwione Zarządzanie i Działanie ⚙️
* Akcje "jednym kliknięciem" do stosowania rekomendacji.
* Możliwość edycji konfiguracji zasobów z poziomu UI.
* Zarządzanie politykami zasobów (Governance).
* (Zaawansowane) Integracja z CI/CD.

### 5. Doskonałe Doświadczenie Użytkownika (UX/UI) ✨
* Przejrzysty, nowoczesny interfejs (np. układ dashboardu zamiast tabeli).
* Zaawansowane filtrowanie i wyszukiwanie.
* Czytelne wykresy danych historycznych (wzbogacone o kontekst requests/limits).
* Konfigurowalne alerty i powiadomienia.
* **Internacjonalizacja (i18n):** Pełne wsparcie dla co najmniej dwóch języków (polski, angielski), włączając w to etykiety, tekst i formatowanie walut.
* **Tryby kolorystyczne:** Możliwość przełączania motywu (np. jasny / ciemny).
* Uwierzytelnianie i Role-Based Access Control (RBAC) z podziałem na role:
    * **Admin:** Pełna kontrola nad aplikacją, może edytować zasoby i zatwierdzać zmiany innych.
    * **Editor (Moderator):** Może aplikować automatyczne rekomendacje. Ręczne edycje wymagają zatwierdzenia przez Admina.
    * **Viewer:** Dostęp tylko do odczytu (np. dla programistów), pozwalający na podgląd zużycia i wykresów bez możliwości wprowadzania zmian.

### 6. Skalowalność i Wydajność 🚀
* Wsparcie dla zarządzania **wieloma klastrami** z jednego interfejsu.
* Wydajny backend (Go) radzący sobie z dużą ilością danych.
* Zoptymalizowane zapytania do API K8s i Prometheus.

---

*(Tutaj można dodać informacje o tym, jak uruchomić projekt, technologiach itp.)*


# K8s Resource Manager

Aplikacja webowa (Go + React) do monitorowania, optymalizacji i zarządzania zasobami (CPU, RAM) oraz kosztami w klastrach Kubernetes.

## 🚀 Kluczowe Funkcje (Stan Obecny)

To, co jest już zaimplementowane i działa:

* ✅ **Dashboard Operacyjny:** Agregacja liczby workloadów, przestrzeni nazw i aktywnych alertów.
* ✅ **Wsparcie dla Wielu Klastrów:** Dynamiczne ładowanie wszystkich kontekstów z lokalnego pliku `kubeconfig` i selektor klastrów w UI.
* ✅ **Zarządzanie Zasobami (Workloads):** Przeglądanie `Deployments`, `StatefulSets`, `DaemonSets` i `CronJobs`. Możliwość ręcznej edycji `requests` i `limits`.
* ✅ **Zaawansowane Rekomendacje:** Silnik rekomendacji w backendzie, który wykrywa:
    * Zasoby przewymiarowane (sugerując `downsizing` na bazie 7-dniowego p95).
    * Zasoby "Zombie" (brak zużycia CPU/RAM).
    * Historyczne błędy `OOMKilled`.
* ✅ **Monitoring Systemu:** Panel statusu "Health Check" monitorujący na żywo połączenie z API Kubernetesa i API Prometheusa dla wybranego klastra.
* ✅ **Wizualizacja Metryk:** Historyczne wykresy zużycia CPU i Pamięci dla każdego zasobu, z widocznymi liniami `request` i `limit`.
* ✅ **Nowoczesny UI/UX:**
    * Motyw Ciemny / Jasny (przełączany w ustawieniach).
    * Pełna internacjonalizacja (i18n) dla języka polskiego (PL) i angielskiego (EN).
* ✅ **Alerty i powiadomienia:** Backendowy silnik reguł (/api/alerts), UI do konfiguracji progów, statystyki i kafelki alertów na dashboardzie. Integracja z Webhookami (np. Slack).
* ✅ **RBAC i Zarządzanie Użytkownikami:**
    * Trwałe przechowywanie użytkowników w MongoDB.
    * Role: Admin, Editor, Viewer.
    * Panel administratora do zarządzania użytkownikami.

---

## 🎯 Roadmapa Rozwoju (Wizja Produktu)

Celem jest stworzenie inteligentnego asystenta, który aktywnie pomaga w optymalizacji i zarządzaniu klastrami na dużą skalę.

### ➡️ Następne Priorytety (Poziom 2)

Kolejne funkcje planowane do implementacji:

#### 2.2 Widok Węzłów (Nodes View) ⭐⭐
* **Cel:** Monitoring węzłów klastra i planowanie pojemności.
* **Backend:** Endpoint `/api/nodes` zwracający listę węzłów, ich metryki CPU/Memory/Disk, status, alokację podów, etykiety i tainty.
* **Frontend:** Dedykowana zakładka "Nodes". Tabela/Grid z węzłami (Capacity vs Allocatable vs Used). Drill-down do listy podów na węźle.

#### 2.3 Zaawansowana Alokacja Kosztów (Showback/Chargeback) ⭐⭐
* (Sekcja usunięta)

#### 2.4 Rozszerzone Integracje Alertów ⭐⭐
* **Cel:** Dalsza rozbudowa powiadomień.
* **Backend:** Integracje Email, scheduler do batchowania, eskalacje per priorytet.
* **Frontend:** Szablony komunikatów, możliwość testowego wysłania alertu.

#### 2.3 Zaawansowana Alokacja Kosztów (Showback/Chargeback) ⭐⭐
* **Cel:** Rozszerzyć obecną zakładkę Cost Allocation o budżety per zespół/projekt i eksporty rozliczeń.
* **Backend:** Grupy kosztów po etykietach (`team`, `project`, `env`), integracja z OpenCost/Kubecost, generowanie raportów CSV/JSON.
* **Frontend:** Monitorowanie realizacji budżetów, alerty progów finansowych, filtrowanie po etykietach i okresach.

#### 2.4 Integracje Alertów i automatyczne kanały ⭐⭐
* **Cel:** Wykorzystać istniejący silnik reguł do wysyłki powiadomień poza UI.
* **Backend:** Integracje Webhook/Slack/Email, scheduler do batchowania, eskalacje per priorytet.
* **Frontend:** Ustawienia kanałów, szablony komunikatów, możliwość testowego wysłania alertu.

---

## 🛠️ Stos Technologiczny

* **Backend:** **Go (Golang)**
    * Biblioteki: `k8s.io/client-go`, `prometheus/client_golang`, `prometheus-operator/pkg/client`
* **Frontend:** **React** (uruchamiany przez Vite)
    * Biblioteki: `Material-UI (MUI)`, `React Router`, `i18next`, `@mui/x-charts`
* **Monitoring:** **Prometheus** (jako źródło danych metryk)

---