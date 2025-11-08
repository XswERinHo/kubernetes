# K8s Resource Manager

Aplikacja webowa do monitorowania, optymalizacji i zarządzania zasobami (CPU, RAM) oraz kosztami w klastrach Kubernetes.

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
* Widoczność kosztów per klaster, namespace, zasób, etykieta.
* Integracja z API chmury lub Kubecost/OpenCost.
* Kwantyfikacja finansowa rekomendacji (szacowane oszczędności).
* Możliwości alokacji kosztów (Showback/Chargeback).
* **Powiązanie z i18n:** Backend powinien obliczać koszty w jednej walucie bazowej. Frontend powinien być odpowiedzialny za **przewalutowanie** i formatowanie kwot w zależności od wybranego języka (np. PLN dla języka polskiego, USD dla angielskiego).

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

* ✅ **Dashboard FinOps:** Agregacja kosztów zużycia i żądań w skali miesiąca, wizualizacja kosztów per przestrzeń nazw (namespace).
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
    * Inteligentne formatowanie walut (PLN/USD) w zależności od wybranego języka.

---

## 🎯 Roadmapa Rozwoju (Wizja Produktu)

Celem jest stworzenie inteligentnego asystenta, który aktywnie pomaga w optymalizacji i zarządzaniu klastrami na dużą skalę.

### ➡️ Następne Priorytety (Poziom 2)

Kolejne funkcje planowane do implementacji:

#### 2.1 Role-Based Access Control (RBAC) ⭐⭐
* **Cel:** Bezpieczeństwo i podział uprawnień.
* **Backend:** Integracja z JWT/OAuth2. Endpointy `/api/auth/login`, `/api/auth/logout`. Middleware sprawdzające uprawnienia. Logi audytowe.
* **Frontend:** Ekran logowania. Warunkowe renderowanie UI na bazie ról:
    * **Admin:** Pełna kontrola.
    * **Editor (Moderator):** Może aplikować rekomendacje, ręczne edycje wymagają zatwierdzenia.
    * **Viewer:** Dostęp tylko do odczytu.

#### 2.2 Widok Węzłów (Nodes View) ⭐⭐
* **Cel:** Monitoring węzłów klastra i planowanie pojemności.
* **Backend:** Endpoint `/api/nodes` zwracający listę węzłów, ich metryki CPU/Memory/Disk, status, alokację podów, etykiety i tainty.
* **Frontend:** Dedykowana zakładka "Nodes". Tabela/Grid z węzłami (Capacity vs Allocatable vs Used). Drill-down do listy podów na węźle.

#### 2.3 Alokacja Kosztów (Cost Allocation) ⭐⭐
* **Cel:** Wdrożenie zasad FinOps – przypisywanie kosztów do zespołów/projektów.
* **Backend:** Endpoint `/api/costs/allocation` grupujący koszty po etykietach (`team`, `project`). Integracja z OpenCost/Kubecost API. Eksport raportów CSV.
* **Frontend:** Zakładka "Cost Allocation". Wykresy kosztów per team/project. Śledzenie budżetów.

#### 2.4 Alerty i Powiadomienia ⭐⭐
* **Cel:** Proaktywne powiadomienia o problemach.
* **Backend:** Konfiguracja reguł alertów (np. wysokie koszty, OOMKilled). Integracja z Slack/Email/Webhooks. Endpoint `/api/alerts` z historią.
* **Frontend:** Zakładka "Alerts". Konfiguracja kanałów powiadomień w Ustawieniach. Wizualne alerty na Dashboard.

---

## 🛠️ Stos Technologiczny

* **Backend:** **Go (Golang)**
    * Biblioteki: `k8s.io/client-go`, `prometheus/client_golang`, `prometheus-operator/pkg/client`
* **Frontend:** **React** (uruchamiany przez Vite)
    * Biblioteki: `Material-UI (MUI)`, `React Router`, `i18next`, `@mui/x-charts`
* **Monitoring:** **Prometheus** (jako źródło danych metryk)

---