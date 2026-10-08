package sdk

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

const (
	testThumbprint = "795E223B443E4ED8B34DF050CC6B6D84C986971F"
	spBase         = `"client_name":"acme","database":"db1","gmalware_api_url":"https://detect.example","gmalware_api_token":"tok","mitigation_action":{"log":true}`
)

func spJSON(extra string) json.RawMessage {
	return json.RawMessage("{" + spBase + "," + extra + "}")
}

func TestM365Auth_EffectiveMode(t *testing.T) {
	if got := (M365Auth{}).EffectiveMode(); got != M365AuthSecret {
		t.Fatalf("empty mode must default to secret, got %q", got)
	}
	if got := (M365Auth{M365AuthMode: M365AuthFederated}).EffectiveMode(); got != M365AuthFederated {
		t.Fatalf("explicit mode must be kept, got %q", got)
	}
}

// Records stored before the authentication mode existed: same JSON keys, no mode.
func TestSharepointConfig_LegacyRecordDecodes(t *testing.T) {
	raw := spJSON(`"m365_tenant_id":"tenant","m365_client_id":"client","m365_client_secret":"s3cret"`)
	config, err := BindAndValidateConfig(SharepointKey, raw)
	if err != nil {
		t.Fatalf("legacy record must bind and validate: %v", err)
	}
	sp := config.(*SharepointConfig)
	if sp.M365AuthMode != "" || sp.EffectiveMode() != M365AuthSecret {
		t.Fatalf("legacy record: mode=%q effective=%q", sp.M365AuthMode, sp.EffectiveMode())
	}
	if sp.M365TenantID != "tenant" || sp.M365ClientID != "client" || sp.M365ClientSecret != "s3cret" {
		t.Fatalf("promoted fields not decoded: %+v", sp.M365Auth)
	}

	// re-encoding keeps the historical keys and omits the empty mode
	out, err := json.Marshal(sp)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"m365_tenant_id":"tenant"`, `"m365_client_id":"client"`, `"m365_client_secret":"s3cret"`} {
		if !strings.Contains(string(out), want) {
			t.Errorf("encoded config misses %s: %s", want, out)
		}
	}
	for _, unwanted := range []string{`"m365_auth_mode"`, `"m365_cert_thumbprint"`, `"m365_app_object_id"`} {
		if strings.Contains(string(out), unwanted) {
			t.Errorf("encoded legacy config must not contain %s: %s", unwanted, out)
		}
	}
}

func TestSharepointConfig_AuthValidation(t *testing.T) {
	cases := []struct {
		name    string
		extra   string
		wantErr string // substring of the validation error, empty for success
	}{
		{"legacy: secret present", `"m365_tenant_id":"t","m365_client_id":"c","m365_client_secret":"s"`, ""},
		{"legacy: secret missing", `"m365_tenant_id":"t","m365_client_id":"c"`, "m365_client_secret"},
		{"secret: explicit, present", `"m365_auth_mode":"secret","m365_tenant_id":"t","m365_client_id":"c","m365_client_secret":"s"`, ""},
		{"secret: explicit, missing", `"m365_auth_mode":"secret","m365_tenant_id":"t","m365_client_id":"c"`, "m365_client_secret"},
		{"certificate: thumbprint, no secret", `"m365_auth_mode":"certificate","m365_tenant_id":"t","m365_client_id":"c","m365_cert_thumbprint":"` + testThumbprint + `"`, ""},
		{"certificate: lower-case thumbprint", `"m365_auth_mode":"certificate","m365_tenant_id":"t","m365_client_id":"c","m365_cert_thumbprint":"` + strings.ToLower(testThumbprint) + `"`, ""},
		{"certificate: with object id", `"m365_auth_mode":"certificate","m365_tenant_id":"t","m365_client_id":"c","m365_cert_thumbprint":"` + testThumbprint + `","m365_app_object_id":"0b2b3b4b-1111-2222-3333-444455556666"`, ""},
		{"certificate: thumbprint missing", `"m365_auth_mode":"certificate","m365_tenant_id":"t","m365_client_id":"c"`, "m365_cert_thumbprint"},
		{"certificate: thumbprint malformed", `"m365_auth_mode":"certificate","m365_tenant_id":"t","m365_client_id":"c","m365_cert_thumbprint":"not-hex"`, "m365_cert_thumbprint"},
		{"certificate: with public certificate", `"m365_auth_mode":"certificate","m365_tenant_id":"t","m365_client_id":"c","m365_cert_thumbprint":"` + testThumbprint + `","m365_cert_public":"-----BEGIN CERTIFICATE-----\nAA==\n-----END CERTIFICATE-----\n"`, ""},
		{"certificate: with key id", `"m365_auth_mode":"certificate","m365_tenant_id":"t","m365_client_id":"c","m365_cert_thumbprint":"` + testThumbprint + `","m365_cert_key_id":"0b2b3b4b-1111-2222-3333-444455556666"`, ""},
		{"certificate: bad key id", `"m365_auth_mode":"certificate","m365_tenant_id":"t","m365_client_id":"c","m365_cert_thumbprint":"` + testThumbprint + `","m365_cert_key_id":"nope"`, "m365_cert_key_id"},
		{"certificate: bad object id", `"m365_auth_mode":"certificate","m365_tenant_id":"t","m365_client_id":"c","m365_cert_thumbprint":"` + testThumbprint + `","m365_app_object_id":"nope"`, "m365_app_object_id"},
		{"federated: nothing else", `"m365_auth_mode":"federated","m365_tenant_id":"t","m365_client_id":"c"`, ""},
		{"unknown mode", `"m365_auth_mode":"delegated","m365_tenant_id":"t","m365_client_id":"c"`, "m365_auth_mode"},
		{"tenant always required", `"m365_auth_mode":"federated","m365_client_id":"c"`, "m365_tenant_id"},
		{"client id optional before enrolment (certificate)", `"m365_auth_mode":"certificate","m365_tenant_id":"t","m365_cert_thumbprint":"` + testThumbprint + `"`, ""},
		{"client id optional before enrolment (federated)", `"m365_auth_mode":"federated","m365_tenant_id":"t"`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := BindAndValidateConfig(SharepointKey, spJSON(tc.extra))
			switch {
			case tc.wantErr == "" && err != nil:
				t.Fatalf("unexpected error: %v", err)
			case tc.wantErr != "" && err == nil:
				t.Fatalf("expected an error mentioning %s", tc.wantErr)
			case tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr):
				t.Fatalf("error %q does not mention %s", err, tc.wantErr)
			}
			if err != nil {
				var vErr ValidationError
				if !errors.As(err, &vErr) {
					t.Fatalf("error must be a ValidationError, got %T", err)
				}
			}
		})
	}
}

// A legacy record switched to the certificate mode by a PATCH (enrolment assistant flow).
func TestPatchConfig_SwitchLegacyRecordToCertificate(t *testing.T) {
	actual, err := BindAndValidateConfig(SharepointKey, spJSON(`"m365_tenant_id":"t","m365_client_id":"c","m365_client_secret":"s"`))
	if err != nil {
		t.Fatal(err)
	}

	_, err = PatchConfig(SharepointKey, actual, json.RawMessage(`{"m365_auth_mode":"certificate"}`))
	if err == nil || !strings.Contains(err.Error(), "m365_cert_thumbprint") {
		t.Fatalf("switching to certificate without thumbprint must fail, got %v", err)
	}

	patched, err := PatchConfig(SharepointKey, actual, json.RawMessage(`{"m365_auth_mode":"certificate","m365_cert_thumbprint":"`+testThumbprint+`"}`))
	if err != nil {
		t.Fatalf("switch to certificate: %v", err)
	}
	sp := patched.(*SharepointConfig)
	if sp.EffectiveMode() != M365AuthCertificate || sp.M365CertThumbprint != testThumbprint {
		t.Fatalf("patched config: %+v", sp.M365Auth)
	}
	// the old secret is still carried (harmless, masked by Strip); clearing it explicitly is allowed
	patched, err = PatchConfig(SharepointKey, sp, json.RawMessage(`{"m365_client_secret":""}`))
	if err != nil {
		t.Fatalf("clearing the secret in certificate mode must be allowed: %v", err)
	}
	if patched.(*SharepointConfig).M365ClientSecret != "" {
		t.Fatal("secret not cleared")
	}
}

func TestM365Auth_Enrolled(t *testing.T) {
	if (M365Auth{M365TenantID: "t"}).Enrolled() {
		t.Fatal("no client ID means not enrolled")
	}
	if !(M365Auth{M365TenantID: "t", M365ClientID: "c"}).Enrolled() {
		t.Fatal("client ID set means enrolled")
	}
}

func TestSharepointConfig_StripMasksSecretOnly(t *testing.T) {
	sp := &SharepointConfig{ReconfigurableSharepointConfig: ReconfigurableSharepointConfig{
		M365Auth: M365Auth{M365AuthMode: M365AuthCertificate, M365ClientSecret: "s", M365CertThumbprint: testThumbprint},
	}}
	stripped := sp.Strip().(SharepointConfig)
	if stripped.M365ClientSecret != "" || stripped.M365CertThumbprint != testThumbprint || sp.M365ClientSecret != "s" {
		t.Fatalf("Strip: %+v (original %+v)", stripped.M365Auth, sp.M365Auth)
	}
}

func TestGetConfigFields_M365Auth(t *testing.T) {
	fields, err := getConfigFields(SharepointConfig{})
	if err != nil {
		t.Fatal(err)
	}
	byKey := map[string]ConfigField{}
	for _, f := range fields {
		byKey[f.Key] = f
	}
	mode, ok := byKey["m365_auth_mode"]
	if !ok || len(mode.Enum) != 3 || mode.Required {
		t.Fatalf("m365_auth_mode field: %+v", mode)
	}
	secret := byKey["m365_client_secret"]
	if !secret.Password || secret.Required || secret.RequiredIf == nil || secret.RequiredIf.Key != "m365_auth_mode" || secret.RequiredIf.Value != "secret" {
		t.Fatalf("m365_client_secret field: %+v", secret)
	}
	thumb := byKey["m365_cert_thumbprint"]
	if thumb.RequiredIf == nil || thumb.RequiredIf.Value != "certificate" {
		t.Fatalf("m365_cert_thumbprint field: %+v", thumb)
	}
	if _, ok := byKey["m365_tenant_id"]; !ok || !byKey["m365_tenant_id"].Required {
		t.Fatal("m365_tenant_id must stay a required top-level field")
	}
	if byKey["m365_client_id"].Required {
		t.Fatal("m365_client_id must be optional at creation (filled by the enrolment)")
	}
}
