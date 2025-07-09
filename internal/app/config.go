package app

import (
	"errors"
	"github.com/AlexxIT/go2rtc/pkg/shell"
	"github.com/AlexxIT/go2rtc/pkg/yaml"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func LoadConfig(v any) {
	// Note: LoadConfig is called during initialization and reads from in-memory configs,
	// not directly from files, so no file lock is needed here
	for _, data := range configs {
		if err := yaml.Unmarshal(data, v); err != nil {
			Logger.Warn().Err(err).Send()
		}
	}
}

// configMu protects all configuration file operations
var configMu sync.Mutex

// LockConfig acquires the configuration file lock
func LockConfig() {
	configMu.Lock()
}

// UnlockConfig releases the configuration file lock
func UnlockConfig() {
	configMu.Unlock()
}

func PatchConfig(path []string, value any) error {
	if ConfigPath == "" {
		return errors.New("config file disabled")
	}

	// Acquire lock to prevent concurrent config file access
	configMu.Lock()
	defer configMu.Unlock()

	start := time.Now()
	Logger.Info().Strs("path", path).Interface("value", value).Msg("config: patching configuration")

	// empty config is OK
	b, readErr := os.ReadFile(ConfigPath)
	if readErr != nil {
		Logger.Warn().Err(readErr).Str("file", ConfigPath).Msg("config: failed to read config file, using empty config")
		b = []byte{}
	} else {
		Logger.Debug().Int("bytes", len(b)).Str("file", ConfigPath).Msg("config: read config file")
	}

	b, err := yaml.Patch(b, path, value)
	if err != nil {
		Logger.Error().Err(err).Strs("path", path).Interface("value", value).Msg("config: failed to patch YAML")
		return err
	}

	Logger.Debug().Int("bytes", len(b)).Msg("config: patched YAML successfully")

	if err := os.WriteFile(ConfigPath, b, 0644); err != nil {
		Logger.Error().Err(err).Str("file", ConfigPath).Int("bytes", len(b)).Msg("config: failed to write config file")
		return err
	}

	Logger.Info().Strs("path", path).Interface("value", value).Dur("duration", time.Since(start)).Msg("config: configuration patched successfully")
	return nil
}

type flagConfig []string

func (c *flagConfig) String() string {
	return strings.Join(*c, " ")
}

func (c *flagConfig) Set(value string) error {
	*c = append(*c, value)
	return nil
}

var configs [][]byte

func initConfig(confs flagConfig) {
	if confs == nil {
		confs = []string{"go2rtc.yaml"}
	}

	for _, conf := range confs {
		if len(conf) == 0 {
			continue
		}
		if conf[0] == '{' {
			// config as raw YAML or JSON
			configs = append(configs, []byte(conf))
		} else if data := parseConfString(conf); data != nil {
			configs = append(configs, data)
		} else {
			// config as file
			if ConfigPath == "" {
				ConfigPath = conf
			}

			Logger.Debug().Str("file", conf).Msg("config: reading config file during initialization")
			if data, err := os.ReadFile(conf); err != nil {
				Logger.Warn().Err(err).Str("file", conf).Msg("config: failed to read config file during initialization")
				continue
			} else if data == nil {
				continue
			} else {
				Logger.Debug().Int("bytes", len(data)).Str("file", conf).Msg("config: read config file successfully during initialization")
			}

			data = []byte(shell.ReplaceEnvVars(string(data)))
			configs = append(configs, data)
		}
	}

	if ConfigPath != "" {
		if !filepath.IsAbs(ConfigPath) {
			if cwd, err := os.Getwd(); err == nil {
				ConfigPath = filepath.Join(cwd, ConfigPath)
			}
		}
		Info["config_path"] = ConfigPath
	}
}

func parseConfString(s string) []byte {
	i := strings.IndexByte(s, '=')
	if i < 0 {
		return nil
	}

	items := strings.Split(s[:i], ".")
	if len(items) < 2 {
		return nil
	}

	// `log.level=trace` => `{log: {level: trace}}`
	var pre string
	var suf = s[i+1:]
	for _, item := range items {
		pre += "{" + item + ": "
		suf += "}"
	}

	return []byte(pre + suf)
}
