package sdk

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestNewConnectorsTypesLoader(t *testing.T) {
	tests := []struct {
		name           string
		wantConnectors map[string]bool
		wantErr        bool
	}{
		{
			name: "ok",
			wantConnectors: map[string]bool{
				M365Key:       true,
				DummyKey:      true,
				ICAPKey:       true,
				SharepointKey: true,
				HostKey:       true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotC, err := NewConnectorsTypesLoader(true)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewConnectorsTypesLoader() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			for k := range gotC.connectorsTypes {
				if _, ok := tt.wantConnectors[k]; !ok {
					t.Errorf("NewConnectorsTypesLoader() got connector %s, didn't want it", k)
					continue
				}
				tt.wantConnectors[k] = false
			}
			for k, v := range tt.wantConnectors {
				if v {
					t.Errorf("NewConnectorsTypesLoader() connector %s not loaded", k)
				}
			}
		})
	}
}

func TestConnectorsTypesLoader_GetConnectorTypes(t *testing.T) {
	type fields struct {
		connectorsTypes map[string]ConnectorType
	}
	tests := []struct {
		name               string
		fields             fields
		wantConnectorTypes []ConnectorType
		wantErr            bool
	}{
		{
			name: "ok",
			fields: fields{
				connectorsTypes: map[string]ConnectorType{
					"connector1": {
						Name:        "Connector 1",
						ID:          "connector1",
						Description: "This is connector 1",
						SetupSteps: []Step{
							{
								Name:        "Setup step #1",
								Description: "Deploy it",
							},
						},
						MitigationInfoType: "infotype1",
					},
					"connector2": {
						Name:        "Connector 2",
						ID:          "connector2",
						Description: "This is connector 2",
						SetupSteps: []Step{
							{
								Name:        "Setup step #1",
								Description: "Deploy it",
							},
						},
						MitigationInfoType: "infotype2",
					},
				},
			},
			wantConnectorTypes: []ConnectorType{
				{
					Name:        "Connector 1",
					ID:          "connector1",
					Description: "This is connector 1",
					SetupSteps: []Step{
						{
							Name:        "Setup step #1",
							Description: "Deploy it",
						},
					},
					MitigationInfoType: "infotype1",
				},
				{
					Name:        "Connector 2",
					ID:          "connector2",
					Description: "This is connector 2",
					SetupSteps: []Step{
						{
							Name:        "Setup step #1",
							Description: "Deploy it",
						},
					},
					MitigationInfoType: "infotype2",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := ConnectorTypeLoader{
				connectorsTypes: tt.fields.connectorsTypes,
			}
			gotConnectorTypes := c.GetConnectorTypes()
			if diff := cmp.Diff(gotConnectorTypes, tt.wantConnectorTypes); diff != "" {
				t.Errorf("ConnectorsTypesLoader.GetConnectorTypes() diff(got-want)=%s", diff)
			}
		})
	}
}

func TestConnectorsTypesLoader_GetTemplatedLaunchSteps(t *testing.T) {
	type args struct {
		connectorType string
		config        LaunchStepConfig
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "error connector type not found",
			args: args{
				connectorType: "toto",
			},
			wantErr: true,
		},
		{
			name: "ok dummy",
			args: args{
				config: LaunchStepConfig{
					ConnectorConfig: DummyConfig{
						ReconfigurableDummyConfig: ReconfigurableDummyConfig{
							CommonConnectorConfig: CommonConnectorConfig{
								GMalwareAPIURL:   "toto",
								GMalwareAPIToken: "totoken",
							},
							DummyString: "test",
						},
					},
				},
				connectorType: DummyKey,
			},
		},
		{
			name: "ok m365",
			args: args{
				config: LaunchStepConfig{
					ConnectorConfig: M365Config{
						CommonConnectorConfig: CommonConnectorConfig{
							GMalwareAPIURL:   "http://ggp.re",
							GMalwareAPIToken: "token",
						},
						M365ClientID:     "clientID",
						M365ClientTenant: "tenantID",
						M365ClientSecret: "secret",
						JournalRecipient: "connector@mycorp.com",
						HeaderTokenValue: "secret-token",
					},
				},
				connectorType: M365Key,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewConnectorsTypesLoader(true)
			if err != nil {
				t.Fatalf("could not load connector type loader")
			}
			_, err = c.GetTemplatedLaunchSteps(tt.args.connectorType, tt.args.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConnectorsTypesLoader.GetTemplatedLaunchSteps() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestConnectorTypeLoader_GetConnectorFile(t *testing.T) {
	type fields struct {
		connectorsTypes map[string]ConnectorType
	}
	type args struct {
		connectorType string
		fileID        string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "ok",
			fields: fields{
				connectorsTypes: map[string]ConnectorType{
					M365Key:  {},
					DummyKey: {},
					ICAPKey:  {},
				},
			},
			args: args{
				connectorType: M365Key,
				fileID:        "GLIMPS-M365-Connector.ps1",
			},
		},
		{
			name: "error connector not found",
			fields: fields{
				connectorsTypes: map[string]ConnectorType{
					M365Key:  {},
					DummyKey: {},
					ICAPKey:  {},
				},
			},
			args: args{
				connectorType: "toto",
				fileID:        "GLIMPS-M365-Connector.ps1",
			},
			wantErr: true,
		},
		{
			name: "ok",
			fields: fields{
				connectorsTypes: map[string]ConnectorType{
					M365Key:  {},
					DummyKey: {},
					ICAPKey:  {},
				},
			},
			args: args{
				connectorType: M365Key,
				fileID:        "GLIMPS-M365-Connector.ps2",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := ConnectorTypeLoader{
				connectorsTypes: tt.fields.connectorsTypes,
			}
			_, err := c.GetConnectorFile(tt.args.connectorType, tt.args.fileID)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConnectorTypeLoader.GetConnectorFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestConnectorTypeLoader_GetTemplatedDockerCompose(t *testing.T) {
	type args struct {
		connectorType string
		config        ConsoleConfig
	}
	tests := []struct {
		name              string
		args              args
		wantDockerCompose string
		wantErr           bool
	}{
		{
			name: "error",
			args: args{
				connectorType: "toto",
				config:        ConsoleConfig{},
			},
			wantErr: true,
		},
		{
			name: "ok m365",
			args: args{
				connectorType: M365Key,
				config:        ConsoleConfig{},
			},
		},
		{
			name: "ok dummy",
			args: args{
				connectorType: DummyKey,
				config:        ConsoleConfig{},
			},
		},
		{
			name: "ok icap",
			args: args{
				connectorType: ICAPKey,
				config:        ConsoleConfig{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewConnectorsTypesLoader(true)
			if err != nil {
				t.Fatalf("could not init connector types loader, err: %v", err)
			}
			_, err = c.GetTemplatedDockerCompose(tt.args.connectorType, tt.args.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConnectorTypeLoader.GetTemplatedDockerCompose() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestConnectorTypeLoader_GetTemplatedHelm(t *testing.T) {
	type args struct {
		connectorType string
		config        any
	}
	tests := []struct {
		name              string
		args              args
		wantDockerCompose string
		wantErr           bool
	}{
		{
			name: "error unknown connector type",
			args: args{
				connectorType: "toto",
				config:        ConsoleConfig{},
			},
			wantErr: true,
		},
		{
			name: "ok sharepoint",
			args: args{
				connectorType: SharepointKey,
				config: SharepointHelmConf{
					ConsoleConfig: ConsoleConfig{
						APIKey: "api-key",
					},
					SharepointWebhookHost: "client1.sharepoint.monserveur.glimps.lan",
				},
			},
		},
		{
			name: "ok dummy",
			args: args{
				connectorType: DummyKey,
				config: DummyHelmConf{
					ConsoleConfig: ConsoleConfig{
						APIKey: "api-key",
					},
					DummyField1: "custom",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewConnectorsTypesLoader(true)
			if err != nil {
				t.Fatalf("could not init connector types loader, err: %v", err)
			}
			_, err = c.GetTemplatedHelm(tt.args.connectorType, tt.args.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConnectorTypeLoader.GetTemplatedHelm() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func Test_getConfigFields(t *testing.T) {
	type TestCommonConnectorConfig struct {
		ClientName          string   `json:"client_name" validate:"required" desc:"Name of the client"`
		GMalwareAPIURL      string   `json:"gmalware_api_url" validate:"required,url" desc:"GLIMPS Malware API URL"`
		GMalwareAPIToken    string   `json:"gmalware_api_token" validate:"required" desc:"GLIMPS Malware API Token"`
		GMalwareNoCertCheck bool     `json:"gmalware_no_cert_check" desc:"Disable certificate check for GLIMPS Malware"`
		GMalwareUserTags    []string `json:"gmalware_user_tags" desc:"List of tags set by connector on GLIMPS Malware detect submission"`
		OtherField          int      `json:"other_field" desc:"Just another field" reconfigurable:"false"`
		SecretField         string   `json:"secret_field" password:"true" desc:"Just another field" reconfigurable:"false"`
		Duration            Duration `json:"duration" desc:"It's a duration"`
	}
	type testWithSubconf struct {
		TestCommonConnectorConfig
		OtherFieldAgain int `json:"other_field_again" desc:"oh, another field"`
	}
	type args struct {
		config any
	}
	tests := []struct {
		name             string
		args             args
		wantConfigFields []ConfigField
		wantErr          bool
	}{
		{
			name: "common config",
			args: args{
				config: TestCommonConnectorConfig{},
			},
			wantConfigFields: []ConfigField{
				{
					Name:           "ClientName",
					Key:            "client_name",
					Type:           "string",
					Description:    "Name of the client",
					Required:       true,
					Validation:     []FrontValidation{},
					Reconfigurable: true,
					DefaultValue:   "",
				},
				{
					Name:           "GMalwareAPIURL",
					Key:            "gmalware_api_url",
					Type:           "string",
					Description:    "GLIMPS Malware API URL",
					Required:       true,
					Validation:     []FrontValidation{FrontValidURL},
					Reconfigurable: true,
					DefaultValue:   "",
				},
				{
					Name:           "GMalwareAPIToken",
					Key:            "gmalware_api_token",
					Type:           "string",
					Description:    "GLIMPS Malware API Token",
					Required:       true,
					Validation:     []FrontValidation{},
					Reconfigurable: true,
					DefaultValue:   "",
				},
				{
					Name:           "GMalwareNoCertCheck",
					Key:            "gmalware_no_cert_check",
					Type:           "boolean",
					Description:    "Disable certificate check for GLIMPS Malware",
					Required:       false,
					Validation:     []FrontValidation{},
					Reconfigurable: true,
					DefaultValue:   false,
				},
				{
					Name:           "GMalwareUserTags",
					Key:            "gmalware_user_tags",
					Type:           "string[]",
					Description:    "List of tags set by connector on GLIMPS Malware detect submission",
					Required:       false,
					Validation:     []FrontValidation{},
					Reconfigurable: true,
					DefaultValue:   []string{},
				},
				{
					Name:         "OtherField",
					Key:          "other_field",
					Type:         "number",
					Description:  "Just another field",
					Validation:   []FrontValidation{},
					DefaultValue: 0,
				},
				{
					Name:         "SecretField",
					Key:          "secret_field",
					Type:         "string",
					Description:  "Just another field",
					Validation:   []FrontValidation{},
					Properties:   []ConfigField{},
					DefaultValue: string(""),
					Password:     true,
				},
				{
					Name:           "Duration",
					Key:            "duration",
					Type:           "string",
					Description:    "It's a duration",
					Validation:     []FrontValidation{},
					Reconfigurable: true,
					DefaultValue:   Duration(0),
				},
			},
		},
		{
			name: "ok with composed struct",
			args: args{
				config: testWithSubconf{},
			},
			wantConfigFields: []ConfigField{
				{
					Name:           "ClientName",
					Key:            "client_name",
					Type:           "string",
					Description:    "Name of the client",
					Required:       true,
					Validation:     []FrontValidation{},
					Reconfigurable: true,
					DefaultValue:   "",
				},
				{
					Name:           "GMalwareAPIURL",
					Key:            "gmalware_api_url",
					Type:           "string",
					Description:    "GLIMPS Malware API URL",
					Required:       true,
					Validation:     []FrontValidation{FrontValidURL},
					Reconfigurable: true,
					DefaultValue:   "",
				},
				{
					Name:           "GMalwareAPIToken",
					Key:            "gmalware_api_token",
					Type:           "string",
					Description:    "GLIMPS Malware API Token",
					Required:       true,
					Validation:     []FrontValidation{},
					Reconfigurable: true,
					DefaultValue:   "",
				},
				{
					Name:           "GMalwareNoCertCheck",
					Key:            "gmalware_no_cert_check",
					Type:           "boolean",
					Description:    "Disable certificate check for GLIMPS Malware",
					Required:       false,
					Validation:     []FrontValidation{},
					Reconfigurable: true,
					DefaultValue:   false,
				},
				{
					Name:           "GMalwareUserTags",
					Key:            "gmalware_user_tags",
					Type:           "string[]",
					Description:    "List of tags set by connector on GLIMPS Malware detect submission",
					Required:       false,
					Validation:     []FrontValidation{},
					Reconfigurable: true,
					DefaultValue:   []string{},
				},
				{
					Name:         "OtherField",
					Key:          "other_field",
					Type:         "number",
					Description:  "Just another field",
					Validation:   []FrontValidation{},
					DefaultValue: 0,
				},
				{
					Name:         "SecretField",
					Key:          "secret_field",
					Type:         "string",
					Description:  "Just another field",
					Validation:   []FrontValidation{},
					Properties:   []ConfigField{},
					DefaultValue: string(""),
					Password:     true,
				},
				{
					Name:           "Duration",
					Key:            "duration",
					Type:           "string",
					Description:    "It's a duration",
					Validation:     []FrontValidation{},
					Reconfigurable: true,
					DefaultValue:   Duration(0),
				},
				{
					Name:           "OtherFieldAgain",
					Key:            "other_field_again",
					Type:           "number",
					Description:    "oh, another field",
					Validation:     []FrontValidation{},
					Reconfigurable: true,
					DefaultValue:   0,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configFields, err := getConfigFields(tt.args.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("getConfigFields() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if diff := cmp.Diff(configFields, tt.wantConfigFields, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("getConfigFields() config fields did not match expected, diff = %v", diff)
			}
		})
	}
}
