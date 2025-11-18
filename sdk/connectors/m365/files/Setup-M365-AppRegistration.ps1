#Requires -Version 5.1
<#
.SYNOPSIS
    Script automatisé pour configurer le connecteur Microsoft 365 dans Azure AD
    
.DESCRIPTION
    Ce script automatise la création d'une App Registration, l'ajout des permissions,
    la création d'un secret client et la récupération des informations de configuration
    pour le connecteur Microsoft 365.
    
.PARAMETER AppName
    Nom de l'application à créer (par défaut: "Microsoft 365 Connector Glimps")
    
.PARAMETER SecretDescription
    Description du secret client (par défaut: "m365 connector glimps")
    
.PARAMETER SecretExpirationMonths
    Durée de validité du secret en mois (par défaut: 12)
    
.EXAMPLE
    .\Setup-M365Connector.ps1
    
.EXAMPLE
    .\Setup-M365Connector.ps1 -AppName "Mon Connecteur M365" -SecretExpirationMonths 6
#>

[CmdletBinding()]
param(
    [Parameter(Mandatory = $false)]
    [string]$AppName = "Microsoft 365 Connector Glimps",
    
    [Parameter(Mandatory = $false)]
    [string]$SecretDescription = "m365 connector glimps",
    
    [Parameter(Mandatory = $false)]
    [ValidateRange(1, 24)]
    [int]$SecretExpirationMonths = 12
)

# Couleurs pour l'affichage
$InfoColor = "Cyan"
$SuccessColor = "Green"
$WarningColor = "Yellow"
$ErrorColor = "Red"

function Write-Info {
    param([string]$Message)
    Write-Host "ℹ️  $Message" -ForegroundColor $InfoColor
}

function Write-Success {
    param([string]$Message)
    Write-Host "✅ $Message" -ForegroundColor $SuccessColor
}

function Write-Warning {
    param([string]$Message)
    Write-Host "⚠️  $Message" -ForegroundColor $WarningColor
}

function Write-Error {
    param([string]$Message)
    Write-Host "❌ $Message" -ForegroundColor $ErrorColor
}

function Test-Prerequisites {
    Write-Host "🔍 Vérification des prérequis pour le script M365 Connector" -ForegroundColor $InfoColor
    Write-Host "=" * 65 -ForegroundColor $InfoColor
    
    $issues = @()
    $warnings = @()
    
    # 1. Vérification de la version PowerShell
    Write-Host "`n1. Version PowerShell:" -ForegroundColor $WarningColor
    $psVersion = $PSVersionTable.PSVersion
    Write-Host "   Version détectée: $psVersion" -ForegroundColor White
    
    if ($psVersion.Major -ge 5) {
        Write-Success "   Version PowerShell compatible"
    } else {
        Write-Error "   Version PowerShell trop ancienne (5.1+ requis)"
        $issues += "PowerShell version $psVersion trop ancienne"
    }
    
    # 2. Vérification de la politique d'exécution
    Write-Host "`n2. Politique d'exécution:" -ForegroundColor $WarningColor
    $executionPolicy = Get-ExecutionPolicy
    Write-Host "   Politique actuelle: $executionPolicy" -ForegroundColor White
    
    $allowedPolicies = @("RemoteSigned", "Unrestricted", "Bypass")
    if ($executionPolicy -in $allowedPolicies) {
        Write-Success "   Politique d'exécution compatible"
    } else {
        Write-Warning "   Politique d'exécution restrictive"
        Write-Host "   💡 Tentative d'ajustement automatique..." -ForegroundColor $InfoColor
        
        try {
            Set-ExecutionPolicy RemoteSigned -Scope CurrentUser -Force
            Write-Success "   Politique d'exécution ajustée avec succès"
        } catch {
            Write-Error "   Impossible d'ajuster la politique d'exécution"
            $issues += "Politique d'exécution restrictive ($executionPolicy)"
        }
    }
    
    # 3. Vérification des modules Microsoft Graph
    Write-Host "`n3. Modules Microsoft Graph:" -ForegroundColor $WarningColor
    
    $requiredModules = @(
        "Microsoft.Graph.Authentication",
        "Microsoft.Graph.Applications"
    )
    
    foreach ($moduleName in $requiredModules) {
        $module = Get-Module -ListAvailable -Name $moduleName | Select-Object -First 1
        if ($module) {
            Write-Success "   $moduleName (v$($module.Version)) - Installé"
        } else {
            Write-Warning "   $moduleName - Non installé (sera installé automatiquement)"
            $warnings += "$moduleName sera installé automatiquement"
        }
    }
    
    # 4. Test de connectivité
    Write-Host "`n4. Connectivité réseau:" -ForegroundColor $WarningColor
    
    $endpoints = @(
        @{Name = "PowerShell Gallery"; Url = "https://www.powershellgallery.com"; Method = "Head"},
        @{Name = "Microsoft Graph"; Url = "https://graph.microsoft.com/v1.0"; Method = "Get"},
        @{Name = "Azure AD Login"; Url = "https://login.microsoftonline.com"; Method = "Head"}
    )
    
    foreach ($endpoint in $endpoints) {
        try {
            if ($endpoint.Method -eq "Get") {
                $response = Invoke-WebRequest -Uri $endpoint.Url -Method Get -TimeoutSec 10 -UseBasicParsing -ErrorAction Stop
                # Pour Microsoft Graph, un code 401 (Unauthorized) est acceptable car cela signifie que le service répond
                if ($response.StatusCode -eq 200 -or $response.StatusCode -eq 401) {
                    Write-Success "   $($endpoint.Name) - Accessible"
                } else {
                    throw "Code de statut inattendu: $($response.StatusCode)"
                }
            } else {
                $null = Invoke-WebRequest -Uri $endpoint.Url -Method Head -TimeoutSec 10 -UseBasicParsing -ErrorAction Stop
                Write-Success "   $($endpoint.Name) - Accessible"
            }
        } catch {
            # Pour Microsoft Graph, les erreurs 401 (Unauthorized) ou 405 (Method Not Allowed) sont acceptables
            if ($endpoint.Name -eq "Microsoft Graph" -and ($_.Exception.Response.StatusCode -eq 401 -or $_.Exception.Response.StatusCode -eq 405)) {
                Write-Success "   $($endpoint.Name) - Accessible (authentification requise)"
            } else {
                Write-Error "   $($endpoint.Name) - Non accessible"
                Write-Host "      Détail: $($_.Exception.Message)" -ForegroundColor Red
                $issues += "Connectivité vers $($endpoint.Name) échouée"
            }
        }
    }
    
    # 5. Vérification de l'environnement utilisateur
    Write-Host "`n5. Environnement utilisateur:" -ForegroundColor $WarningColor
    
    try {
        # Vérification si on est sur Windows et si les APIs Windows sont disponibles
        if ($PSVersionTable.Platform -eq "Win32NT" -or [System.Environment]::OSVersion.Platform -eq "Win32NT" -or $IsWindows) {
            try {
                $currentUser = [System.Security.Principal.WindowsIdentity]::GetCurrent()
                $principal = New-Object System.Security.Principal.WindowsPrincipal($currentUser)
                $isAdmin = $principal.IsInRole([System.Security.Principal.WindowsBuiltInRole]::Administrator)
                
                Write-Host "   Utilisateur: $($currentUser.Name)" -ForegroundColor White
                if ($isAdmin) {
                    Write-Success "   Exécution en tant qu'administrateur"
                } else {
                    Write-Info "   Exécution en tant qu'utilisateur (suffisant)"
                }
            }
            catch {
                # Fallback pour les environnements Windows restreints
                Write-Host "   Utilisateur: $($env:USERNAME)" -ForegroundColor White
                Write-Info "   Impossible de déterminer les privilèges administrateur"
            }
        }
        else {
            # Pour les systèmes non-Windows (Linux, macOS)
            $currentUser = $env:USER
            if (-not $currentUser) { $currentUser = whoami 2>$null }
            if (-not $currentUser) { $currentUser = "Utilisateur inconnu" }
            
            Write-Host "   Utilisateur: $currentUser" -ForegroundColor White
            Write-Info "   Plateforme: $($PSVersionTable.Platform -or 'Non-Windows')"
            Write-Info "   Vérification des privilèges non applicable sur cette plateforme"
        }
    }
    catch {
        # Fallback ultime
        $userName = $env:USERNAME -or $env:USER -or "Utilisateur inconnu"
        Write-Host "   Utilisateur: $userName" -ForegroundColor White
        Write-Warning "   Impossible de déterminer l'environnement d'exécution"
        Write-Info "   Cela n'affectera pas le fonctionnement du script"
    }
    
    # Résumé final
    Write-Host "`n" -NoNewline
    Write-Host "📋 RÉSUMÉ DES PRÉREQUIS" -ForegroundColor $InfoColor
    Write-Host "=" * 25 -ForegroundColor $InfoColor
    
    if ($issues.Count -eq 0) {
        Write-Success "🎉 Tous les prérequis critiques sont satisfaits !"
        
        if ($warnings.Count -gt 0) {
            Write-Host "`nAvertissements:" -ForegroundColor $WarningColor
            foreach ($warning in $warnings) {
                Write-Host "   • $warning" -ForegroundColor $WarningColor
            }
        }
        
        Write-Host "`n💡 Le script va maintenant procéder à la configuration..." -ForegroundColor $InfoColor
        Start-Sleep -Seconds 2
        return $true
    } else {
        Write-Error "❌ Problèmes critiques détectés:"
        foreach ($issue in $issues) {
            Write-Host "   • $issue" -ForegroundColor $ErrorColor
        }
        Write-Host "`n🛠️  Corrigez ces problèmes avant de relancer le script" -ForegroundColor $ErrorColor
        return $false
    }
}

function Test-RequiredModules {
    Write-Info "Vérification des modules PowerShell requis..."
    
    $requiredModules = @(
        @{Name = "Microsoft.Graph.Authentication"; MinVersion = "1.0.0"},
        @{Name = "Microsoft.Graph.Applications"; MinVersion = "1.0.0"}
    )
    
    $missingModules = @()
    $conflictModules = @()
    
    # Nettoyer les modules Graph éventuellement chargés pour éviter les conflits
    Write-Info "Nettoyage des modules Microsoft Graph existants..."
    try {
        $loadedGraphModules = Get-Module Microsoft.Graph.* | Where-Object { $_.Name -like "Microsoft.Graph.*" }
        if ($loadedGraphModules) {
            Write-Warning "Modules Microsoft Graph détectés en mémoire, nettoyage en cours..."
            $loadedGraphModules | Remove-Module -Force -ErrorAction SilentlyContinue
            Write-Success "Modules existants supprimés de la session"
        }
    }
    catch {
        Write-Warning "Impossible de nettoyer complètement les modules existants: $($_.Exception.Message)"
    }
    
    foreach ($module in $requiredModules) {
        Write-Info "Vérification du module: $($module.Name)"
        
        # Recherche des versions installées
        $installedModules = Get-Module -ListAvailable -Name $module.Name | Sort-Object Version -Descending
        
        if ($installedModules) {
            $latestModule = $installedModules | Select-Object -First 1
            Write-Info "Version installée trouvée: $($latestModule.Version)"
            
            if ($latestModule.Version -ge $module.MinVersion) {
                Write-Success "Module $($module.Name) (v$($latestModule.Version)) - Compatible"
                
                # Vérifier s'il y a plusieurs versions installées
                if ($installedModules.Count -gt 1) {
                    Write-Warning "Plusieurs versions détectées pour $($module.Name)"
                    $conflictModules += @{
                        Name = $module.Name
                        LatestVersion = $latestModule.Version
                        AllVersions = $installedModules.Version
                    }
                }
            } else {
                Write-Warning "Version trop ancienne pour $($module.Name) (installée: $($latestModule.Version), requise: $($module.MinVersion))"
                $missingModules += $module.Name
            }
        } else {
            Write-Warning "Module $($module.Name) non trouvé"
            $missingModules += $module.Name
        }
    }
    
    # Gestion des conflits de versions
    if ($conflictModules.Count -gt 0) {
        Write-Warning "Conflits de versions détectés. Nettoyage recommandé..."
        foreach ($conflict in $conflictModules) {
            Write-Host "   Module: $($conflict.Name)" -ForegroundColor Yellow
            Write-Host "   Versions installées: $($conflict.AllVersions -join ', ')" -ForegroundColor Yellow
        }
        
        $cleanupChoice = Read-Host "Voulez-vous nettoyer les anciennes versions? (O/N) [Recommandé: O]"
        if ($cleanupChoice -eq "O" -or $cleanupChoice -eq "o" -or $cleanupChoice -eq "") {
            foreach ($conflict in $conflictModules) {
                Write-Info "Nettoyage des anciennes versions de $($conflict.Name)..."
                try {
                    # Garder seulement la version la plus récente
                    $oldVersions = Get-Module -ListAvailable -Name $conflict.Name | 
                                  Where-Object { $_.Version -ne $conflict.LatestVersion }
                    
                    foreach ($oldVersion in $oldVersions) {
                        Write-Info "Suppression de $($conflict.Name) v$($oldVersion.Version)..."
                        Uninstall-Module -Name $conflict.Name -RequiredVersion $oldVersion.Version -Force -ErrorAction SilentlyContinue
                    }
                    Write-Success "Anciennes versions de $($conflict.Name) supprimées"
                }
                catch {
                    Write-Warning "Impossible de supprimer toutes les anciennes versions de $($conflict.Name): $($_.Exception.Message)"
                }
            }
        }
    }
    
    # Installation des modules manquants
    if ($missingModules.Count -gt 0) {
        Write-Warning "Modules manquants détectés. Installation en cours..."
        foreach ($moduleName in $missingModules) {
            Write-Info "Installation du module: $moduleName"
            try {
                # Désinstaller toutes les versions existantes d'abord si nécessaire
                $existingVersions = Get-Module -ListAvailable -Name $moduleName
                if ($existingVersions) {
                    Write-Info "Suppression des versions existantes de $moduleName..."
                    $existingVersions | ForEach-Object {
                        Uninstall-Module -Name $moduleName -RequiredVersion $_.Version -Force -ErrorAction SilentlyContinue
                    }
                }
                
                # Installation de la version la plus récente
                Install-Module -Name $moduleName -Scope CurrentUser -Force -AllowClobber -SkipPublisherCheck
                Write-Success "Module $moduleName installé avec succès"
            }
            catch {
                Write-Error "Impossible d'installer le module $moduleName : $($_.Exception.Message)"
                
                # Tentative alternative avec -Force et -AllowPrerelease
                try {
                    Write-Info "Tentative d'installation alternative pour $moduleName..."
                    Install-Module -Name $moduleName -Scope CurrentUser -Force -AllowClobber -SkipPublisherCheck -AllowPrerelease -ErrorAction Stop
                    Write-Success "Module $moduleName installé avec succès (version alternative)"
                }
                catch {
                    Write-Error "Échec définitif de l'installation de $moduleName : $($_.Exception.Message)"
                    exit 1
                }
            }
        }
    }
    
    # Import des modules avec gestion des conflits
    Write-Info "Chargement des modules Microsoft Graph..."
    try {
        # Forcer la suppression de tous les modules Graph avant import
        Get-Module Microsoft.Graph.* | Remove-Module -Force -ErrorAction SilentlyContinue
        
        # Import avec gestion explicite des versions
        foreach ($module in $requiredModules) {
            $latestVersion = Get-Module -ListAvailable -Name $module.Name | 
                           Sort-Object Version -Descending | 
                           Select-Object -First 1
            
            if ($latestVersion) {
                Write-Info "Import de $($module.Name) v$($latestVersion.Version)..."
                Import-Module -Name $module.Name -RequiredVersion $latestVersion.Version -Force -Global
                Write-Success "Module $($module.Name) chargé avec succès"
            }
        }
    }
    catch {
        Write-Error "Erreur lors du chargement des modules: $($_.Exception.Message)"
        
        # Tentative de chargement sans version spécifique
        Write-Info "Tentative de chargement alternatif..."
        try {
            Import-Module Microsoft.Graph.Authentication -Force -Global
            Import-Module Microsoft.Graph.Applications -Force -Global
            Write-Success "Modules chargés avec succès (méthode alternative)"
        }
        catch {
            Write-Error "Impossible de charger les modules Microsoft Graph: $($_.Exception.Message)"
            Write-Error "Veuillez redémarrer PowerShell et relancer le script"
            exit 1
        }
    }
    
    Write-Success "Tous les modules requis sont disponibles et chargés"
}

function Connect-ToMicrosoftGraph {
    Write-Info "Connexion à Microsoft Graph..."
    
    try {
        # Déconnexion si déjà connecté
        Disconnect-MgGraph -ErrorAction SilentlyContinue
        
        # Connexion avec les scopes nécessaires
        Connect-MgGraph -Scopes "Application.ReadWrite.All", "AppRoleAssignment.ReadWrite.All", "Directory.Read.All" -NoWelcome
        
        $context = Get-MgContext
        if ($context) {
            Write-Success "Connecté à Microsoft Graph"
            Write-Info "Tenant: $($context.TenantId)"
            Write-Info "Utilisateur: $($context.Account)"
        }
        else {
            throw "Impossible de récupérer le contexte de connexion"
        }
    }
    catch {
        Write-Error "Échec de la connexion à Microsoft Graph: $_"
        exit 1
    }
}

function New-AppRegistration {
    param([string]$DisplayName)
    
    Write-Info "Création de l'App Registration: $DisplayName"
    
    try {
        # Vérifier si l'application existe déjà
        $existingApp = Get-MgApplication -Filter "displayName eq '$DisplayName'" -ErrorAction SilentlyContinue
        
        if ($existingApp) {
            Write-Warning "Une application avec le nom '$DisplayName' existe déjà"
            $response = Read-Host "Voulez-vous la supprimer et en créer une nouvelle? (O/N)"
            if ($response -eq "O" -or $response -eq "o") {
                Remove-MgApplication -ApplicationId $existingApp.Id
                Write-Success "Application existante supprimée"
            }
            else {
                Write-Info "Utilisation de l'application existante"
                return $existingApp
            }
        }
        
        # Création de la nouvelle application
        $appParams = @{
            DisplayName = $DisplayName
            SignInAudience = "AzureADMyOrg"
            Description = "Connecteur pour accéder aux données Microsoft 365"
        }
        
        $app = New-MgApplication @appParams
        Write-Success "App Registration créée avec succès"
        Write-Info "Application ID: $($app.AppId)"
        
        return $app
    }
    catch {
        Write-Error "Erreur lors de la création de l'App Registration: $_"
        exit 1
    }
}

function Set-ApiPermissions {
    param(
        [string]$ApplicationId,
        [array]$Permissions
    )
    
    Write-Info "Configuration des permissions API..."
    
    try {
        # Microsoft Graph Service Principal ID
        $graphServicePrincipal = Get-MgServicePrincipal -Filter "appId eq '00000003-0000-0000-c000-000000000000'"
        
        if (-not $graphServicePrincipal) {
            throw "Impossible de trouver le Service Principal Microsoft Graph"
        }
        
        # Récupération des rôles d'application Microsoft Graph
        $graphAppRoles = $graphServicePrincipal.AppRoles
        
        # Construction de la liste des permissions requises
        $requiredResourceAccess = @()
        $resourceAccess = @()
        
        foreach ($permission in $Permissions) {
            $appRole = $graphAppRoles | Where-Object { $_.Value -eq $permission }
            if ($appRole) {
                $resourceAccess += @{
                    Id = $appRole.Id
                    Type = "Role"  # Application permission
                }
                Write-Info "Permission ajoutée: $permission"
            }
            else {
                Write-Warning "Permission non trouvée: $permission"
            }
        }
        
        if ($resourceAccess.Count -gt 0) {
            $requiredResourceAccess += @{
                ResourceAppId = "00000003-0000-0000-c000-000000000000"  # Microsoft Graph
                ResourceAccess = $resourceAccess
            }
            
            # Mise à jour de l'application avec les permissions
            Update-MgApplication -ApplicationId $ApplicationId -RequiredResourceAccess $requiredResourceAccess
            Write-Success "Permissions API configurées avec succès"
        }
        
        return $resourceAccess
    }
    catch {
        Write-Error "Erreur lors de la configuration des permissions: $_"
        exit 1
    }
}

function Grant-AdminConsent {
    param(
        [string]$ApplicationId,
        [string]$ServicePrincipalId,
        [array]$ResourceAccess
    )
    
    Write-Info "Attribution du consentement administrateur..."
    
    try {
        # Récupération du Service Principal Microsoft Graph
        $graphServicePrincipal = Get-MgServicePrincipal -Filter "appId eq '00000003-0000-0000-c000-000000000000'"
        
        foreach ($access in $ResourceAccess) {
            try {
                $params = @{
                    PrincipalId = $ServicePrincipalId
                    ResourceId = $graphServicePrincipal.Id
                    AppRoleId = $access.Id
                }
                
                New-MgServicePrincipalAppRoleAssignment -ServicePrincipalId $ServicePrincipalId @params -ErrorAction SilentlyContinue
            }
            catch {
                # Ignorer si la permission est déjà accordée
                if ($_.Exception.Message -notlike "*already exists*") {
                    Write-Warning "Impossible d'accorder la permission: $($_.Exception.Message)"
                }
            }
        }
        
        Write-Success "Consentement administrateur accordé"
    }
    catch {
        Write-Error "Erreur lors de l'attribution du consentement: $_"
        exit 1
    }
}

function New-ClientSecret {
    param(
        [string]$ApplicationId,
        [string]$Description,
        [int]$ExpirationMonths
    )
    
    Write-Info "Création du secret client..."
    
    try {
        $endDate = (Get-Date).AddMonths($ExpirationMonths)
        
        $secretParams = @{
            ApplicationId = $ApplicationId
            PasswordCredential = @{
                DisplayName = $Description
                EndDateTime = $endDate
            }
        }
        
        $secret = Add-MgApplicationPassword @secretParams
        
        Write-Success "Secret client créé avec succès"
        Write-Info "Expiration: $($endDate.ToString('yyyy-MM-dd'))"
        
        return $secret
    }
    catch {
        Write-Error "Erreur lors de la création du secret: $_"
        exit 1
    }
}

function Show-ConfigurationSummary {
    param(
        [object]$Application,
        [object]$Secret,
        [string]$TenantId
    )
    
    Write-Host "`n" -NoNewline
    Write-Host "=" * 80 -ForegroundColor $SuccessColor
    Write-Host "CONFIGURATION DU CONNECTEUR MICROSOFT 365 TERMINÉE" -ForegroundColor $SuccessColor
    Write-Host "=" * 80 -ForegroundColor $SuccessColor
    
    Write-Host "`nInformations de configuration:" -ForegroundColor $InfoColor
    Write-Host "─" * 40 -ForegroundColor $InfoColor
    
    Write-Host "Directory (tenant) ID: " -NoNewline -ForegroundColor White
    Write-Host $TenantId -ForegroundColor Yellow
    
    Write-Host "Application (client) ID: " -NoNewline -ForegroundColor White
    Write-Host $Application.AppId -ForegroundColor Yellow
    
    Write-Host "Client Secret Value: " -NoNewline -ForegroundColor White
    Write-Host $Secret.SecretText -ForegroundColor Red
    
    Write-Host "`nConfiguration Docker Compose:" -ForegroundColor $InfoColor
    Write-Host "─" * 40 -ForegroundColor $InfoColor
    
    $dockerConfig = @"
environment:
  - TENANT_ID=$TenantId
  - CLIENT_ID=$($Application.AppId)
  - CLIENT_SECRET=$($Secret.SecretText)
"@
    
    Write-Host $dockerConfig -ForegroundColor Green
    
    Write-Host "`n" -NoNewline
    Write-Host "⚠️  IMPORTANT: Sauvegardez immédiatement le Client Secret dans un gestionnaire de mots de passe sécurisé!" -ForegroundColor Red
    Write-Host "   Cette valeur ne sera plus jamais affichée." -ForegroundColor Red
    Write-Host "`n" -NoNewline
}

# Script principal
function Main {
    Write-Host "🚀 Configuration automatisée du connecteur Microsoft 365" -ForegroundColor $SuccessColor
    Write-Host "=" * 60 -ForegroundColor $SuccessColor
    
    try {
        # 0. Vérification des prérequis
        if (-not (Test-Prerequisites)) {
            Write-Error "Arrêt du script en raison de prérequis non satisfaits"
            exit 1
        }
        
        Write-Host "`n🔧 Début de la configuration automatisée..." -ForegroundColor $InfoColor
        Write-Host "=" * 50 -ForegroundColor $InfoColor
        
        # 1. Vérification et installation des modules
        Test-RequiredModules
        
        # 2. Connexion à Microsoft Graph
        Connect-ToMicrosoftGraph
        
        # 3. Récupération du Tenant ID
        $context = Get-MgContext
        $tenantId = $context.TenantId
        
        # 4. Création de l'App Registration
        $app = New-AppRegistration -DisplayName $AppName
        
        # 5. Création du Service Principal pour l'application
        Write-Info "Création du Service Principal..."
        $servicePrincipal = New-MgServicePrincipal -AppId $app.AppId
        Write-Success "Service Principal créé"
        
        # 6. Configuration des permissions
        $permissions = @(
            "Mail.Read",
            "Mail.ReadWrite", 
            "Mail.Send",
            "MailboxSettings.ReadWrite",
            "User.Read.All"
        )
        
        $resourceAccess = Set-ApiPermissions -ApplicationId $app.Id -Permissions $permissions
        
        # 7. Attribution du consentement administrateur
        Start-Sleep -Seconds 2  # Attendre que les permissions soient propagées
        Grant-AdminConsent -ApplicationId $app.Id -ServicePrincipalId $servicePrincipal.Id -ResourceAccess $resourceAccess
        
        # 8. Création du secret client
        $secret = New-ClientSecret -ApplicationId $app.Id -Description $SecretDescription -ExpirationMonths $SecretExpirationMonths
        
        # 9. Affichage du résumé
        Show-ConfigurationSummary -Application $app -Secret $secret -TenantId $tenantId
        
        Write-Success "Configuration terminée avec succès! 🎉"
        
    }
    catch {
        Write-Error "Erreur durant la configuration: $_"
        exit 1
    }
    finally {
        # Nettoyage
        Write-Info "Déconnexion de Microsoft Graph..."
        Disconnect-MgGraph -ErrorAction SilentlyContinue
    }
}

# Exécution du script principal
Main
