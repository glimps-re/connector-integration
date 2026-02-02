package sdk

type HostConfig struct {
	CommonConnectorConfig    `yaml:",inline" mapstructure:",squash"`
	Workers                  int                  `json:"workers" mapstructure:"workers" yaml:"workers" validate:"min=1" desc:"Number of concurrent workers for file analysis (default: 4, affects CPU usage)"`
	ExtractWorkers           int                  `json:"extract_workers" mapstructure:"extract_workers" yaml:"extract_workers" validate:"min=1" desc:"Number of concurrent workers for archive extraction (default: 2, used when extract is enabled)"`
	Extract                  bool                 `json:"extract" mapstructure:"extract" yaml:"extract" desc:"Enable archive extraction for files exceeding max_file_size (archives are unpacked and contents scanned)"`
	RecursiveExtractMaxDepth int                  `json:"recursive_extract_max_depth" mapstructure:"recursive_extract_max_depth" yaml:"recursive_extract_max_depth" desc:"Maximum nesting level for extraction. Beyond it, files are directly send for analyze (if possible)"`
	RecursiveExtractMaxSize  string               `json:"recursive_extract_max_size" mapstructure:"recursive_extract_max_size" yaml:"recursive_extract_max_size" desc:"Maximum size for extracted content from a root archive (across all nesting levels), e.g. '5GB'. Beyond it, files are directly send for analyze. Note: the actual total may exceed by up to one archive's extracted content"`
	RecursiveExtractMaxFiles int                  `json:"recursive_extract_max_files" mapstructure:"recursive_extract_max_files" yaml:"recursive_extract_max_files" desc:"Maximum number of files to extract recursively"`
	MaxFileSize              string               `json:"max_file_size" mapstructure:"max_file_size" yaml:"max_file_size" desc:"Maximum file size to send for analyze (e.g., '100MB'). Files exceeding this are extracted if 'extract' is enabled, otherwise rejected"`
	Paths                    []string             `json:"paths" yaml:"paths" validate:"required,min=1" desc:"List of directories or files to monitor and scan (can be absolute or relative paths)"`
	FollowSymlinks           bool                 `json:"follow_symlinks" yaml:"follow_symlinks" desc:"Follow symbolic links when scanning directories (if disabled, symlinks are skipped)"`
	Actions                  HostActionsConfig    `json:"actions" mapstructure:"actions" yaml:"actions" desc:"Actions to perform on scanned files (delete, quarantine, log, move, print)"`
	Quarantine               HostQuarantineConfig `json:"quarantine" mapstructure:"quarantine" yaml:"quarantine" desc:"Configuration for encrypted quarantine storage of malware files"`
	Monitoring               HostMonitoringConfig `json:"monitoring" mapstructure:"monitoring" yaml:"monitoring" desc:"Configuration for continuous directory monitoring and periodic re-scanning"`
	Move                     HostMoveConfig       `json:"move" mapstructure:"move" yaml:"move" desc:"Configuration for moving clean files from source to destination after scanning"`
	Print                    HostPrintConfig      `json:"print" mapstructure:"print" yaml:"print" desc:"Configuration for outputting scan reports to console or file"`
	PluginsConfig            string               `json:"plugins_config" yaml:"plugins_config" mapstructure:"plugins_config" desc:"Path to plugins configuration file (required for host connector plugin functionality)"`
}

type HostPrintConfig struct {
	Location string `json:"location" mapstructure:"location" yaml:"location" desc:"File path for scan reports (leave empty to print to stdout)"`
	Verbose  bool   `json:"verbose" mapstructure:"verbose" yaml:"verbose" desc:"Report all scanned files, including clean files (not just malware detections)"`
}

type HostActionsConfig struct {
	Delete     bool `json:"delete" mapstructure:"delete" yaml:"delete" desc:"Delete detected malware files automatically"`
	Quarantine bool `json:"quarantine" mapstructure:"quarantine" yaml:"quarantine" desc:"Move malware files to encrypted quarantine storage (requires quarantine configuration)"`
	Print      bool `json:"print" mapstructure:"print" yaml:"print" desc:"Output scan results to console or file (see print configuration)"`
	Log        bool `json:"log" mapstructure:"log" yaml:"log" desc:"Log malware detections (written to connector logs)"`
	Move       bool `json:"move" mapstructure:"move" yaml:"move" desc:"Move clean files from source to destination after scanning (requires move configuration)"`
}

type HostMonitoringConfig struct {
	PreScan           bool     `json:"prescan" mapstructure:"prescan" yaml:"prescan" desc:"Immediately scan all existing files in monitored paths when monitoring starts"`
	Period            Duration `json:"period" mapstructure:"period" yaml:"period" desc:"If set, enable periodic re-scan. Interval between periodic re-scans (e.g., '1h', '30m')"`
	ModificationDelay Duration `json:"modification_delay" mapstructure:"modification_delay" yaml:"modification_delay" desc:"Wait time after file modification before scanning (e.g., '30s', prevents scanning incomplete writes)"`
}

type HostQuarantineConfig struct {
	Location string `json:"location" mapstructure:"location" yaml:"location" desc:"Directory path where quarantined files are stored (files are encrypted with .lock extension)"`
	Password string `json:"password" mapstructure:"password" yaml:"password" password:"true" desc:"Password for encrypting quarantined files (required to restore files later)"` //nolint:gosec // config field, not an exposed secret
	Registry string `json:"registry" mapstructure:"registry" yaml:"registry" desc:"Path to the database that store quarantined and restored file entry (leave empty for in-memory store, lost on restart)"`
}

type HostMoveConfig struct {
	Destination string `json:"destination" mapstructure:"destination" yaml:"destination" desc:"Target directory for moving clean files (preserves subdirectory structure)"`
	Source      string `json:"source" mapstructure:"source" yaml:"source" desc:"Source directory filter (only clean files within this path are moved to destination)"`
}
