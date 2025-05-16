package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// SiteConfig represents configuration for an upload service
type SiteConfig struct {
	Name         string            `json:"name"`
	RequestURL   string            `json:"requestURL"`
	RequestType  string            `json:"requestType"`
	FileFormName string            `json:"fileFormName,omitempty"`
	ResponseType string            `json:"responseType"`
	Regexps      map[string]string `json:"regexps"`
	Headers      map[string]string `json:"headers,omitempty"`
	Arguments    map[string]string `json:"arguments,omitempty"`
}

// Config represents the application configuration
type Config struct {
	DefaultFileUpload   string                `json:"defaultFileUpload"`
	DefaultImageUpload  string                `json:"defaultImageUpload"`
	DefaultURLShortener string                `json:"defaultUrlShortener,omitempty"`
	HistoryPath         string                `json:"historyPath"`
	SaveDir             string                `json:"saveDir"`
	Organized           bool                  `json:"organized"`
	Uploaders           map[string]SiteConfig `json:"uploaders"`
	Shorteners          map[string]SiteConfig `json:"shorteners"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() Config {
	return Config{
		DefaultFileUpload:  "imgur",
		DefaultImageUpload: "imgur",
		HistoryPath:        "$HOME/Pictures/Screenshots/caplet",
		SaveDir:            "$HOME/Pictures/Screenshots/caplet",
		Organized:          true,
		Shorteners:         map[string]SiteConfig{},
		Uploaders: map[string]SiteConfig{
			"imgur": {
				Name:         "Imgur",
				RequestURL:   "https://api.imgur.com/3/image",
				FileFormName: "image",
				ResponseType: "json",
				RequestType:  "POST",
				Regexps: map[string]string{
					"url": "\"link\":\"(.+?)\"",
				},
				Headers: map[string]string{
					"Authorization": "Client-ID b972ecca954f246",
				},
			},
		},
	}
}

// ExtractJSONKeys extracts JSON keys from fields
func ExtractJSONKeys(fields ...string) map[string]string {
	regexps := make(map[string]string)

	for _, field := range fields {
		if field == "" {
			continue
		}

		re := regexp.MustCompile(`\$json:([a-zA-Z0-9_]+)\$`)
		matches := re.FindAllString(field, -1)

		for _, match := range matches {
			key := strings.ReplaceAll(strings.ReplaceAll(match, "$json:", ""), "$", "")
			regexps["url"] = "\"" + key + "\":\"(.+?)\""
		}
	}

	return regexps
}

// ImportSXCU imports ShareX custom uploader configs
func ImportSXCU(path string) error {
	// Read the SXCU file
	sxcuData, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("error reading SXCU file: %w", err)
	}

	// Parse the SXCU JSON
	var sxcu map[string]interface{}
	if err := json.Unmarshal(sxcuData, &sxcu); err != nil {
		return fmt.Errorf("error parsing SXCU file: %w", err)
	}

	isURLShortener := false
	if strings.Contains(sxcu["DestinationType"].(string), "URLShortener") {
		isURLShortener = true
	}

	// Extract URL field if it exists
	urlField := ""
	if url, ok := sxcu["URL"].(string); ok {
		urlField = url
	}

	regexps := ExtractJSONKeys(urlField)

	// Create a SiteConfig from the SXCU data
	siteConfig := SiteConfig{
		Name:         sxcu["Name"].(string),
		RequestURL:   sxcu["RequestURL"].(string),
		ResponseType: "regex",
		Regexps:      regexps,
	}

	// Set RequestType if it exists, otherwise default to POST
	if requestMethod, ok := sxcu["RequestMethod"].(string); ok {
		siteConfig.RequestType = requestMethod
	} else {
		siteConfig.RequestType = "POST"
	}

	// Set FileFormName if it exists
	if fileFormName, ok := sxcu["FileFormName"].(string); ok {
		siteConfig.FileFormName = fileFormName
	}

	// Set Headers if they exist
	if headers, ok := sxcu["Headers"].(map[string]interface{}); ok {
		siteConfig.Headers = make(map[string]string)
		for key, val := range headers {
			if strVal, ok := val.(string); ok {
				siteConfig.Headers[key] = strVal
			}
		}
	}

	// Set Arguments if they exist
	if args, ok := sxcu["Arguments"].(map[string]interface{}); ok {
		siteConfig.Arguments = make(map[string]string)
		for key, val := range args {
			if strVal, ok := val.(string); ok {
				siteConfig.Arguments[key] = strVal
			}
		}
	}

	// Load current config
	config, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("error loading current config: %w", err)
	}

	// Add the new site config
	if isURLShortener {
		config.DefaultURLShortener = siteConfig.Name
		config.Shorteners[siteConfig.Name] = siteConfig
		fmt.Printf("Added URL shortener: \"%s\"\n", siteConfig.Name)
	} else {
		config.DefaultFileUpload = siteConfig.Name
		config.DefaultImageUpload = siteConfig.Name
		config.Uploaders[siteConfig.Name] = siteConfig
		fmt.Printf("Added uploader: \"%s\"\n", siteConfig.Name)
	}

	// Write the updated config
	configPath := filepath.Join(os.Getenv("HOME"), ".config", "caplet", "config.json")
	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling config: %w", err)
	}

	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		return fmt.Errorf("error writing config file: %w", err)
	}

	return nil
}

// LoadConfig loads the configuration from file or returns default
func LoadConfig() (Config, error) {
	configPath := filepath.Join(os.Getenv("HOME"), ".config", "caplet", "config.json")

	configData, err := os.ReadFile(configPath)

	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No config found, using default configuration")

			// Create the config directory
			configDir := filepath.Dir(configPath)
			if err := os.MkdirAll(configDir, 0755); err != nil {
				return Config{}, fmt.Errorf("error creating config directory: %w", err)
			}

			// Write default config
			defaultConfig := DefaultConfig()
			configData, err := json.MarshalIndent(defaultConfig, "", "  ")
			if err != nil {
				return Config{}, fmt.Errorf("error marshaling default config: %w", err)
			}

			if err := os.WriteFile(configPath, configData, 0644); err != nil {
				return Config{}, fmt.Errorf("error writing default config: %w", err)
			}

			return defaultConfig, nil
		}

		return Config{}, fmt.Errorf("error reading config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(configData, &config); err != nil {
		return Config{}, fmt.Errorf("error parsing config: %w", err)
	}

	return config, nil
}
