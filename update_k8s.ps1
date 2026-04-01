

Write-Host "1. Budowanie nowego obrazu Backend..." -ForegroundColor Cyan
docker build -t k8s-manager-backend:latest ./backend

Write-Host "2. Budowanie nowego obrazu Frontend..." -ForegroundColor Cyan
docker build -t k8s-manager-frontend:latest ./frontend

Write-Host "3. Aktualizacja konfiguracji Kubernetes (YAML)..." -ForegroundColor Cyan
kubectl apply -f k8s/

Write-Host "4. Restartowanie aplikacji w klastrze..." -ForegroundColor Cyan
kubectl rollout restart deployment/backend
kubectl rollout restart deployment/frontend

Write-Host "Gotowe! Odśwież stronę za chwilę." -ForegroundColor Green
