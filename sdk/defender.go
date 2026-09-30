package sdk

type DefenderConfig struct {
	CommonConnectorConfig
	TenantID                string   `json:"tenant_id" validate:"required" desc:"Microsoft Entra tenant ID hosting the App Registration"`
	ClientID                string   `json:"client_id" validate:"required" desc:"Application (client) ID of the Defender App Registration"`
	ClientSecret            string   `json:"client_secret,omitempty" password:"true" validate:"required" desc:"Client secret generated for the Defender App Registration"`
	Endpoint                string   `json:"endpoint" validate:"omitempty,url" desc:"Microsoft Defender for Endpoint API URL (default: https://api.securitycenter.microsoft.com)"`
	HTTPTimeout             Duration `json:"http_timeout" validate:"omitempty,gt=0" desc:"Timeout of a Defender control-plane call, file downloads excluded (e.g. '30s')"`
	PollInterval            Duration `json:"poll_interval" validate:"omitempty,gt=0" desc:"Interval between two Defender alert polls (e.g. '1m')"`
	WorkerNb                int      `json:"worker_nb" validate:"omitempty,gt=0" desc:"Number of alerts processed concurrently (default: 20)"`
	AlertLookback           Duration `json:"alert_lookback" validate:"omitempty,gt=0" desc:"How far back the first poll reaches (e.g. '24h'). Unset starts from now"`
	DownloadTimeout         Duration `json:"download_timeout" validate:"omitempty,gt=0" desc:"Budget for collecting one file over Live Response, the wait for an offline device included (default: 2h30m, must stay under 3h30m)"`
	ProcessResolvedAlerts   bool     `json:"process_resolved_alerts" desc:"Whether alerts already resolved in Defender are analyzed too"`
	CommentOnly             bool     `json:"comment_only" desc:"Only write the GLIMPS verdict as an alert comment, leaving the alert classification and determination untouched"`
	QuarantineFile          bool     `json:"quarantine_file" desc:"Whether a file identified as malware is stopped and quarantined on the endpoint"`
	PostIndicator           bool     `json:"post_indicator" desc:"Whether the hash of a file identified as malware is posted as a Defender threat indicator"`
	IndicatorResponseAction string   `json:"indicator_response_action" validate:"omitempty,oneof=Alert Warn Block Audit BlockAndRemediate AlertAndBlock" desc:"Action Defender applies to the posted indicator (default: Alert)"`
}

func (c *DefenderConfig) Strip() any {
	cc := *c
	cc.ClientSecret = ""
	return cc
}
