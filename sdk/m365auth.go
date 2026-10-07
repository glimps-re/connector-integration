package sdk

// M365AuthMode selects how a connector authenticates against the client's Microsoft Entra application.
type M365AuthMode string

const (
	// M365AuthSecret uses a client secret created on the client's app registration. Default for
	// configurations created before the mode existed.
	M365AuthSecret M365AuthMode = "secret"
	// M365AuthCertificate uses a certificate whose private key never leaves the cluster running the connector.
	M365AuthCertificate M365AuthMode = "certificate"
	// M365AuthFederated uses a Kubernetes service account token exchanged through a federated identity credential.
	M365AuthFederated M365AuthMode = "federated"
)

// M365Auth holds the Microsoft Entra authentication settings shared by Microsoft 365 connectors.
//
// JSON keys are those of the historical SharepointConfig fields, so records stored before the
// authentication mode existed still decode: an empty mode means secret (see EffectiveMode).
// The private key of the certificate mode and the federated token are never part of the
// configuration: they are mounted into the connector by its deployment.
type M365Auth struct {
	M365AuthMode M365AuthMode `json:"m365_auth_mode,omitempty" mapstructure:"m365_auth_mode" validate:"omitempty,oneof=secret certificate federated" desc:"How the connector authenticates against the app registration: 'secret' (client secret, default), 'certificate' (certificate held by the connector deployment) or 'federated' (workload identity)"`
	M365TenantID string       `json:"m365_tenant_id" mapstructure:"m365_tenant_id" validate:"required" desc:"Tenant ID"`
	// The client ID is not required at creation: the console operator creates the instance before the
	// client's app registration exists, and the enrolment assistant fills it in. The connector waits
	// for it before starting (see Enrolled).
	M365ClientID string `json:"m365_client_id" mapstructure:"m365_client_id" validate:"omitempty" desc:"M365 app registration client ID (filled by the Microsoft 365 enrolment; the connector waits for it)"`

	// Mode secret
	M365ClientSecret string `json:"m365_client_secret,omitempty" mapstructure:"m365_client_secret" password:"true" validate:"required_if=M365AuthMode secret,required_without=M365AuthMode" desc:"M365 app registration client secret (required in mode secret)"`

	// Mode certificate
	M365CertThumbprint string `json:"m365_cert_thumbprint,omitempty" mapstructure:"m365_cert_thumbprint" validate:"required_if=M365AuthMode certificate,omitempty,hexadecimal,len=40" desc:"SHA-1 thumbprint (40 hexadecimal characters) of the certificate registered on the app registration (required in mode certificate)"`
	M365AppObjectID    string `json:"m365_app_object_id,omitempty" mapstructure:"m365_app_object_id" validate:"omitempty,uuid" desc:"Object ID of the app registration (not the client ID). Optional, enables automatic certificate rotation in mode certificate"`
	M365CertKeyID      string `json:"m365_cert_key_id,omitempty" mapstructure:"m365_cert_key_id" validate:"omitempty,uuid" desc:"keyId of the certificate on the app registration (keyCredentials). Optional, written by the enrolment and by the rotation so the previous certificate can be removed automatically"`
}

// Enrolled reports whether the Microsoft 365 application is known, i.e. whether the connector can
// authenticate. Before enrolment the connector registers to the manager and waits.
func (a M365Auth) Enrolled() bool {
	return a.M365ClientID != ""
}

// EffectiveMode returns the authentication mode to use, defaulting to secret for configurations
// predating the mode field.
func (a M365Auth) EffectiveMode() M365AuthMode {
	if a.M365AuthMode == "" {
		return M365AuthSecret
	}
	return a.M365AuthMode
}
