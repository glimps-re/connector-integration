package sdk

type M365Config struct {
	CommonConnectorConfig
	JournalRecipient             string `json:"journal_recipient" validate:"required" desc:"recipient of the journaling mail"`
	DeleteMalwareMail            bool   `json:"delete_malware_mail" desc:"Whether mail identified as malware should be deleted from the inbox"`
	QuarantineMailbox            string `json:"quarantine_mailbox" desc:"Quarantine mailbox, must be part of the domain, mail identified as malware are imported in it"`
	IncludeMailContentQuarantine bool   `json:"include_mail_content_quarantine" desc:"Whether quarantine mail should contains original mail content in their body"`
	AddNotAnalyzedRule           bool   `json:"add_not_analyzed_rule" desc:"Whether not analyzed label should be added automatically to new incoming mail"`
	SetLegitCategory             bool   `json:"set_legit_category" desc:"Whether to add a legit label to mail identified as safe"`
	M365ClientID                 string `json:"m365_client_id" validate:"required" desc:"M365 App Registration Tenant Client ID"`
	M365ClientTenant             string `json:"m365_client_tenant" validate:"required" desc:"M365 App Registration Tenant Name"`
	M365ClientSecret             string `json:"m365_client_secret,omitempty" password:"true" validate:"required" desc:"M365 App Registration generated secret key"`
	HeaderTokenValue             string `json:"header_token_value" validate:"required" desc:"Mail header value set by an exchange rule for mail sent to journaling rule address, to authenticate"`
}

func (c *M365Config) Strip() any {
	cc := *c
	cc.M365ClientSecret = ""
	return cc
}
