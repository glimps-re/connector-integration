package sdk

import (
	"fmt"
	"net/url"
)

type SharepointConfig struct {
	ReconfigurableSharepointConfig

	Database string `json:"database" validate:"required" reconfigurable:"false" desc:"Database to use. e.g. 'example1' means connector will use database located at /etc/glimps_connector/bdd/example1.db (file will be created if it does not exist)."`

	// Not necessary fields :
	// RestoreToken (only for standalone, not needed for console)"`
}

type ReconfigurableSharepointConfig struct {
	CommonConnectorConfig

	ClientName       string `json:"client_name" validate:"required"`
	M365TenantID     string `json:"m365_tenant_id" validate:"required" desc:"Tenant ID"`
	M365ClientID     string `json:"m365_client_id" validate:"required" desc:"M365 app registration client ID"`
	M365ClientSecret string `json:"m365_client_secret,omitempty" password:"true" validate:"required" desc:"M365 app registration client secret"`

	RealTimeMonitoring bool   `json:"real_time_monitoring" mapstructure:"real-time-monitoring" desc:"Use real-time monitoring. Requires 'WebhookURL' to be provided"`
	WebhookURL         string `json:"webhook_url" validate:"required_if=RealTimeMonitoring true,omitempty,url,startswith=https" desc:"URL where microsoft will send webhook notifications (technically to webhook-url/api/v1). Must starts by https"`

	MitigationAction SPMitigationActions `json:"mitigation_action" mapstructure:"mitigation_action" validate:"required" desc:"Action to perform when a file is detected as malware (only one can be selected)"`

	MaxUploadSize string `json:"max_upload_size" desc:"Maximum file size to send to Detect (format as: 100MB). Files above that limit will not be analyzed and an alert will be raised. Default value is 100MB"`

	RetryFrequency string `json:"retry_frequency" desc:"Frequency to try/retry submitting files that were rejected due to reached quotas. Has a default value if left empty"`

	MonitoringFrequency string `json:"monitoring_frequency" desc:"Frequency to analyze changes on monitored drives (independently of webhook notifications) (format: 10m). Default value is 10m."`

	QuarantineURL                        string `json:"quarantine_url" validate:"omitempty,url" desc:"URL of the sharepoint site that will be used as quarantine"`
	QuarantineLibName                    string `json:"quarantine_lib_name" desc:"Library name to use inside the quarantine site. If not provided, default library is used (usually named 'Documents')"`
	QuarantineDeleteMalwareOnMSDetection bool   `json:"quarantine_delete_malware_on_microsoft_detection" desc:"Delete files categorized as malware by Microsoft. If not selected, no remediation will be made on those files. This option only applies when mitigation action is quarantine"`

	// Monitored items
	MonitorAllWithoutInitialScan bool `json:"monitor_all_without_initial_scan" desc:"Monitor all drives, with no initial scan made by default"`
	MonitorAllWithInitialScan    bool `json:"monitor_all_with_initial_scan" desc:"Monitor all drives, with initial scan made by default"`

	SitesToMonitorWithoutInitialScan []string `json:"sites_to_monitor_without_initial_scan" validate:"dive,url" desc:"Sites to monitor without initial scan (format: 'https://mySharepoint.com/sites/mySite')"`
	SitesToMonitorWithInitialScan    []string `json:"sites_to_monitor_with_initial_scan" validate:"dive,url" desc:"Same as SitesToMonitor, but with initial scan"`

	GroupsToMonitorWithoutInitialScan []string `json:"groups_to_monitor_without_initial_scan" validate:"dive" desc:"Groups to monitor without initial scan (group names, e.g. 'myGroup')"`
	GroupsToMonitorWithInitialScan    []string `json:"groups_to_monitor_with_initial_scan" validate:"dive" desc:"Same as GroupsToMonitor, but with initial scan"`

	// MonitorAll mode only
	SitesToIgnore  []string `json:"sites_to_ignore" validate:"dive,url" desc:"Sites to not monitor when scope is MonitorAll (format: 'https://mySharepoint.com/sites/mySite')"`
	GroupsToIgnore []string `json:"groups_to_ignore" validate:"dive" desc:"Groups to not monitor when scope is MonitorAll (group names, e.g. 'myGroup')"`

	ExcludeDirs  []string `json:"exclude_dirs" desc:"Directories to exclude from analysis. MUST respect following format (complicated, will change in future version): siteName;;;libName;;;/full/path/to/exclude;;;[listOfExtensions] Example: https://myTenant.sharepoint.com/sites/mySite;;;Documents;;;/folder1/folder2;;;[.exe,.txt] [list-of-extensions] is optional, put [] if you want all files to be skipped in that dir, no matter the extension"`
	ExcludeFiles []string `json:"exclude_files" desc:"Files to exclude from analysis. MUST respect following format: siteName;;;libName;;;/full/path/to/file/myFile.txt Example: https://myTenant.sharepoint.com/sites/mySite;;;Documents;;;/folder1/folder2/myFile.txt"`
}

type SPMitigationActions struct {
	Quarantine bool `json:"quarantine" mapstructure:"quarantine" desc:"Move malware files to quarantine site (requires quarantine configuration)"`
	Delete     bool `json:"delete" mapstructure:"delete" desc:"Permanently deletes malware files"`
	Log        bool `json:"log" mapstructure:"log" desc:"Log malware detections (in connector manager's logs). Files detected as malware are left untouched"`
}

type SharepointHelmConf struct {
	ConsoleConfig
	SharepointWebhookHost string `desc:"domain name where microsoft will send webhook notifications (e.g. client1.sharepoint.myserver.glimps.lan)"`
}

func (c *SharepointConfig) Strip() any {
	cc := *c
	cc.M365ClientSecret = ""
	return cc
}

func (c *SharepointConfig) GetHelmConfig(consoleConfig ConsoleConfig) (helmConfig any, err error) {
	var spWebhookHost string
	if c.WebhookURL != "" {
		var webhookURL *url.URL
		webhookURL, err = url.Parse(c.WebhookURL)
		if err != nil {
			err = fmt.Errorf("error parsing current conf webhookURL, %w", err)
			return
		}
		spWebhookHost = webhookURL.Hostname()
	}

	helmConfig = SharepointHelmConf{
		ConsoleConfig:         consoleConfig,
		SharepointWebhookHost: spWebhookHost,
	}
	return
}
