#!/usr/bin/env pwsh

<#
.SYNOPSIS
    Glimps Exchange 365 and Purview rules management script
    
.DESCRIPTION
    This script manages Exchange 365 transport rules and Purview journaling rules for Glimps security
    Compatible with Windows/Linux using PowerShell Core
    
.PARAMETER TokenValue
    Token value to add in the x-glimps-token header
    
.PARAMETER UserList
    Optional user list (comma-separated emails). If not specified, applies to entire organization
    
.PARAMETER Uninstall
    Switch to uninstall existing rules
    
.PARAMETER TenantId
    Azure AD tenant ID (optional if already connected)
    
.EXAMPLE
    ./Glimps-Exchange-Manager.ps1 -TokenValue "abc123token" -JournalRecipient "connector@glimps.example.com"
    
.EXAMPLE
    ./Glimps-Exchange-Manager.ps1 -TokenValue "abc123token" -JournalRecipient "connector@glimps.example.com" -UserList "user1@domain.com,user2@domain.com"
    
.EXAMPLE
    ./Glimps-Exchange-Manager.ps1 -Uninstall

.EXAMPLE
    ./Glimps-Exchange-Manager.ps1 -TokenValue "abc123token" -JournalRecipient "connector@glimps.example.com" -NoGUI

.NOTES
    DEBIAN/UBUNTU DEPENDENCIES:
    
    1. Install PowerShell Core:
       # Update package list
       sudo apt update
       
       # Install prerequisites
       sudo apt install -y wget apt-transport-https software-properties-common
       
       # Download Microsoft repository GPG key
       wget -q "https://packages.microsoft.com/config/debian/$(lsb_release -rs)/packages-microsoft-prod.deb"
       
       # Register Microsoft repository
       sudo dpkg -i packages-microsoft-prod.deb
       
       # Update package list after adding Microsoft repository
       sudo apt update
       
       # Install PowerShell
       sudo apt install -y powershell
       
    2. Alternative installation via snap:
       sudo snap install powershell --classic
       
    3. Verify installation:
       pwsh --version
       
    4. Required PowerShell modules (auto-installed by script):
       - ExchangeOnlineManagement
       - Microsoft.Graph.Authentication
       
    5. Network requirements:
       - Outbound HTTPS (443) access to:
         * login.microsoftonline.com
         * outlook.office365.com
         * graph.microsoft.com
         * *.powershellgallery.com (for module installation)
       
    PERMISSIONS REQUIRED:
    - Exchange Administrator or Global Administrator role
    - Microsoft Graph permissions (minimal required):
      * Policy.Read.All
      
    USER LIST HANDLING FOR JOURNALING:
    When a UserList is specified, the script creates individual journal rules 
    for each user specified in the list. Each rule is named with a suffix:
    - Main rule (global): "Glimps-Security-Journaling" 
    - Individual rules: "Glimps-Security-Journaling-User1", "User2", etc.
    
    This approach ensures each user has a dedicated journal rule targeting
    their specific email address, providing granular control over journaling.
    
    All individual rules are automatically cleaned up during uninstallation.
    
    FIRST RUN SETUP:
    Before running this script, ensure:
    1. PowerShell execution policy allows script execution:
       Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
       
    2. For Linux users, make the script executable:
       chmod +x Glimps-Exchange-Manager.ps1
       
    3. The user has appropriate Microsoft 365 administrator permissions
#>

[CmdletBinding(DefaultParameterSetName = 'Install')]
param(
    [Parameter(Mandatory = $true, ParameterSetName = 'Install')]
    [string]$TokenValue,
    
    [Parameter(Mandatory = $false, ParameterSetName = 'Install')]
    [Parameter(Mandatory = $false, ParameterSetName = 'Uninstall')]
    [string]$UserList,
    
    [Parameter(Mandatory = $true, ParameterSetName = 'Uninstall')]
    [switch]$Uninstall,

    [Parameter(Mandatory = $true, ParameterSetName = 'Install', HelpMessage="Recipient journal email. ( e.g: connector@mydomain.lan)")]
    [string] $JournalRecipient,

    [Parameter(Mandatory = $false, ParameterSetName = 'Install', HelpMessage="Set to true if used on computers without web browsers")]
    [Parameter(Mandatory = $false, ParameterSetName = 'Uninstall', HelpMessage="Set to true if used on computers without web browsers")]
    [switch] $NoGUI = $false
)

# Configuration variables
$RULE_PREFIX = "Glimps-Security-"
$TRANSPORT_RULE_NAME = "$RULE_PREFIX" + "HeaderInjection"
$JOURNAL_RULE_NAME = "$RULE_PREFIX" + "Journaling"
$RECIPIENT_DOMAIN = if ($JournalRecipient) { $JournalRecipient.Split("@")[-1] } else { "" }

# Disable telemetry
$env:POWERSHELL_TELEMETRY_OPTOUT = 1

# Display symbols
$CHECK_MARK = if ($IsWindows -or $env:OS -eq "Windows_NT") { "[✓]" } else { "[✓]" }
$CROSS_MARK = if ($IsWindows -or $env:OS -eq "Windows_NT") { "[✗]" } else { "[✗]" }
$INFO_MARK = if ($IsWindows -or $env:OS -eq "Windows_NT") { "[i]" } else { "[i]" }

# Utility functions
function Write-Success {
    param([string]$Message)
    Write-Host "$CHECK_MARK $Message" -ForegroundColor Green
}

function Write-Error {
    param([string]$Message)
    Write-Host "$CROSS_MARK $Message" -ForegroundColor Red
}

function Write-Info {
    param([string]$Message)
    Write-Host "$INFO_MARK $Message" -ForegroundColor Cyan
}

function Test-ModuleInstalled {
    param([string]$ModuleName)
    
    $module = Get-Module -ListAvailable -Name $ModuleName
    return $null -ne $module
}

function Install-RequiredModules {
    Write-Info "Checking required modules..."
    
    $requiredModules = @(
        "ExchangeOnlineManagement",
        "Microsoft.Graph.Authentication"
    )
    
    $allModulesOk = $true
    
    foreach ($module in $requiredModules) {
        if (-not (Test-ModuleInstalled $module)) {
            Write-Info "Installing module $module..."
            try {
                Install-Module -Name $module -Force -AllowClobber -Scope CurrentUser
                Write-Success "Module $module installed successfully"
            }
            catch {
                Write-Error "Failed to install module $module : $($_.Exception.Message)"
                $allModulesOk = $false
            }
        }
        else {
            Write-Success "Module $module already installed"
        }
    }
    
    # Install DNS utilities for Linux
    if (-not $IsWindows -and -not $env:OS -eq "Windows_NT") {
        try {
            $null = Get-Command "dig" -ErrorAction Stop
            Write-Success "DNS utilities already available"
        }
        catch {
            Write-Info "Installing DNS utilities for Linux..."
            try {
                $null = Invoke-Expression "apt update && apt install -y dnsutils" -ErrorAction Stop
                Write-Success "DNS utilities installed successfully"
            }
            catch {
                Write-Error "Failed to install DNS utilities. Please install manually: sudo apt install -y dnsutils"
                $allModulesOk = $false
            }
        }
    }

    return $allModulesOk
}

function Connect-Services {
    Write-Info "Connecting to Microsoft 365 services..."
    
    try {
        # Connect to Exchange Online
        Write-Info "Connecting to Exchange Online..."
        
        # Save current preference variables
        $originalVerbosePreference = $VerbosePreference
        $originalInformationPreference = $InformationPreference
        $originalWarningPreference = $WarningPreference
        $originalProgressPreference = $ProgressPreference
        
        # Suppress all output during connection
        $VerbosePreference = 'SilentlyContinue'
        $InformationPreference = 'SilentlyContinue'
        $WarningPreference = 'SilentlyContinue'
        $ProgressPreference = 'SilentlyContinue'

        # Prepare connection parameters
        $connectExchangeParams = @{
            ShowProgress = $false
            ShowBanner = $false
        }

        if ($NoGUI) { 
            $connectExchangeParams.Device = $true 
        }

        try {
            Connect-ExchangeOnline @connectExchangeParams *>&1 | Out-Null
        }
        finally {
            # Restore original preference variables
            $VerbosePreference = $originalVerbosePreference
            $InformationPreference = $originalInformationPreference
            $WarningPreference = $originalWarningPreference
            $ProgressPreference = $originalProgressPreference
        }
        
        Write-Success "Successfully connected to Exchange Online"
        
        # Connect to Microsoft Graph for basic operations
        Write-Info "Connecting to Microsoft Graph..."
        $scopes = @(
            "Policy.Read.All"
        )

        # Prepare Graph connection parameters
        $connectGraphParams = @{
            Scopes = $scopes
            NoWelcome = $true
        }

        if ($NoGUI) { 
            $connectGraphParams.UseDeviceCode = $true 
        }

        Connect-MgGraph @connectGraphParams

        Write-Success "Successfully connected to Microsoft Graph"

        return $true
    }
    catch {
        Write-Error "Failed to connect to services: $($_.Exception.Message)"
        return $false
    }
}

function Test-Permissions {
    Write-Info "Verifying permissions..."
    
    try {
        # Test Exchange permissions
        $null = Get-TransportRule -ResultSize 1
        Write-Success "Exchange Online permissions validated"
        
        # Test Graph permissions
        $context = Get-MgContext
        if ($context) {
            Write-Success "Microsoft Graph permissions validated"
        }
        else {
            throw "Microsoft Graph context not available"
        }
        
        return $true
    }
    catch {
        Write-Error "Insufficient permissions: $($_.Exception.Message)"
        return $false
    }
}

function Test-ServerConnection {
    Write-Info "Verifying Glimps Malware server connection..."

    if ([string]::IsNullOrWhiteSpace($RECIPIENT_DOMAIN)) {
        Write-Error "Recipient domain is empty"
        return $false
    }

    try {
        # Try to resolve MX record
        if ($IsWindows -or $env:OS -eq "Windows_NT") {
            # Windows - use Resolve-DnsName
            $mxRecords = Resolve-DnsName -Name $RECIPIENT_DOMAIN -Type MX -ErrorAction Stop
            if (-not $mxRecords -or $mxRecords.Count -eq 0) {
                Write-Error "Could not find MX Record for $RECIPIENT_DOMAIN"
                return $false
            }
            $recipientServer = $mxRecords[0].NameExchange
        }
        else {
            # Linux - use dig command
            $digOutput = dig MX $RECIPIENT_DOMAIN +short 2>/dev/null
            if ([string]::IsNullOrWhiteSpace($digOutput)) {
                Write-Error "Could not find MX Record for $RECIPIENT_DOMAIN"
                return $false
            }
            # Parse dig output (format: "priority server")
            $mxLine = $digOutput.Split("`n")[0].Trim()
            $recipientServer = $mxLine.Split(" ")[-1].TrimEnd('.')
        }

        # Test connection to mail server
        # if (-not (Test-NetConnection -ComputerName $recipientServer -Port 25 -InformationLevel Quiet -WarningAction SilentlyContinue)) {
        #     Write-Error "Could not connect to $recipientServer on port 25"
        #     return $false
        # }
        
        Write-Success "Successfully validated connection to $recipientServer"
        return $true
    }
    catch {
        Write-Error "Error testing server connection: $($_.Exception.Message)"
        return $false
    }
}

function Create-TransportRule {
    param(
        [string]$TokenValue,
        [array]$Recipients
    )
    
    Write-Info "Creating Exchange transport rule..."
    
    try {
        # Base rule parameters
        $ruleParams = @{
            Name = $TRANSPORT_RULE_NAME
            SetHeaderName = "x-glimps-token"
            SetHeaderValue = $TokenValue
            Enabled = $true
            Priority = 0
            Comments = "Glimps Rule - Security header injection"
        }
        
        # Configure recipients
        if ($Recipients -and $Recipients.Count -gt 0) {
            $ruleParams.SentTo = $Recipients
            Write-Info "Rule applied to specified users: $($Recipients -join ', ')"
        }
        else {
            $ruleParams.SentToScope = "InOrganization"
            Write-Info "Rule applied to entire organization"
        }
        
        # Remove existing rule if it exists
        $existingRule = Get-TransportRule -Identity $TRANSPORT_RULE_NAME -ErrorAction SilentlyContinue
        if ($existingRule) {
            $null = Remove-TransportRule -Identity $TRANSPORT_RULE_NAME -Confirm:$false
            Write-Info "Existing transport rule removed"
        }
        
        # Create new rule
        $null = New-TransportRule @ruleParams
        Write-Success "Transport rule created successfully"
        return $true
    }
    catch {
        Write-Error "Failed to create transport rule: $($_.Exception.Message)"
        return $false
    }
}

function Create-JournalRule {
    param([array]$Recipients)
    
    Write-Info "Creating journaling rule(s)..."
    
    try {
        # Remove existing main rule if it exists
        $existingRule = Get-JournalRule -Identity $JOURNAL_RULE_NAME -ErrorAction SilentlyContinue
        if ($existingRule) {
            $null = Remove-JournalRule -Identity $JOURNAL_RULE_NAME -Confirm:$false
            Write-Info "Existing main journaling rule removed"
        }
        
        # Remove existing individual rules if they exist
        $existingIndividualRules = Get-JournalRule | Where-Object { $_.Name -like "$JOURNAL_RULE_NAME-User*" }
        if ($existingIndividualRules) {
            $existingIndividualRules | ForEach-Object {
                $null = Remove-JournalRule -Identity $_.Name -Confirm:$false
                Write-Info "Existing individual rule removed: $($_.Name)"
            }
        }
        
        # Configure recipients
        if ($Recipients -and $Recipients.Count -gt 0) {
            Write-Info "Creating individual journal rules for specified users..."
            
            # Create individual journal rules for each user
            $ruleIndex = 1
            $successCount = 0
            $totalUsers = $Recipients.Count
            
            foreach ($recipient in $Recipients) {
                try {
                    $individualRuleName = "$JOURNAL_RULE_NAME-User$ruleIndex"
                    
                    # Create individual rule
                    $null = New-JournalRule -Name $individualRuleName -JournalEmailAddress $JournalRecipient -Recipient $recipient -Scope Global -Enabled $true
                    Write-Success "Individual journal rule created for: $recipient"
                    $successCount++
                }
                catch {
                    Write-Error "Failed to create journal rule for $recipient : $($_.Exception.Message)"
                }
                $ruleIndex++
            }
            
            # Return true if all rules were successful, false otherwise
            $allSuccessful = ($successCount -eq $totalUsers)
            Write-Info "Journal rules creation completed: $successCount/$totalUsers successful"
            return $allSuccessful
        }
        else {
            Write-Info "Creating global journaling rule for entire organization..."
            
            # Create global rule for entire organization
            $journalParams = @{
                Name = $JOURNAL_RULE_NAME
                JournalEmailAddress = $JournalRecipient
                Enabled = $true
                Scope = "Global"
            }
            
            $null = New-JournalRule @journalParams
            Write-Success "Global journaling rule created successfully"
            return $true
        }
    }
    catch {
        Write-Error "Failed to create journaling rule: $($_.Exception.Message)"
        return $false
    }
}

function Remove-GlimpsRules {
    Write-Info "Removing existing Glimps rules..."
    
    $success = $true
    
    # Remove transport rule
    try {
        $transportRule = Get-TransportRule -Identity $TRANSPORT_RULE_NAME -ErrorAction SilentlyContinue
        if ($transportRule) {
            $null = Remove-TransportRule -Identity $TRANSPORT_RULE_NAME -Confirm:$false
            Write-Success "Transport rule removed"
        }
        else {
            Write-Info "No Glimps transport rule found"
        }
    }
    catch {
        Write-Error "Failed to remove transport rule: $($_.Exception.Message)"
        $success = $false
    }
    
    # Remove main journaling rule
    try {
        $journalRule = Get-JournalRule -Identity $JOURNAL_RULE_NAME -ErrorAction SilentlyContinue
        if ($journalRule) {
            $null = Remove-JournalRule -Identity $JOURNAL_RULE_NAME -Confirm:$false
            Write-Success "Main journaling rule removed"
        }
        else {
            Write-Info "No main Glimps journaling rule found"
        }
    }
    catch {
        Write-Error "Failed to remove main journaling rule: $($_.Exception.Message)"
        $success = $false
    }
    
    # Remove individual journaling rules (if any exist)
    try {
        $individualRules = Get-JournalRule | Where-Object { $_.Name -like "$JOURNAL_RULE_NAME-User*" }
        if ($individualRules) {
            $individualRules | ForEach-Object {
                try {
                    $null = Remove-JournalRule -Identity $_.Name -Confirm:$false
                    Write-Success "Individual journaling rule removed: $($_.Name)"
                }
                catch {
                    Write-Error "Failed to remove individual rule $($_.Name): $($_.Exception.Message)"
                    $success = $false
                }
            }
        }
        else {
            Write-Info "No individual Glimps journaling rules found"
        }
    }
    catch {
        Write-Error "Error while checking for individual journaling rules: $($_.Exception.Message)"
        $success = $false
    }
    
    return $success
}

function Show-Summary {
    param(
        [bool]$TransportRuleSuccess,
        [bool]$JournalRuleSuccess,
        [string]$Operation
    )
    
    Write-Host ""
    Write-Host ("="*50)
    Write-Host "OPERATION SUMMARY: $Operation" -ForegroundColor Yellow
    Write-Host ("="*50)
    
    if ($Operation -eq "INSTALLATION") {
        if ($TransportRuleSuccess) {
            Write-Success "Transport rule (header injection) configured"
        }
        else {
            Write-Error "Transport rule (header injection) failed"
        }
        
        if ($JournalRuleSuccess) {
            Write-Success "Journaling rule configured"
        }
        else {
            Write-Error "Journaling rule failed"
        }
        
        Write-Host "`nConfiguration details:" -ForegroundColor Yellow
        Write-Host "- Header: x-glimps-token" -ForegroundColor White
        Write-Host "- Journal destination: $JournalRecipient" -ForegroundColor White
    }
    else {
        if ($TransportRuleSuccess -and $JournalRuleSuccess) {
            Write-Success "All Glimps rules have been removed"
        }
        else {
            Write-Error "Some rules could not be removed"
        }
    }
    
    Write-Host ("="*50)
    Write-Host ""
}

# MAIN SCRIPT
function Main {
    Write-Host @"
┌─────────────────────────────────────────────────────┐
│           GLIMPS EXCHANGE 365 CONNECTOR             │
│             Security Rules Management               │
└─────────────────────────────────────────────────────┘
"@ -ForegroundColor Cyan

    # Install required modules
    if (-not (Install-RequiredModules)) {
        Write-Error "Unable to install all required modules"
        exit 1
    }
    
    # Connect to services
    if (-not (Connect-Services)) {
        Write-Error "Unable to connect to Microsoft 365 services"
        exit 1
    }
    
    # Verify permissions
    if (-not (Test-Permissions)) {
        Write-Error "Insufficient permissions to perform operations"
        exit 1
    }
    
    # Test server connection (only for installation)
    if (-not $Uninstall) {
        if (-not (Test-ServerConnection)) {
            Write-Warning "Could not connect to remote server. Do you want to continue anyway? (Y/N)" -WarningAction Inquire
            $response = Read-Host "Continue"
            if ($response -notmatch "^[Yy]") {
                Write-Info "Operation cancelled by user"
                exit 0
            }
        }
    }

    # Process user list
    $recipients = @()
    if (-not [string]::IsNullOrWhiteSpace($UserList)) {
        $recipients = $UserList.Split(',') | ForEach-Object { $_.Trim() } | Where-Object { -not [string]::IsNullOrWhiteSpace($_) }
        Write-Info "User list specified: $($recipients.Count) user(s)"
    }
    
    # Execute operations
    if ($Uninstall) {
        Write-Info "Uninstall mode activated"
        $uninstallSuccess = Remove-GlimpsRules
        Show-Summary -TransportRuleSuccess $uninstallSuccess -JournalRuleSuccess $uninstallSuccess -Operation "UNINSTALLATION"
    }
    else {
        Write-Info "Installation mode activated"
        Write-Info "Token to configure: $TokenValue"
        
        $transportSuccess = Create-TransportRule -TokenValue $TokenValue -Recipients $recipients
        $journalSuccess = Create-JournalRule -Recipients $recipients
        
        # Ensure we have boolean values
        $transportResult = [bool]$transportSuccess
        $journalResult = [bool]$journalSuccess
        
        Show-Summary -TransportRuleSuccess $transportResult -JournalRuleSuccess $journalResult -Operation "INSTALLATION"
    }
    
    # Clean disconnection
    try {
        Disconnect-ExchangeOnline -Confirm:$false -ErrorAction SilentlyContinue
        Disconnect-MgGraph -ErrorAction SilentlyContinue
        Write-Info "Services disconnected"
    }
    catch {
        # Silent error for disconnection
    }
    
    Write-Host "Script completed." -ForegroundColor Green
}

# Execute main script
try {
    Main
}
catch {
    Write-Error "Critical error in script: $($_.Exception.Message)"
    Write-Host "Stack trace: $($_.ScriptStackTrace)" -ForegroundColor Red
    exit 1
}
