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