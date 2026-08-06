package config

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load reads, expands, strictly decodes, defaults, and validates one bootstrap
// YAML document. It returns no partially valid configuration on failure.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, &ConfigError{
			Kind:    ErrorKindRead,
			Path:    path,
			Problem: "read file: " + err.Error(),
			Err:     err,
		}
	}

	expanded, err := expandEnvironment(string(data), os.LookupEnv)
	if err != nil {
		return nil, err
	}

	cfg, err := decodeStrict(expanded)
	if err != nil {
		return nil, err
	}
	if err := Validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func expandEnvironment(data string, lookup func(string) (string, bool)) (string, error) {
	missingSet := make(map[string]struct{})
	expanded := os.Expand(data, func(name string) string {
		value, ok := lookup(name)
		if !ok {
			missingSet[name] = struct{}{}
		}
		return value
	})
	if len(missingSet) == 0 {
		return expanded, nil
	}

	missing := make([]string, 0, len(missingSet))
	for name := range missingSet {
		missing = append(missing, name)
	}
	sort.Strings(missing)
	return "", &ConfigError{
		Kind:    ErrorKindEnvironment,
		Path:    "environment",
		Problem: "missing variables: " + strings.Join(missing, ", "),
	}
}

func decodeStrict(data string) (*Config, error) {
	decoder := yaml.NewDecoder(strings.NewReader(data))
	decoder.KnownFields(true)

	cfg := configWithDefaults()
	if err := decoder.Decode(&cfg); err != nil {
		return nil, &ConfigError{
			Kind:    ErrorKindParse,
			Path:    "document",
			Problem: err.Error(),
			Err:     err,
		}
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("multiple YAML documents are not allowed")
		}
		return nil, &ConfigError{
			Kind:    ErrorKindParse,
			Path:    "document",
			Problem: err.Error(),
			Err:     err,
		}
	}

	timeoutPresent, err := providerTimeoutPresence(data)
	if err != nil {
		return nil, &ConfigError{
			Kind:    ErrorKindParse,
			Path:    "document",
			Problem: err.Error(),
			Err:     err,
		}
	}
	applyProviderTimeoutDefaults(&cfg, timeoutPresent)
	return &cfg, nil
}

func providerTimeoutPresence(data string) ([]bool, error) {
	var document yaml.Node
	if err := yaml.Unmarshal([]byte(data), &document); err != nil {
		return nil, err
	}
	if len(document.Content) == 0 {
		return nil, nil
	}
	providers := mappingValue(document.Content[0], "providers")
	if providers == nil || providers.Kind != yaml.SequenceNode {
		return nil, nil
	}
	present := make([]bool, len(providers.Content))
	for i, provider := range providers.Content {
		present[i] = mappingValue(provider, "timeout") != nil
	}
	return present, nil
}

func mappingValue(mapping *yaml.Node, key string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}
