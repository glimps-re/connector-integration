package sdk

type DummyConfig struct {
	ReconfigurableDummyConfig
	DummyString2 string `json:"dummy_string_2" reconfigurable:"false"`
}

type ReconfigurableDummyConfig struct {
	CommonConnectorConfig
	DummyString string        `json:"dummy_string" validate:"required" `
	Password    string        `json:"password,omitempty" password:"true"` //nolint:gosec // config field, not an exposed secret
	Enum        string        `json:"enum" validate:"required,oneof=quarantine delete log" desc:"Action to perform when a file is detected as malware."`
	Objects     []DummyObject `json:"objects" desc:"Array of objects"`
}

type DummyObject struct {
	SubField1 string              `json:"sub_field_1" desc:"Sub field 1"`
	SubField2 int                 `json:"sub_field_2" desc:"Sub field 2"`
	SubField3 []string            `json:"sub_field_3" desc:"Sub field 3"`
	SubField4 []DummyNestedObject `json:"sub_field_4" desc:"Sub field 4"`
}

type DummyNestedObject struct {
	NestedField1 string `json:"nested_field_1" desc:"Nested field 1"`
	NestedField2 string `json:"nested_field_2" desc:"Nested field 2"`
}

type DummyHelmConf struct {
	ConsoleConfig
	DummyField1 string
}

func (c *DummyConfig) Strip() any {
	cc := *c
	cc.Password = ""
	return cc
}

func (c *DummyConfig) GetHelmConfig(consoleConfig ConsoleConfig) (helmConfig any, err error) {
	helmConfig = DummyHelmConf{
		ConsoleConfig: consoleConfig,
		DummyField1:   c.DummyString,
	}
	return
}
