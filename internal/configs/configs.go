package configs

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"

	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/util"
	"gopkg.in/yaml.v2"
)

const (
	PVPEnabled  = `enabled`
	PVPDisabled = `disabled`
	PVPOff      = `off`
	PVPLimited  = `limited`
)

var (
	configData Config         = newUnloadedConfig()
	overrides  map[string]any = make(map[string]any)

	keyLookups  map[string]string = map[string]string{}
	typeLookups map[string]string = map[string]string{}

	// moduleOverlayKeys tracks which keys in the overrides union were written
	// by AddOverlayOverrides (module data-overlay defaults) rather than by the
	// operator (config-overrides.yaml load or SetVal). Module defaults may
	// freely overwrite their own previous registrations, but never an
	// operator-supplied value. SetVal unmarks a key (operator now owns it) and
	// SetOverrides resets the ledger (the union was replaced from file).
	moduleOverlayKeys = map[string]struct{}{}

	configDataLock       sync.RWMutex
	ErrInvalidConfigName = errors.New("invalid config name")
	ErrLockedConfig      = errors.New("config name is locked")
)

type Config struct {
	// Start config subsections
	Server       Server       `yaml:"Server"`
	Memory       Memory       `yaml:"Memory"`
	LootGoblin   LootGoblin   `yaml:"LootGoblin"`
	Timing       Timing       `yaml:"Timing"`
	FilePaths    FilePaths    `yaml:"FilePaths"`
	GamePlay     GamePlay     `yaml:"GamePlay"`
	Integrations Integrations `yaml:"Integrations"`
	TextFormats  TextFormats  `yaml:"TextFormats"`
	Translation  Translation  `yaml:"Translation"`
	Network      Network      `yaml:"Network"`
	SpecialRooms SpecialRooms `yaml:"SpecialRooms"`
	Validation   Validation   `yaml:"Validation"`
	Roles        Roles        `yaml:"Roles"`
	// Plugins is a special case
	Modules Modules `yaml:"Modules"`

	Balance Balance `yaml:"Balance"`

	LLM LLM `yaml:"LLM"`

	// APIFramework is the server's model key, its one daily token budget and
	// its breaker, shared by the AI companion and bauble naming.
	APIFramework APIFramework `yaml:"APIFramework"`

	Analytics Analytics `yaml:"Analytics"`

	Logging Logging `yaml:"Logging"`

	Playtest Playtest `yaml:"Playtest"`

	// End config subsections

	seedInt int64 `yaml:"-"`

	validated bool
}

func AddOverlayOverrides(dotMap map[string]any) error {
	// Write lock: this mutates configData (via OverlayOverrides) and the
	// keyLookups/typeLookups/overrides maps. It previously held only RLock,
	// racing with concurrent getters/readers. No config getter calls this, so
	// upgrading to the write lock cannot deadlock.
	configDataLock.Lock()
	defer configDataLock.Unlock()

	// Register type/key lookups for every key in the overlay, but only
	// write values for keys that have not already been set by a user override
	// (e.g. config-overrides.yaml). This ensures module defaults never clobber
	// operator-supplied values.
	//
	// DOGMud divergence from upstream GoMud (which skips ANY already-set key):
	// keys whose current value came from a previous AddOverlayOverrides call
	// (tracked in moduleOverlayKeys) may still be overwritten here, so modules
	// and tests can re-register/update their own defaults. Only operator
	// values are protected.
	newKeys := map[string]any{}
	flatOverrides := Flatten(overrides)
	for k, v := range dotMap {

		if strings.Index(k, `.`) != -1 {

			parts := strings.Split(k, `.`)

			for i := len(parts) - 1; i >= 0; i-- {
				tmpKey := strings.Join(parts[i:], `.`)
				keyLookups[strings.ToLower(tmpKey)] = k

				tmpKey = strings.Join(parts[i:], ``)
				keyLookups[strings.ToLower(tmpKey)] = k

			}

		} else {
			keyLookups[strings.ToLower(k)] = k
		}

		typeLookups[k] = reflect.TypeOf(v).String()

		_, alreadySet := flatOverrides[k]
		_, moduleOwned := moduleOverlayKeys[k]
		if !alreadySet || moduleOwned {
			overrides[k] = v
			newKeys[k] = v
			moduleOverlayKeys[k] = struct{}{}
		}
	}

	if len(newKeys) == 0 {
		return nil
	}

	// Overlay the full override set rather than just the new keys. Overlaying
	// only the new keys would unmarshal a partial Modules block into the live
	// config, replacing the inner Modules[<name>] map wholesale and discarding
	// every operator-supplied value for that module. Flatten first: at this
	// point `overrides` is mixed-shape (nested maps loaded from
	// config-overrides.yaml plus the flat dotted keys added above), and
	// OverlayOverrides re-nests it internally.
	return configData.OverlayOverrides(Flatten(overrides))
}

// OverlayDotMap overlays values from a dot-syntax map onto the Config.
func (c *Config) OverlayOverrides(dotMap map[string]any) error {
	// First unflatten the dot map into a nested map.
	nestedMap := unflattenMap(dotMap)

	// Marshal the nested map into YAML bytes.
	b, err := yaml.Marshal(nestedMap)
	if err != nil {
		return err
	}

	// Unmarshal the YAML bytes into the existing Config struct.
	return yaml.Unmarshal(b, c)
}

func (c *Config) DotPaths() map[string]any {
	result := make(map[string]any)
	// Get the underlying value of c (we assume c is a pointer).
	v := reflect.ValueOf(c).Elem()
	c.buildDotPaths(v, "", result)
	return result
}

func (c *Config) buildDotPaths(v reflect.Value, prefix string, result map[string]any) {
	// If the value is a pointer, dereference it.
	if v.Kind() == reflect.Interface || v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return
		}
		c.buildDotPaths(v.Elem(), prefix, result)
		return
	}

	switch v.Kind() {
	case reflect.Struct:
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			field := t.Field(i)
			// Skip unexported fields.
			if field.PkgPath != "" {
				continue
			}
			fieldVal := v.Field(i)
			// Determine the key name using the YAML tag if available.
			key := field.Name
			if yamlTag, ok := field.Tag.Lookup("yaml"); ok {
				if tagName := strings.Split(yamlTag, ",")[0]; tagName != "" {
					key = tagName
				}
			}
			// Construct the new prefix.
			newPrefix := key
			if prefix != "" {
				newPrefix = prefix + "." + key
			}
			// Recursively build paths.
			c.buildDotPaths(fieldVal, newPrefix, result)
		}
	case reflect.Map:
		// If the map is nil, store nil for the current prefix.
		if v.IsNil() {
			result[prefix] = make(map[string]any)
			return
		}
		// Iterate over each key in the map.
		for _, key := range v.MapKeys() {
			// Convert the key to string (works for string keys; for others, fmt.Sprintf is used).
			keyStr := fmt.Sprintf("%v", key.Interface())
			newPrefix := keyStr
			if prefix != "" {
				newPrefix = prefix + "." + keyStr
			}
			c.buildDotPaths(v.MapIndex(key), newPrefix, result)
		}
	default:
		// For non-struct fields, store the value using the accumulated prefix.
		result[prefix] = v.Interface()
	}
}

func GetOverrides() map[string]any {
	return overrides
}

func (c *Config) SetOverrides(newOverrides map[string]any) error {

	overrides = newOverrides
	// The union was replaced wholesale (operator file load); any module
	// ownership marks are stale and must not let a later module overlay
	// clobber operator values.
	moduleOverlayKeys = map[string]struct{}{}
	c.OverlayOverrides(overrides)

	return nil
}

// Ensures certain ranges and defaults are observed
func (c *Config) Validate() {

	c.Server.Validate()
	c.Memory.Validate()
	c.LootGoblin.Validate()
	c.Timing.Validate()
	c.FilePaths.Validate()
	c.GamePlay.Validate()
	c.Integrations.Validate()
	c.TextFormats.Validate()
	c.Translation.Validate()
	c.Network.Validate()
	c.SpecialRooms.Validate()
	c.Validation.Validate()
	c.Modules.Validate()
	c.Roles.Validate()
	c.Balance.Validate()
	c.LLM.Validate()
	c.APIFramework.Validate()
	c.Analytics.Validate()
	c.Playtest.Validate()

	// nothing to do with LootGoblinIncludeRecentRooms

	// Nothing to do with Locked

	c.seedInt = 0
	for i, num := range util.Md5Bytes([]byte(string(c.Server.Seed))) {
		c.seedInt += int64(num) << i
	}

	c.validated = true
}

// ensureConfigValidated runs configData.Validate() exactly once before the
// first config read, under the WRITE lock. Validate() mutates configData
// (clamps fields, computes seedInt, sets validated=true). The getters used to
// run it while holding only configDataLock.RLock(), so two goroutines hitting
// any getter concurrently on first use both observed validated==false and both
// mutated configData — a data race the -race detector flags (and a real
// startup race, since many goroutines read config before it is validated).
// Double-checked locking keeps the hot path (already validated) on the cheap
// read lock. Safe from deadlock: Validate() and the subsection validators only
// touch their own fields and never call a config getter.
func ensureConfigValidated() {
	configDataLock.RLock()
	done := configData.validated
	configDataLock.RUnlock()
	if done {
		return
	}

	configDataLock.Lock()
	defer configDataLock.Unlock()
	if !configData.validated {
		configData.Validate()
	}
}

func (c *Config) setEnvAssignments(clear bool) {

	// We use reflect.Indirect to handle if cfg is a pointer or not
	v := reflect.ValueOf(c).Elem()

	// We'll need the struct type as well (to get field names).
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		fieldVal := v.Field(i)
		fieldType := t.Field(i)

		if fieldVal.Type().Kind() != reflect.String {
			continue
		}

		if envName := fieldType.Tag.Get(`env`); envName != `` {
			if fieldVal.CanSet() {
				if envVal := os.Getenv(envName); envVal != `` {

					if clear {
						envVal = ``
					}

					fieldVal.Set(reflect.ValueOf(ConfigSecret(envVal)))

				}
			}
		}

	}
}

func (c Config) IsBannedName(name string) (string, bool) {

	name = strings.ToLower(strings.TrimSpace(name))

	for _, bannedName := range c.Validation.BannedNames {
		if util.StringWildcardMatch(name, strings.ToLower(bannedName)) {
			return bannedName, true
		}
	}

	return "", false
}

func (c Config) SeedInt() int64 {
	return c.seedInt
}

func (c Config) AllConfigData(excludeStrings ...string) map[string]any {

	finalOutput := make(map[string]any)

	for name, value := range c.DotPaths() {

		if len(excludeStrings) > 0 {
			testName := strings.ToLower(name)
			skip := false
			for _, s := range excludeStrings {
				if util.StringWildcardMatch(testName, s) {
					skip = true
					break
				}
			}
			if skip {
				continue
			}
		}

		finalOutput[name] = value
	}
	return finalOutput
}

// SetVal is the operator's write: `server set`, the `server config` menu,
// `setmotd` and module config setters (plugins.PluginConfig.Set). It refuses
// any key IsLocked names with ErrLockedConfig, checked on the RESOLVED path so
// a bare suffix key ("seed") cannot slip past a full-path lock.
func SetVal(propertyPath string, newVal string) error {
	return setVal(propertyPath, newVal, true)
}

// SetEngineVal is the engine's own write, for values the server maintains
// itself that Server.Locked keeps from operators: Server.CurrentVersion
// (internal/migration) and Server.NextRoomId (internal/rooms). It skips the
// Server.Locked list but still refuses the hard list, because no engine write
// needs a security key.
func SetEngineVal(propertyPath string, newVal string) error {
	return setVal(propertyPath, newVal, false)
}

func setVal(propertyPath string, newVal string, operator bool) error {

	propertyPath, propertyType := FindFullPath(propertyPath)

	// Locks first, so a hard-locked key is refused as locked even where the
	// lookups cannot resolve it.
	if isHardLocked(propertyPath) || (operator && IsLocked(propertyPath)) {
		return fmt.Errorf(`%w: %s`, ErrLockedConfig, propertyPath)
	}

	if newVal == RedactedValue {
		return fmt.Errorf(`%w: %s`, ErrRedactedValue, propertyPath)
	}

	if propertyType == `` {
		return errors.New(`invalid property name: ` + propertyPath)
	}

	quickMap := make(map[string]any)
	quickMap[propertyPath] = StringToConfigValue(newVal, propertyType)

	flatOverrides := Flatten(overrides)
	flatQuickmap := Flatten(quickMap)

	for k, v := range flatQuickmap {
		flatOverrides[k] = v
		// The operator now owns this key; a later module overlay
		// (AddOverlayOverrides) must not overwrite it.
		delete(moduleOverlayKeys, k)
	}

	overrides = unflattenMap(flatOverrides)

	// save the new config.
	writeBytes, err := yaml.Marshal(overrides)
	if err != nil {
		return err
	}

	overridePath := overridePathFor(GetConfig().FilePaths.DataFiles.String())
	saveErr := util.Save(overridePath, writeBytes, bool(configData.FilePaths.CarefulSaveFiles))

	// The overrides map was already mutated above, so memory follows it even
	// when the durable save failed (prod mounts /mud-config read-only).
	// Returning before this overlay let overrides and the live config diverge,
	// which froze Server.NextRoomId in memory while the map kept advancing and
	// made the room allocator hand out the same id repeatedly. The save error
	// is still returned so callers log it or report it.
	if err = configData.OverlayOverrides(overrides); err != nil {
		if saveErr != nil {
			return errors.Join(saveErr, err)
		}
		return err
	}

	configData.Validate()
	return saveErr
}

func GetConfig() Config {
	ensureConfigValidated()

	configDataLock.RLock()
	defer configDataLock.RUnlock()

	return configData
}

// OverridePathFor is overridePathFor for callers outside this package.
func OverridePathFor(dataFiles string) string {
	return overridePathFor(dataFiles)
}

// overridePathFor returns where config-overrides.yaml lives for a world whose
// DataFiles is dataFiles. CONFIG_PATH, when set, wins. The caller passes the
// DataFiles that is (or is about to be) in effect: ReloadConfig must pass the
// one it just loaded, because on the first boot call configData is still the
// unloaded default and would name _datafiles/world/default.
func overridePathFor(dataFiles string) string {
	overridePath := os.Getenv(`CONFIG_PATH`)
	if overridePath == `` {
		overridePath = dataFiles + `/config-overrides.yaml`
	}

	return overridePath
}

// newUnloadedConfig returns the Config a load unmarshals INTO, with the
// "unset" sentinels pre-seeded.
//
// Most knobs can tell "absent" from "explicitly set" because their zero value
// is illegal, so a `<= 0` guard in Validate() can safely restore the default.
// ProgressionFailureFraction cannot: 0 is a legal explicit off-switch ("a lost
// action teaches nothing"), and an absent YAML key also unmarshals to 0, so no
// predicate applied AFTER the unmarshal can separate the two cases. Seeding a
// negative sentinel BEFORE the unmarshal makes "still negative" mean "absent"
// and nothing else.
//
// This works because yaml.v2 decodes into an addressable struct in place and
// leaves fields whose keys are absent from the document untouched. loadConfig
// is the only path that decodes _datafiles/config.yaml, and the overlay and
// env-assignment passes that follow only write keys they were given, so nothing
// between here and Validate() can clobber the sentinel.
//
// It also seeds the package-level configData, so a GetConfig() read that lands
// before ReloadConfig() completes resolves to the default via
// ensureConfigValidated rather than reading a bare 0. Every neighbouring
// `<= 0`-idiom knob already gets its default in that window; without this the
// sentinel knobs would be the sole asymmetry.
func newUnloadedConfig() Config {
	c := Config{}
	c.Balance.ProgressionFailureFraction = -1
	c.Balance.ProneAutoStandRounds = -1
	return c
}

// loadConfig decodes a config document the way ReloadConfig does: into the
// sentinel-seeded struct, so an absent key stays distinguishable from an
// explicit zero. Tests exercise the real load path through here, which is the
// point: asserting on a hand-seeded struct would leave the WIRING untested, and
// asserting on the shipped config.yaml (which now names the key) would be a
// gate that cannot fail.
func loadConfig(document []byte) (Config, error) {
	c := newUnloadedConfig()
	if err := yaml.Unmarshal(document, &c); err != nil {
		return Config{}, err
	}
	return c, nil
}

func ReloadConfig() error {

	configPath := util.FilePath(`_datafiles/config.yaml`)

	bytes, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	tmpConfigData, err := loadConfig(bytes)
	if err != nil {
		return err
	}

	// Build a special lookup to attempt to match old data or even some minor typos
	keyLookups, typeLookups = buildKeyLookups(configData)

	// Resolve DataFiles the way FilePaths.Validate will, so an absent key
	// still maps to the default world.
	loadedPaths := tmpConfigData.FilePaths
	loadedPaths.Validate()
	overridePath := overridePathFor(loadedPaths.DataFiles.String())

	mudlog.Info("ReloadConfig()", "overridePath", overridePath)

	if _, err := os.Stat(util.FilePath(overridePath)); err == nil {
		if overridePath != `` {

			mudlog.Info("ReloadConfig()", "Loading overrides", true)

			overrideBytes, err := os.ReadFile(util.FilePath(overridePath))
			if err != nil {
				return err
			}

			tmpOverrides := map[string]any{}
			err = yaml.Unmarshal(overrideBytes, &tmpOverrides)
			if err != nil {
				return err
			}

			// Attempt a correction for bad names
			for k, v := range tmpOverrides {
				if newKey, _ := FindFullPath(k); newKey != k {
					tmpOverrides[newKey] = v
					delete(tmpOverrides, k)
				}
			}

			tmpConfigData.SetOverrides(tmpOverrides)
		}
	} else {
		mudlog.Info("ReloadConfig()", "Loading overrides", false)
	}

	tmpConfigData.setEnvAssignments(false)

	tmpConfigData.Validate()

	configDataLock.Lock()
	defer configDataLock.Unlock()
	// Assign it
	configData = tmpConfigData

	return nil
}

func FindFullPath(inputKey string) (properKey string, typeName string) {

	if v, ok := keyLookups[strings.ToLower(inputKey)]; ok {
		return v, typeLookups[v]
	}
	return inputKey, typeLookups[inputKey]
}

// buildKeyLookups builds the tables FindFullPath reads from c's dot paths.
// Every key is reachable by its full path and by each dotted and undotted
// suffix, lowercased, which is how `server set seed 1` finds Server.Seed. A
// suffix shared by two keys resolves to one of them; which one depends on map
// iteration order, so it is not stable across runs.
func buildKeyLookups(c Config) (keys map[string]string, types map[string]string) {
	keys = map[string]string{}
	types = map[string]string{}
	for k, v := range c.AllConfigData() {
		if strings.Contains(k, `.`) {
			parts := strings.Split(k, `.`)
			for i := len(parts) - 1; i >= 0; i-- {
				keys[strings.ToLower(strings.Join(parts[i:], `.`))] = k
				keys[strings.ToLower(strings.Join(parts[i:], ``))] = k
			}
		} else {
			keys[strings.ToLower(k)] = k
		}
		types[k] = reflect.TypeOf(v).String()
	}
	return keys, types
}

// Usage: configs.GetSecret(c.DiscordWebhookUrl)
func GetSecret(v ConfigSecret) string {
	return string(v)
}

// flatten recursively flattens a map[string]any.
// It supports both map[string]any and map[any]any values,
// which is useful when unmarshaling YAML.
func Flatten(input map[string]any) map[string]any {
	flatMap := make(map[string]any)
	flattenHelper("", input, flatMap)
	return flatMap
}

// flattenHelper is a recursive helper that constructs the flattened map.
func flattenHelper(prefix string, input map[string]any, flatMap map[string]any) {
	for key, value := range input {
		// Construct the new key path.
		var newKey string
		if prefix == "" {
			newKey = key
		} else {
			newKey = prefix + "." + key
		}

		// Handle nested maps from YAML unmarshaling, which can be of type map[any]any.
		switch v := value.(type) {
		case map[string]any:
			flattenHelper(newKey, v, flatMap)
		case map[any]any:
			// Convert map[any]any to map[string]any.
			converted := make(map[string]any)
			for k, val := range v {
				if strKey, ok := k.(string); ok {
					converted[strKey] = val
				}
			}
			flattenHelper(newKey, converted, flatMap)
		default:
			flatMap[newKey] = value
		}
	}
}
