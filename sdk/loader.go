package sdk

import (
	"archive/zip"
	"bytes"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"text/template"
	"time"

	"gopkg.in/yaml.v3"
)

//go:embed connectors/*
var configFS embed.FS

const (
	DummyKey      = "dummy"
	M365Key       = "m365"
	ICAPKey       = "icap"
	SharepointKey = "sharepoint"
	HostKey       = "host"

	connectorsFolderName = "connectors"
	helmFolderName       = "helm"
	helmValuesFileName   = "values.yaml"

	dockerComposeFileName = "docker-compose.yaml"
	connectorFileName     = "connector.yaml"
	logoFileName          = "logo.png"
)

type ConnectorTypeLoader struct {
	connectorsTypes map[string]ConnectorType
}

func NewConnectorsTypesLoader(dev bool) (connLoader ConnectorTypeLoader, err error) {
	entries, err := configFS.ReadDir(connectorsFolderName)
	if err != nil {
		return
	}
	connLoader = ConnectorTypeLoader{
		connectorsTypes: make(map[string]ConnectorType),
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		connectorType, getErr := getConnectorDesc(entry.Name(), filepath.Join(connectorsFolderName, entry.Name()), dev)
		switch {
		case errors.Is(getErr, ErrDevConnector):
			continue
		case getErr != nil:
			err = fmt.Errorf("could not read connector %s file properly, error: %w", entry.Name(), getErr)
			return
		default:
			connLoader.connectorsTypes[connectorType.ID] = connectorType
		}
	}
	return
}

type ConnectorType struct {
	Name               string        `yaml:"name" json:"name"`
	ID                 string        `yaml:"-" json:"id" desc:"e.g. icap,sharepoint,m365"`
	Description        string        `yaml:"description" json:"description"`
	DevOnly            bool          `yaml:"dev_only" json:"-"`
	SetupSteps         []Step        `yaml:"setup_steps" json:"setup" desc:"prerequisite steps"`
	Configs            []ConfigField `json:"config_fields"`
	LaunchSteps        []Step        `yaml:"launch_steps" json:"-" desc:"steps to deploy connector"`
	MitigationInfoType string        `yaml:"mitigation_info_type" json:"mitigation_info_type" desc:"what's connector treat : file, email, url"`
	Logo               string        `yaml:"-" json:"logo"`
	Helm               bool          `yaml:"-" json:"helm" desc:"whether helm chart is available for this connector type"`
	DockerCompose      bool          `yaml:"-" json:"docker_compose" desc:"whether docker compose is available for this connector type"`
	HelmVersion        string        `yaml:"-" json:"helm_version" desc:"helm chart version"`
}

type ConnectorFile struct {
	Name        string `yaml:"name" json:"name"`
	Filename    string `yaml:"filename" json:"filename"`
	Description string `yaml:"description" json:"description"`
}

type Step struct {
	Name        string   `yaml:"name" json:"name"`
	Description string   `yaml:"description" json:"description" desc:"used as a template (=> can contain config field names, e.g. {{ .GMalwareAPIURL }})"`
	Files       []string `yaml:"files" json:"files"`
}

type FrontValidation string

const (
	FrontValidURL   FrontValidation = "url"
	FrontValidEmail FrontValidation = "email"

	// TO DO : add these to our custom validator
	FrontValidDuration FrontValidation = "duration"
	FrontValidFileSize FrontValidation = "filesize"

	ReconfigurableTag string = "reconfigurable"
)

// Infos for the frontend
type ConfigField struct {
	Name           string            `json:"name" desc:"display name"`
	Key            string            `json:"key"`
	Type           ConfigFieldType   `json:"type" desc:"front type. either: boolean,string,number,string[],object"`
	Description    string            `json:"description"`
	Required       bool              `json:"required"`
	Validation     []FrontValidation `json:"validation" desc:"to perform specific types of validation"`
	Reconfigurable bool              `json:"reconfigurable" desc:"true to allow field update (=reconfiguration) (false => user will only be able to see current value)."`
	Properties     []ConfigField     `json:"properties"`
	DefaultValue   any               `json:"default_value"`
}

type ConfigFieldType string

const (
	Number      ConfigFieldType = "number"
	String      ConfigFieldType = "string"
	StringArray ConfigFieldType = "string[]"
	Boolean     ConfigFieldType = "boolean"
	Object      ConfigFieldType = "object"
)

// These configs should contain directly a list of fields (no nested struct) whose name
// are explicit enough (because their name are currently used as displayed name by the frontend).
type ConnectorConfig interface {
	DummyConfig | M365Config | ICAPConfig | SharepointConfig | HostConfig
}

// Connector *Config must satisfy this interface if it needs to lint some secrets
type ConfigStripper interface {
	Strip() any
}

// Connector *Config must satisfy this interface if it provide a helm deployment
type ConfigHelmer interface {
	GetHelmConfig(consoleConfig ConsoleConfig) (helmConfig any, err error)
}

type CommonConnectorConfig struct {
	GMalwareAPIURL      string   `json:"gmalware_api_url" yaml:"gmalware_api_url" mapstructure:"gmalware_api_url" validate:"required,url" desc:"GLIMPS Malware API URL" `
	GMalwareAPIToken    string   `json:"gmalware_api_token" yaml:"gmalware_api_token" mapstructure:"gmalware_api_token" validate:"required" desc:"GLIMPS Malware API Token" `
	GMalwareNoCertCheck bool     `json:"gmalware_no_cert_check" yaml:"gmalware_no_cert_check" mapstructure:"gmalware_no_cert_check" desc:"Disable certificate check for GLIMPS Malware"`
	GMalwareUserTags    []string `json:"gmalware_user_tags" yaml:"gmalware_user_tags" mapstructure:"gmalware_user_tags" desc:"List of tags set by connector on GLIMPS Malware detect submission"`
	GMalwareTimeout     Duration `json:"gmalware_timeout" yaml:"gmalware_timeout" mapstructure:"gmalware_timeout" desc:"gmalware submission timeout" `
	GMalwareBypassCache bool     `json:"gmalware_bypass_cache" yaml:"gmalware_bypass_cache" mapstructure:"gmalware_bypass_cache" desc:"bypass gmalware"`
	GMalwareSyndetect   bool     `json:"gmalware_syndetect" yaml:"gmalware_syndetect" mapstructure:"gmalware_syndetect" desc:"use syndetect"`
	Debug               bool     `json:"debug" yaml:"debug" mapstructure:"debug" desc:"Enable debug log"`
}

type ConsoleConfig struct {
	APIKey   string
	URL      string
	Insecure bool
}

func (c ConnectorTypeLoader) GetConnectorTypes() (connectorTypes []ConnectorType) {
	for _, v := range c.connectorsTypes {
		connectorTypes = append(connectorTypes, v)
	}
	slices.SortStableFunc(connectorTypes, func(a, b ConnectorType) int {
		switch {
		case a.ID > b.ID:
			return 1
		case a.ID == b.ID:
			return 0
		default:
			return -1
		}
	})
	return
}

func (c ConnectorTypeLoader) GetConnectorType(typeID string) (connectorType ConnectorType, err error) {
	for _, ct := range c.connectorsTypes {
		if ct.ID == typeID {
			connectorType = ct
			return
		}
	}
	err = fmt.Errorf("connector type %s not found", typeID)
	return
}

var (
	ErrConnectorTypeNotFound = errors.New("error connector type not found")
	ErrConnectorFileNotFound = errors.New("error connector file not found")
	ErrBadConfigFieldStruct  = errors.New("error config field has unknown type, could not prepare config form properly")
	ErrDevConnector          = errors.New("error connector only available in dev mode")
)

type LaunchStepConfig struct {
	ConnectorConfig any
	ConsoleConfig   ConsoleConfig
}

func (c ConnectorTypeLoader) GetTemplatedLaunchSteps(connectorType string, config LaunchStepConfig) (steps []Step, err error) {
	if _, ok := c.connectorsTypes[connectorType]; !ok {
		err = ErrConnectorTypeNotFound
		return
	}
	steps = make([]Step, 0, len(c.connectorsTypes[connectorType].LaunchSteps))
	for _, step := range c.connectorsTypes[connectorType].LaunchSteps {
		tmpl, tmplErr := template.New("step").Parse(step.Description)
		if tmplErr != nil {
			err = tmplErr
			return
		}
		buff := bytes.NewBuffer(nil)
		if err = tmpl.Execute(buff, config); err != nil {
			return
		}
		step.Description = buff.String()
		steps = append(steps, step)
	}
	return
}

func (c ConnectorTypeLoader) GetTemplatedDockerCompose(connectorTypeID string, config ConsoleConfig) (dockerCompose string, err error) {
	connectorType, ok := c.connectorsTypes[connectorTypeID]
	if !ok {
		err = ErrConnectorTypeNotFound
		return
	}

	if !connectorType.DockerCompose {
		err = ErrNoComposeForConnector
		return
	}

	rawCompose, err := configFS.ReadFile(filepath.Join(connectorsFolderName, connectorTypeID, dockerComposeFileName))
	if err != nil {
		return
	}
	tmpl, err := template.New("compose").Parse(string(rawCompose))
	if err != nil {
		return
	}
	b := bytes.NewBuffer(nil)
	if err = tmpl.Execute(b, config); err != nil {
		return
	}
	dockerCompose = b.String()
	return
}

func (c ConnectorTypeLoader) GetTemplatedHelm(connectorTypeID string, config any) (r io.Reader, err error) {
	connectorType, ok := c.connectorsTypes[connectorTypeID]
	if !ok {
		err = ErrConnectorTypeNotFound
		return
	}

	if !connectorType.Helm {
		err = ErrNoHelmForConnector
		return
	}

	// get helm values templated
	rawValues, err := configFS.ReadFile(filepath.Join(connectorsFolderName, connectorTypeID, helmFolderName, helmValuesFileName))
	if err != nil {
		return
	}
	tmpl, err := template.New("helmValues").Parse(string(rawValues))
	if err != nil {
		return
	}
	// add files to archive
	buffer := bytes.NewBuffer(nil)
	archive := zip.NewWriter(buffer)
	defer func() {
		if closeErr := archive.Close(); closeErr != nil {
			logger.Warn("error closing zip", slog.String("error", closeErr.Error()))
		}
	}()
	w, err := archive.Create(helmValuesFileName)
	if err != nil {
		return
	}

	if err = tmpl.Execute(w, config); err != nil {
		return
	}

	helmFileName := fmt.Sprintf("%v-%v.tgz", connectorTypeID, connectorType.HelmVersion)
	w, err = archive.Create(helmFileName)
	if err != nil {
		return
	}
	helmFile, err := configFS.Open(filepath.Join(connectorsFolderName, connectorTypeID, helmFolderName, helmFileName))
	if err != nil {
		return
	}
	defer func() {
		if e := helmFile.Close(); e != nil {
			logger.Warn("error closing file", slog.String("error", e.Error()))
		}
	}()
	_, err = io.Copy(w, helmFile)
	if err != nil {
		return
	}
	r = buffer
	return
}

func (c ConnectorTypeLoader) GetConnectorFile(connectorType string, fileID string) (file io.ReadCloser, err error) {
	if _, ok := c.connectorsTypes[connectorType]; !ok {
		err = ErrConnectorTypeNotFound
		return
	}
	file, err = configFS.Open(filepath.Join(connectorsFolderName, connectorType, "files", fileID))
	switch {

	case errors.Is(err, fs.ErrNotExist):
		err = errors.Join(ErrConnectorFileNotFound, err)
		return

	default:
		return
	}
}

func checkHelmFolder(connectorFolder string) (helmVersion string, err error) {
	entries, err := configFS.ReadDir(filepath.Join(connectorFolder, helmFolderName))
	if err != nil {
		return
	}
	valuesOK := false
	chartOK := false
	for _, entry := range entries {
		if entry.Name() == helmValuesFileName {
			valuesOK = true
			continue
		}
		r := regexp.MustCompile(`.*-(\d\.\d\.\d)\.tgz`)
		version := r.FindStringSubmatch(entry.Name())
		if len(version) > 1 {
			helmVersion = version[1]
			chartOK = true
			continue
		}
	}
	if !valuesOK || !chartOK {
		err = errors.New("invalid helm folder")
		return
	}
	return
}

func getConnectorDesc(id string, connectorFolder string, devMode bool) (connectorType ConnectorType, err error) {
	connectorType = ConnectorType{
		SetupSteps:  []Step{},
		LaunchSteps: []Step{},
		Configs:     []ConfigField{},
	}
	entries, err := configFS.ReadDir(connectorFolder)
	if err != nil {
		return
	}
	for _, entry := range entries {
		filename := filepath.Base(entry.Name())
		switch filename {
		case dockerComposeFileName:
			connectorType.DockerCompose = true
			continue
		case helmFolderName:
			helmVersion, helmErr := checkHelmFolder(connectorFolder)
			if helmErr != nil {
				continue
			}
			connectorType.HelmVersion = helmVersion
			connectorType.Helm = true
			continue
		case logoFileName:
		case connectorFileName:
		default:
			continue
		}

		path := filepath.Join(connectorFolder, entry.Name())
		file, openErr := configFS.Open(path)
		if openErr != nil {
			err = openErr
			logger.Error("error loading connector description file", slog.String("file", entry.Name()), slog.String("error", err.Error()))
			return
		}
		defer func() {
			if e := file.Close(); e != nil {
				logger.Warn("error closing file", slog.String("file", entry.Name()), slog.String("error", err.Error()))
				return
			}
		}()
		rawContent, readErr := io.ReadAll(file)
		if readErr != nil {
			err = readErr
			logger.Error("error loading connector description file", slog.String("file", entry.Name()), slog.String("error", err.Error()))
			return
		}

		switch filepath.Base(entry.Name()) {
		case logoFileName:
			connectorType.Logo = base64.StdEncoding.EncodeToString(rawContent)
		case connectorFileName:
			err = yaml.Unmarshal(rawContent, &connectorType)
			if err != nil {
				return
			}
			if connectorType.DevOnly && !devMode {
				err = ErrDevConnector
				return
			}
			for i, s := range connectorType.SetupSteps {
				if s.Files == nil {
					connectorType.SetupSteps[i].Files = make([]string, 0)
				}
			}
			for i, s := range connectorType.LaunchSteps {
				if s.Files == nil {
					connectorType.LaunchSteps[i].Files = make([]string, 0)
				}
			}
			connectorType.ID = id

			defaultConfig, defaultErr := InitDefault(connectorType.ID)
			if defaultErr != nil {
				err = defaultErr
				return
			}

			configFields, fieldsErr := getConfigFields(reflect.ValueOf(defaultConfig).Elem().Interface())
			if fieldsErr != nil {
				err = fieldsErr
				return
			}
			connectorType.Configs = configFields
		}
	}
	return
}

// extracts info from any connector config struct to build configFields.
// e.g. ICAPConfig ; SharepointConfig ; M365Config
func getConfigFields(config any) (configFields []ConfigField, err error) {
	configType := reflect.TypeOf(config)
	configValue := reflect.ValueOf(config)

	for i := range configType.NumField() {
		field := configType.Field(i)
		defaultValue := configValue.Field(i).Interface()

		var fieldType ConfigFieldType
		subFields := []ConfigField{}

		switch field.Type.Kind() {

		case reflect.Bool:
			fieldType = Boolean

		case reflect.String:
			fieldType = String

		case reflect.Int64:
			// underlying type of time.Duration is Int64
			switch field.Type {
			case reflect.TypeFor[time.Duration]():
				fieldType = String
			case reflect.TypeFor[Duration]():
				fieldType = String
			default:
				fieldType = Number
			}

		case reflect.Int:
			fieldType = Number

		case reflect.Slice:
			if field.Type.Elem().Kind() == reflect.String {
				fieldType = StringArray
				break
			}
			err = ErrBadConfigFieldStruct
			return

		case reflect.Struct:
			subConfigFields, subErr := getConfigFields(reflect.ValueOf(config).Field(i).Interface())
			if subErr != nil {
				err = subErr
				return
			}

			// We considere it as a composed struct
			if field.Tag.Get("json") == "" {
				configFields = append(configFields, subConfigFields...)
				continue
			}

			subFields = subConfigFields
			fieldType = Object

		default:
			err = ErrBadConfigFieldStruct
			return
		}

		jsonTags, ok := field.Tag.Lookup("json")
		if !ok {
			continue
		}
		fieldKey := strings.Split(jsonTags, ",")[0]

		fieldDesc, _ := field.Tag.Lookup("desc")

		required := false
		validation := []FrontValidation{}
		if validate, ok := field.Tag.Lookup("validate"); ok {
			rules := strings.SplitSeq(validate, ",")
			for rule := range rules {
				switch rule {
				case "required":
					required = true
				case "url":
					validation = append(validation, FrontValidURL)
				case "email":
					validation = append(validation, FrontValidEmail)

					// to add when available in our custom validator :
					// case "duration":
					// 	validation = append(validation, FrontValidDuration)
					// case "filesize":
					// 	validation = append(validation, FrontValidDuration)
				}
			}
		}

		reconfigurable := true // consider all fields reconfigurable by default
		reconfigurableStr, ok := field.Tag.Lookup(ReconfigurableTag)
		if ok && reconfigurableStr == "false" {
			reconfigurable = false
		}

		configFields = append(configFields, ConfigField{
			Name:           field.Name, // if we decide that Name is no longer field.Name, must update setConfigFieldsValues()
			Key:            fieldKey,
			Type:           fieldType,
			Description:    fieldDesc,
			Required:       required,
			Validation:     validation,
			Reconfigurable: reconfigurable,
			Properties:     subFields,
			DefaultValue:   defaultValue,
		})
	}
	return
}

// InitDefault gives pointer to default config struct for given connector type
func InitDefault(connectorType string) (config any, err error) {
	defaultCommonConfig := CommonConnectorConfig{
		GMalwareUserTags: []string{},
		GMalwareTimeout:  Duration(time.Minute * 5),
	}

	switch connectorType {
	case M365Key:
		config = &M365Config{
			CommonConnectorConfig: defaultCommonConfig,
		}
	case DummyKey:
		config = &DummyConfig{
			CommonConnectorConfig: defaultCommonConfig,
		}
	case ICAPKey:
		config = &ICAPConfig{
			CommonConnectorConfig: defaultCommonConfig,
		}
	case SharepointKey:
		config = &SharepointConfig{
			ReconfigurableSharepointConfig: ReconfigurableSharepointConfig{
				CommonConnectorConfig:             defaultCommonConfig,
				SitesToMonitorWithoutInitialScan:  []string{},
				SitesToMonitorWithInitialScan:     []string{},
				GroupsToMonitorWithoutInitialScan: []string{},
				GroupsToMonitorWithInitialScan:    []string{},
				SitesToIgnore:                     []string{},
				GroupsToIgnore:                    []string{},
				ExcludeDirs:                       []string{},
				ExcludeFiles:                      []string{},
			},
		}
	case HostKey:
		config = &HostConfig{
			CommonConnectorConfig: defaultCommonConfig,
			Actions: HostActionsConfig{
				Delete:     true,
				Quarantine: true,
				Log:        true,
			},
			Quarantine: HostQuarantineConfig{
				Password: "infected",
				Location: "/var/lib/gmhost",
			},
			Monitoring: HostMonitoringConfig{
				ModificationDelay: Duration(time.Second * 30),
			},
			Workers:        4,
			ExtractWorkers: 2,
			MaxFileSize:    "100MiB",
			Paths:          []string{},
		}
	default:
		err = ErrInvalidConnectorType
		return
	}
	return
}

func PatchConfig(connectorType string, rawActualConfig any, rawConfig json.RawMessage) (config any, err error) {
	if rawConfig == nil {
		config = rawActualConfig
		return
	}
	switch connectorType {
	case M365Key:
		actualConfig, ok := rawActualConfig.(*M365Config)
		if !ok {
			err = errors.New("invalid config")
			return
		}
		err = BindAndValidateRaw(actualConfig, rawConfig)
		if err != nil {
			return
		}
		config = actualConfig
	case DummyKey:
		dummyConfig := new(ReconfigurableDummyConfig)
		err = BindRaw(dummyConfig, rawConfig)
		if err != nil {
			return
		}
		actualConfig, ok := rawActualConfig.(*DummyConfig)
		if !ok {
			err = errors.New("invalid config")
			return
		}
		err = BindAndValidateRaw(actualConfig, rawConfig)
		if err != nil {
			return
		}
		config = actualConfig
	case ICAPKey:
		actualConfig, ok := rawActualConfig.(*ICAPConfig)
		if !ok {
			err = errors.New("invalid config")
			return
		}
		err = BindAndValidateRaw(actualConfig, rawConfig)
		if err != nil {
			return
		}
		config = actualConfig
	case SharepointKey:
		sharepointReconfig := new(ReconfigurableSharepointConfig)
		err = BindRaw(sharepointReconfig, rawConfig)
		if err != nil {
			return
		}
		actualConfig, ok := rawActualConfig.(*SharepointConfig)
		if !ok {
			err = errors.New("invalid config")
			return
		}
		err = BindAndValidateRaw(actualConfig, rawConfig)
		if err != nil {
			return
		}
		config = actualConfig
	case HostKey:
		actualConfig, ok := rawActualConfig.(*HostConfig)
		if !ok {
			err = errors.New("invalid config")
			return
		}
		err = BindAndValidateRaw(actualConfig, rawConfig)
		if err != nil {
			return
		}
		config = actualConfig
	default:
		err = errors.New("invalid connector type")
		return
	}
	return
}
