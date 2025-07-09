package api

import (
	"io"
	"net/http"
	"os"
	"time"

	"github.com/AlexxIT/go2rtc/internal/app"
	"gopkg.in/yaml.v3"
)

func configHandler(w http.ResponseWriter, r *http.Request) {
	if app.ConfigPath == "" {
		http.Error(w, "", http.StatusGone)
		return
	}

	switch r.Method {
	case "GET":
		// Acquire lock to prevent concurrent config file access
		app.LockConfig()
		defer app.UnlockConfig()

		start := time.Now()
		app.Logger.Info().Str("method", "GET").Str("endpoint", "/api/config").Msg("config: reading configuration via API")

		data, err := os.ReadFile(app.ConfigPath)
		if err != nil {
			app.Logger.Error().Err(err).Str("file", app.ConfigPath).Msg("config: failed to read config file via API")
			http.Error(w, "", http.StatusNotFound)
			return
		}

		app.Logger.Info().Int("bytes", len(data)).Dur("duration", time.Since(start)).Msg("config: configuration read successfully via API")
		// https://www.ietf.org/archive/id/draft-ietf-httpapi-yaml-mediatypes-00.html
		Response(w, data, "application/yaml")

	case "POST", "PATCH":
		// Acquire lock to prevent concurrent config file access
		app.LockConfig()
		defer app.UnlockConfig()

		start := time.Now()
		app.Logger.Info().Str("method", r.Method).Str("endpoint", "/api/config").Msg("config: updating configuration via API")

		data, err := io.ReadAll(r.Body)
		if err != nil {
			app.Logger.Error().Err(err).Msg("config: failed to read request body")
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		app.Logger.Debug().Int("bytes", len(data)).Msg("config: received request body")

		if r.Method == "PATCH" {
			// no need to validate after merge
			data, err = mergeYAML(app.ConfigPath, data)
			if err != nil {
				app.Logger.Error().Err(err).Msg("config: failed to merge YAML")
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			app.Logger.Debug().Int("bytes", len(data)).Msg("config: YAML merged successfully")
		} else {
			// validate config
			if err = yaml.Unmarshal(data, map[string]any{}); err != nil {
				app.Logger.Error().Err(err).Msg("config: invalid YAML in request")
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			app.Logger.Debug().Msg("config: YAML validation successful")
		}

		if err = os.WriteFile(app.ConfigPath, data, 0644); err != nil {
			app.Logger.Error().Err(err).Str("file", app.ConfigPath).Int("bytes", len(data)).Msg("config: failed to write config file via API")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		app.Logger.Info().Str("method", r.Method).Int("bytes", len(data)).Dur("duration", time.Since(start)).Msg("config: configuration updated successfully via API")
	}
}

func mergeYAML(file1 string, yaml2 []byte) ([]byte, error) {
	// Read the contents of the first YAML file
	app.Logger.Debug().Str("file", file1).Msg("config: reading file for YAML merge")
	data1, err := os.ReadFile(file1)
	if err != nil {
		app.Logger.Error().Err(err).Str("file", file1).Msg("config: failed to read file for YAML merge")
		return nil, err
	}
	app.Logger.Debug().Int("bytes", len(data1)).Msg("config: read file for YAML merge successfully")

	// Unmarshal the first YAML file into a map
	var config1 map[string]any
	if err = yaml.Unmarshal(data1, &config1); err != nil {
		return nil, err
	}

	// Unmarshal the second YAML document into a map
	var config2 map[string]any
	if err = yaml.Unmarshal(yaml2, &config2); err != nil {
		return nil, err
	}

	// Merge the two maps
	config1 = merge(config1, config2)

	// Marshal the merged map into YAML
	return yaml.Marshal(&config1)
}

func merge(dst, src map[string]any) map[string]any {
	for k, v := range src {
		if vv, ok := dst[k]; ok {
			switch vv := vv.(type) {
			case map[string]any:
				v := v.(map[string]any)
				dst[k] = merge(vv, v)
			case []any:
				v := v.([]any)
				dst[k] = v
			default:
				dst[k] = v
			}
		} else {
			dst[k] = v
		}
	}
	return dst
}
