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

	MaxUploadSize string `json:"max_upload_size" desc:"Maximum file size to send to Detect (format as: 100MB). Files above that limit will not be analyzed and an alert will be raised."`

	RetryFrequency Duration `json:"retry_frequency" desc:"Frequency to try/retry submitting files that were rejected due to reached quotas."`

	MonitoringFrequency Duration `json:"monitoring_frequency" desc:"Frequency to analyze changes on monitored drives (independently of webhook notifications) (format: 10m)."`

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

	ExclusionRules []SPExclusionRule `json:"exclusion_rules" mapstructure:"exclusion-rules" desc:"Exclusion rules allow to exclude certain files or folders from analysis. It is particularly useful for files that are modified regularly, for which whitelisting by hash is not sufficient. Each rule is associated to a single drive (= a library in a site)"`

	TimeoutFactor int `json:"timeout_factor" mapstructure:"timeout-factor" validate:"min=1" desc:"Optional factor to increase timeouts (if set, must be an integer >= 1). 1 to use default timeouts values"`
}

type SPMitigationActions struct {
	Quarantine bool `json:"quarantine" mapstructure:"quarantine" desc:"Move malware files to quarantine site (requires quarantine configuration)"`
	Delete     bool `json:"delete" mapstructure:"delete" desc:"Permanently deletes malware files"`
	Log        bool `json:"log" mapstructure:"log" desc:"Log malware detections (in connector manager's logs). Files detected as malware are left untouched"`
}

type SPExclusionRule struct {
	SiteURL              string               `json:"site_url" mapstructure:"site-url" validate:"required,url" desc:"e.g. https://myTenant.sharepoint.com/sites/mySite"`
	LibName              string               `json:"lib_name" mapstructure:"lib-name" validate:"required" desc:"Library name in site (e.g. Documents). In a SharePoint site, document libraries can be accessed from the site menu. Once in a library, its name is displayed at the top of the page, above files"`
	FilesToExclude       []string             `json:"files_to_exclude" mapstructure:"files-to-exclude" desc:"List of file paths to exclude from analysis. Must be absolute paths (e.g. /folder1/folder2/myFile.txt)"`
	DirectoriesToExclude []DirectoryToExclude `json:"directories_to_exclude" mapstructure:"directories-to-exclude" desc:"List of directory paths to exclude from analysis. Note: exclusion only applies to files directly in specified folder. (so if '/folder1' is excluded, then '/folder1/folder2' isn't"`
}

type DirectoryToExclude struct {
	Path       string   `json:"path" mapstructure:"path" validate:"required" desc:"Directory path to exclude (must be absolute, e.g. /folder1/folder2)."`
	Extensions []string `json:"extensions" mapstructure:"extensions" desc:"List of extensions (e.g. .exe) to exclude in that directory. Leave empty to exclude all files, no matter the extension"`
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
