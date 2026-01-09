package sdk

type ICAPConfig struct {
	CommonConnectorConfig
	SamplingThreshold int64 `json:"sampling_threshold" yaml:"sampling_threshold" mapstructure:"sampling_threshold" validate:"min=0" desc:"Sampling threshold for ICAP requests"`
	SamplingHeadSize  int64 `json:"sampling_head_size" yaml:"sampling_head_size" mapstructure:"sampling_head_size" validate:"min=0" desc:"Size of head sample in bytes"`
	SamplingTailSize  int64 `json:"sampling_tail_size" yaml:"sampling_tail_size" mapstructure:"sampling_tail_size" validate:"min=0" desc:"Size of tail sample in bytes"`
}
