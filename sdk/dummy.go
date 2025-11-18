package sdk

type DummyConfig struct {
	CommonConnectorConfig
	DummyString  string `json:"dummy_string" validate:"required" `
	DummyString2 string `json:"dummy_string_2" reconfigurable:"false"` // non-reconfigurable field used in TU
	Password     string `json:"password,omitempty"`
}

type ReconfigurableDummyConfig struct {
	CommonConnectorConfig
	DummyString string `json:"dummy_string" validate:"required" `
	Password    string `json:"password"`
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
