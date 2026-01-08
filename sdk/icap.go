package sdk

type ICAPConfig struct {
	CommonConnectorConfig
	Sampling ICAPSamplingConfig `json:"sampling" yaml:"sampling" mapstructure:"sampling" desc:"if enabled, and file size higher than treshold, file will be sampled (head and tail) before analysis"`
}

type ICAPSamplingConfig struct {
	Threshold int64 `json:"threshold" yaml:"threshold" mapstructure:"threshold" validate:"min=0" desc:"Sampling threshold for ICAP requests (disabled = 0)"`
	HeadSize  int64 `json:"head_size" yaml:"head_size" mapstructure:"head_size" validate:"min=0" desc:"Size of head sample in bytes"`
	TailSize  int64 `json:"tail_size" yaml:"tail_size" mapstructure:"tail_size" validate:"min=0" desc:"Size of tail sample in bytes"`
}
