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

	ClientName       string `json:"client_name" desc:"client name, used to restore quarantine"`
	M365TenantID     string `json:"m365_tenant_id" validate:"required" desc:"Tenant ID"`
	M365ClientID     string `json:"m365_client_id" validate:"required" desc:"M365 app registration client ID"`
	M365ClientSecret string `json:"m365_client_secret,omitempty" validate:"required" desc:"M365 app registration client secret"`

	WebhookURL string `json:"webhook_url" validate:"omitempty,url,startswith=https" desc:"URL where microsoft will send webhook notifications (technically to webhook-url/api/v1). it must be publicly available and serve on https. Leave empty to not use real-time monitoring."`

	GMalwareAnalysisTimeout string `json:"gmalware_analysis_timeout" desc:"Timeout for Detect analysis. if empty, uses a default value (~ 3min)"`
	MaxUploadSize           string `json:"max_upload_size" desc:"Max size for monitored files to upload to Detect (format as 100MB). Files above that limit will not be analyzed and an alert will be raised. Default value is 100MB."`

	RetryFrequency string `json:"retry_frequency" desc:"Frequency to try/retry submitting files that were rejected due to reached quotas. Has a default value if left empty."`

	MonitoringFrequency string `json:"monitoring_frequency" desc:"Frequency to analyze changes on monitored drives (independently of webhook notifications). Default value is 10m (10 minutes)."`
	DeleteMalware       bool   `json:"delete_malware" desc:"Delete files categorized as malware by Detect."`

	QuarantineURL                        string `json:"quarantine_url" validate:"omitempty,url" desc:"URL of the sharepoint site that will be used as quarantine. Leave empty to not use quarantine."`
	QuarantineLibName                    string `json:"quarantine_lib_name" desc:"Name of the library inside the quarantine site, to use as the quarantine root folder. If empty, default lib is used."`
	QuarantineDeleteMalwareOnMSDetection bool   `json:"quarantine_delete_malware_on_microsoft_detection" desc:"Delete files categorized as malware by Microsoft. If not selected, no remediation will be made on those files. This option only applies when quarantine is configured."`

	// Monitored items
	SitesToMonitorWithoutInitialScan []string `json:"sites_to_monitor_without_initial_scan" validate:"dive,url" desc:"Sites to monitor (only new or modified content) (must be in the format: 'https://mySharepoint.com/sites/mySite')."`
	SitesToMonitorWithInitialScan    []string `json:"sites_to_monitor_with_initial_scan" validate:"dive,url" desc:"Same as SitesToMonitor, but initial content is analyzed at connector start."`

	GroupsToMonitorWithoutInitialScan []string `json:"groups_to_monitor_without_initial_scan" validate:"dive" desc:"Groups to monitor (only new or modified content) (group names, e.g. 'myGroup')."`
	GroupsToMonitorWithInitialScan    []string `json:"groups_to_monitor_with_initial_scan" validate:"dive" desc:"Same as GroupsToMonitor, but initial content is analyzed at connector start."`

	// Only use when monitor all
	SitesToIgnore  []string `json:"sites_to_ignore" validate:"dive,url" desc:"Sites to not monitor when scope is in MonitorAll (must be in the format: 'https://mySharepoint.com/sites/mySite')."`
	GroupsToIgnore []string `json:"groups_to_ignore" validate:"dive" desc:"Groups to not monitor when scope is in MonitorAll (group names, e.g. 'myGroup')."`

	// Alerting
	AlertingSMTPHost          string `json:"alerting_smtp_host" validate:"required" desc:"Format: smtp.example.com"`
	AlertingSMTPPort          int    `json:"alerting_smtp_port" validate:"required" desc:"e.g. 587"`
	AlertingSMTPLogin         string `json:"alerting_smtp_login" validate:"required" desc:"Mail login account (format: email@mail.com)"`
	AlertingSMTPPassword      string `json:"alerting_smtp_password" validate:"required"  password:"true" desc:"Mail password account"`
	AlertingSMTPSkipCertCheck bool   `json:"alerting_smtp_skip_cert_check" desc:"Skip certificates check"`

	AlertingTestMailSending string   `json:"alerting_test_mail_sending" validate:"omitempty,email" desc:"Email address where an email will be sent at startup, to check mail sending works (optional). If empty, only a dial and authenticate check will be made."`
	AlertingMailFrom        string   `json:"alerting_mail_from" validate:"required,email" desc:"Alert sender's email address."`
	AlertingToAdmins        []string `json:"alerting_to_admins" validate:"dive,email" desc:"List of admin email addresses to send alerts to."`
	AlertingToClients       []string `json:"alerting_to_clients" validate:"required,min=1,dive,email" desc:"List of client email addresses to send alerts to."`

	AlertingAdminMailSubject string `json:"alerting_admin_mail_subject" desc:"Admin mail's subject. Leave empty to use default."`
	AlertingAdminMailBody    string `json:"alerting_admin_mail_body" desc:"Admin mail's body. Leave empty to use default."`

	AlertingClientMailSubject string `json:"alerting_client_mail_subject" desc:"Client mail's subject. Leave empty to use default."`
	AlertingClientMailBody    string `json:"alerting_client_mail_body" desc:"Client mail's body. Leave empty to use default."`

	SendWarningAlertsToAdmins bool `json:"send_warning_alerts_to_admins" desc:"Send warning alerts to admins (in addition to clients)."`
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
