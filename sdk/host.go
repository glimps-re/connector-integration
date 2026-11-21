package sdk

type HostConfig struct {
	CommonConnectorConfig    `yaml:",inline" mapstructure:",squash"`
	Workers                  int                    `json:"workers" mapstructure:"workers" yaml:"workers" validate:"min=1" desc:"Number of concurrent workers for file analysis (default: 4, affects CPU usage)"`
	ExtractWorkers           int                    `json:"extract_workers" mapstructure:"extract_workers" yaml:"extract_workers" validate:"min=1" desc:"Number of input archives processed simultaneously. Setting it too high makes many archives compete for the same analysis pool, slowing the completion of each individual input. Ratio should be at least 1:10 compared to analysis workers (1:20 can be a good choice to speed up input process time). (default: 2, used when extract is enabled)"`
	Extract                  bool                   `json:"extract" mapstructure:"extract" yaml:"extract" desc:"Enable archive extraction (archives are unpacked and contents scanned)"`
	ExtractMinSize           string                 `json:"extract_min_size" mapstructure:"extract_min_size" yaml:"extract_min_size" desc:"Minimum file size to attempt extraction (e.g., '8KB'). Files at or below this size are sent for analysis instead of being extracted (default: 8KB)"`
	RecursiveExtractMaxDepth int                    `json:"recursive_extract_max_depth" mapstructure:"recursive_extract_max_depth" yaml:"recursive_extract_max_depth" desc:"Maximum nesting level for recursive extraction. Beyond this depth, nested archives are sent for analysis instead of being extracted"`
	RecursiveExtractMaxSize  string                 `json:"recursive_extract_max_size" mapstructure:"recursive_extract_max_size" yaml:"recursive_extract_max_size" desc:"Maximum total size of extracted content across all nesting levels (e.g., '5GB'). When reached, remaining archives are sent for analysis instead of being extracted. Note: may exceed by up to one archive's extracted content"`
	RecursiveExtractMaxFiles int                    `json:"recursive_extract_max_files" mapstructure:"recursive_extract_max_files" yaml:"recursive_extract_max_files" desc:"Maximum number of files to extract recursively"`
	MaxFileSize              string                 `json:"max_file_size" mapstructure:"max_file_size" yaml:"max_file_size" desc:"Maximum file size to submit to GLIMPS Malware Detect (e.g., '100MB')"`
	Paths                    []string               `json:"paths" yaml:"paths" validate:"required,min=1" desc:"List of directories or files to monitor and scan (can be absolute or relative paths)"`
	FollowSymlinks           bool                   `json:"follow_symlinks" mapstructure:"follow_symlinks" yaml:"follow_symlinks" desc:"Follow symbolic links when scanning directories (if disabled, symlinks are skipped)"`
	Actions                  HostActionsConfig      `json:"actions" mapstructure:"actions" yaml:"actions" desc:"Actions to perform on scanned files (delete, quarantine, log, move, print)"`
	Quarantine               HostQuarantineConfig   `json:"quarantine" mapstructure:"quarantine" yaml:"quarantine" desc:"Configuration for encrypted quarantine storage of malware files"`
	Monitoring               HostMonitoringConfig   `json:"monitoring" mapstructure:"monitoring" yaml:"monitoring" desc:"Configuration for continuous directory monitoring and periodic re-scanning"`
	Move                     HostMoveConfig         `json:"move" mapstructure:"move" yaml:"move" desc:"Configuration for moving clean files from source to destination after scanning"`
	Print                    HostPrintConfig        `json:"print" mapstructure:"print" yaml:"print" desc:"Configuration for outputting scan reports to console or file"`
	WorkDir                  string                 `json:"work_dir" mapstructure:"work_dir" yaml:"work_dir" desc:"Base directory of the connector private working directory: a gmhost-* sub-directory is created there for archive extractions and plugin files, and removed at stop (default: the system temporary directory, TMPDIR honored). It must not be a monitored path. Use a local disk: a memory filesystem (tmpfs) uses RAM, a network filesystem is slow and its locks unreliable."`
	UnanalyzablePolicy       HostUnanalyzablePolicy `json:"unanalyzable_policy" mapstructure:"unanalyzable_policy" yaml:"unanalyzable_policy" desc:"What to do with files that cannot be analyzed"`
	FilepathFilter           HostFilepathFilter     `json:"filepath_filter" mapstructure:"filepath_filter" yaml:"filepath_filter" desc:"Path rules applied before analysis"`
	PluginsConfig            string                 `json:"plugins_config" yaml:"plugins_config" mapstructure:"plugins_config" desc:"Path to plugins configuration file (required for host connector plugin functionality)"`
}

// HostUnanalyzablePolicy sets the handling of files that cannot be analyzed.
// An empty value selects the connector default.
type HostUnanalyzablePolicy struct {
	TooBig        string `json:"too_big" mapstructure:"too_big" yaml:"too_big" validate:"omitempty,oneof=error pass mitigate" desc:"Files above the size limit that are not extracted (and members of an archive over it, or left out by the extraction limits): 'error' (left in place, default), 'pass' (delivered, marked not analyzed) or 'mitigate' (treated as malware)."`
	Empty         string `json:"empty" mapstructure:"empty" yaml:"empty" validate:"omitempty,oneof=skip pass mitigate" desc:"Empty files: 'pass' (delivered as a new empty file, marked empty in the reports, default; requires move.mode 'copy', refused with 'rename'), 'skip' (ignored) or 'mitigate' (treated as malware). When the key is absent and move.mode is 'rename', the connector uses 'skip' and warns."`
	Encrypted     string `json:"encrypted" mapstructure:"encrypted" yaml:"encrypted" validate:"omitempty,oneof=error pass mitigate" desc:"Archives encrypted with an unknown password: 'error' (left in place, default), 'pass' (delivered, marked encrypted) or 'mitigate' (treated as malware)."`
	ExtractError  string `json:"extract_error" mapstructure:"extract_error" yaml:"extract_error" validate:"omitempty,oneof=error as_file" desc:"Archives that the extractor finds damaged or inconsistent: 'error' (left in place and reported, default), or 'as_file' (analyzed as a plain file and delivered if nothing is detected, only when the extractor proved that no encrypted content is hidden; any other damaged archive follows the encrypted policy)."`
	AnalysisError string `json:"analysis_error" mapstructure:"analysis_error" yaml:"analysis_error" validate:"omitempty,oneof=error mitigate pass" desc:"Files GMalware failed to analyze: 'error' (left in place and reported, default), 'mitigate' (treated as malware, reason analysis-error) or 'pass' (delivered without a complete analysis, marked analysis-error: content not verified). A file with a detection is always malware, whatever this value."`
}

// HostFilepathFilter holds path rules, as Go regular expressions (RE2, linear
// time) matched on the path of a file. For a member of an archive the path is
// '<archive path>/<member path>'. A pattern that is empty, over 1024 bytes, not
// valid, or written in both lists is refused by the validation of the
// configuration (tag filepath_pattern).
type HostFilepathFilter struct {
	ForbiddenPaths []string `json:"forbidden_paths" mapstructure:"forbidden_paths" yaml:"forbidden_paths" validate:"max=256" desc:"Files whose path matches are treated as malware (at most 256 patterns per list, 1024 bytes each, 32 KiB in all); for a member of an archive the path is '<archive path>/<member path>'. A forbidden path wins over a skipped one."`
	SkippedPaths   []string `json:"skipped_paths" mapstructure:"skipped_paths" yaml:"skipped_paths" validate:"max=256" desc:"Files whose path matches (members of archives included, as '<archive path>/<member path>') are delivered without analysis, marked policy-skipped (at most 256 patterns per list, 1024 bytes each, 32 KiB in all). Risk: depositors choose the names of their files, folders and archive members; anchor these patterns on folders depositors cannot create."`
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
	PreScan              bool     `json:"prescan" mapstructure:"prescan" yaml:"prescan" desc:"Immediately scan all existing files in monitored paths when monitoring starts"`
	Period               Duration `json:"period" mapstructure:"period" yaml:"period" desc:"If set, enable periodic re-scan. Interval between periodic re-scans (e.g., '1h', '30m')"`
	ModificationDelay    Duration `json:"modification_delay" mapstructure:"modification_delay" yaml:"modification_delay" desc:"Wait time after file modification before scanning (e.g., '30s', prevents scanning incomplete writes)"`
	DisableFSWatcher     bool     `json:"disable_fs_watcher" mapstructure:"disable_fs_watcher" yaml:"disable_fs_watcher" desc:"Disable filesystem event watching and rely on the pre-scan and periodic rescan only (default: false). Requires monitoring.period. Use it on network filesystems that do not report events or where watches are limited."`
	MaxTrackedMemory     string   `json:"max_tracked_memory" mapstructure:"max_tracked_memory" yaml:"max_tracked_memory" desc:"Memory bound of the table of files the monitor tracks while they wait for their modification delay (e.g., '256MiB', default: 256MiB). Below the bound nobody is limited; when it is full, the owner holding the most yields first and its files are found again at the next rescan (only when written to again if monitoring.period is 0)."`
	MaxTrackedDirPercent int      `json:"max_tracked_dir_percent" mapstructure:"max_tracked_dir_percent" yaml:"max_tracked_dir_percent" validate:"min=0,max=100" desc:"Share of the tracked table, in percent (default: 25), above which an owner (a user or session folder) is visited first when the table is full; it is not a ceiling."`
}

type HostQuarantineConfig struct {
	Location string `json:"location" mapstructure:"location" yaml:"location" desc:"Directory path where quarantined files are stored (files are encrypted with .lock extension)"`
	Password string `json:"password" mapstructure:"password" yaml:"password" password:"true" desc:"Password for encrypting quarantined files (required to restore files later)"`
	Registry string `json:"registry" mapstructure:"registry" yaml:"registry" desc:"Path to the database that store quarantined and restored file entry (leave empty for in-memory store, lost on restart)"`
	Format   string `json:"format" mapstructure:"format" yaml:"format" validate:"omitempty,oneof=tar zip" desc:"Container format for new quarantined files: 'tar' (encrypted tar, default) or 'zip' (password-protected ZIP openable with standard archive tools; existing files stay readable whatever the value). With 'zip' the quarantine password must be considered public: do not reuse it elsewhere"`
}

type HostMoveConfig struct {
	Destination   string `json:"destination" mapstructure:"destination" yaml:"destination" desc:"Target directory for moving clean files (preserves subdirectory structure)"`
	Source        string `json:"source" mapstructure:"source" yaml:"source" desc:"Source directory filter (only clean files within this path are moved to destination)"`
	Mode          string `json:"mode" mapstructure:"mode" yaml:"mode" validate:"omitempty,oneof=copy rename" desc:"How clean files are moved: 'copy' (default: the analyzed file is copied, the copy is checked against the analyzed SHA256, then the source is deleted, so nothing a depositor still holds open can reach the destination; costs one more read and write) or 'rename' (the file itself is moved: cheaper, but a writer that kept it open can still change it; a warning is raised at each configuration)."`
	MinThroughput string `json:"min_throughput" mapstructure:"min_throughput" yaml:"min_throughput" desc:"Minimum expected copy throughput per second in copy mode (e.g., '10MiB', default: 10MiB); a copy gets size/throughput plus one minute, and a slower one is abandoned for that attempt and retried."`
}
