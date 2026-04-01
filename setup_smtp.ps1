# SMTP Configuration Script
# Replace the placeholder values with your actual credentials before running
# WARNING: Do NOT commit credentials to git - this file should be added to .gitignore

$hostName = "smtp.poczta.onet.pl"
$port = "587"
$user = "your-email@op.pl"              # Replace with your email
$pass = 'your-password-here'            # Replace with your password

# --- Do not modify below this line ---

Write-Host "Configuring SMTP for: $user on server $hostName..." -ForegroundColor Cyan

$userBytes = [System.Text.Encoding]::UTF8.GetBytes($user)
$userB64 = [System.Convert]::ToBase64String($userBytes)

$passBytes = [System.Text.Encoding]::UTF8.GetBytes($pass)
$passB64 = [System.Convert]::ToBase64String($passBytes)

# Update backend.yaml
$backendFile = "k8s/backend.yaml"
$backendContent = Get-Content $backendFile -Raw
$backendContent = $backendContent -replace '(?ms)(name: SMTP_HOST\s+value: )".*?"', "`$1`"$hostName`""
$backendContent = $backendContent -replace '(?ms)(name: SMTP_PORT\s+value: )"\d+"', "`$1`"$port`""
Set-Content $backendFile $backendContent

# Update secrets.yaml
$secretsFile = "k8s/secrets.yaml"
$secretsContent = Get-Content $secretsFile -Raw

function Update-Secret ($content, $key, $newValue) {
    if ($content -match "# ${key}:") {
        return $content -replace "# ${key}: .*", "${key}: $newValue"
    } elseif ($content -match "${key}:") {
        return $content -replace "${key}: .*", "${key}: $newValue"
    }
    return $content
}

$secretsContent = Update-Secret $secretsContent "smtp-user" $userB64
$secretsContent = Update-Secret $secretsContent "smtp-pass" $passB64

Set-Content $secretsFile $secretsContent

Write-Host "Configuration updated successfully!" -ForegroundColor Green
Write-Host "Host: $hostName"
Write-Host "User: $user"
Write-Host "Password has been encoded and saved in k8s/secrets.yaml"
Write-Host "Run: .\update_k8s.ps1 to deploy changes" -ForegroundColor Cyan
