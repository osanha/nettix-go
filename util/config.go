package util

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// PropertiesConfig provides configuration based on a properties file.
type PropertiesConfig struct {
	properties map[string]string
}

// NewPropertiesConfig creates a new PropertiesConfig from a properties file.
func NewPropertiesConfig(path string) (*PropertiesConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	config := &PropertiesConfig{
		properties: make(map[string]string),
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if len(line) == 0 || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}

		// Find the separator
		sepIdx := strings.IndexAny(line, "=:")
		if sepIdx < 0 {
			continue
		}

		key := strings.TrimSpace(line[:sepIdx])
		value := strings.TrimSpace(line[sepIdx+1:])
		config.properties[key] = value
	}

	return config, scanner.Err()
}

// Contains checks if the configuration contains the given key.
func (c *PropertiesConfig) Contains(key string) bool {
	_, ok := c.properties[key]
	return ok
}

// GetString returns the value associated with the key as a string.
func (c *PropertiesConfig) GetString(key string) (string, error) {
	if val, ok := c.properties[key]; ok {
		return val, nil
	}
	return "", fmt.Errorf("key not found: %s", key)
}

// GetStringDefault returns the value or a default if not found.
func (c *PropertiesConfig) GetStringDefault(key, defaultValue string) string {
	if val, ok := c.properties[key]; ok {
		return val
	}
	return defaultValue
}

// GetStringOrNil returns the value or nil if not found.
func (c *PropertiesConfig) GetStringOrNil(key string) *string {
	if val, ok := c.properties[key]; ok {
		return &val
	}
	return nil
}

// GetInt returns the value as an int.
func (c *PropertiesConfig) GetInt(key string) (int, error) {
	str, err := c.GetString(key)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(str)
}

// GetIntOrNil returns the value as an int pointer or nil.
func (c *PropertiesConfig) GetIntOrNil(key string) *int {
	if val, ok := c.properties[key]; ok {
		if i, err := strconv.Atoi(val); err == nil {
			return &i
		}
	}
	return nil
}

// GetLong returns the value as an int64.
func (c *PropertiesConfig) GetLong(key string) (int64, error) {
	str, err := c.GetString(key)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(str, 10, 64)
}

// GetBool returns the value as a bool.
func (c *PropertiesConfig) GetBool(key string) (bool, error) {
	str, err := c.GetString(key)
	if err != nil {
		return false, err
	}
	return strconv.ParseBool(str)
}

// GetDouble returns the value as a float64.
func (c *PropertiesConfig) GetDouble(key string) (float64, error) {
	str, err := c.GetString(key)
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(str, 64)
}

// GetStringArray returns the value as a string array (CSV format).
func (c *PropertiesConfig) GetStringArray(key string) ([]string, error) {
	str, err := c.GetString(key)
	if err != nil {
		return nil, err
	}

	parts := strings.Split(str, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		result = append(result, strings.TrimSpace(p))
	}
	return result, nil
}
