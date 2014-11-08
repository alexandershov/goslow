package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"reflect"
)

type Config struct {
	address             string
	driver              string
	dataSource          string
	minKeyLength        int
	keySalt             string
	createDefaultRules  bool
	singleDomainUrlPath string
}

var DEFAULT_CONFIG *Config = &Config{
	address:             ":5103",
	driver:              "memory",
	dataSource:          "",
	minKeyLength:        6,
	keySalt:             "",
	createDefaultRules:  false,
	singleDomainUrlPath: "/goslow/",
}

func NewConfigFromArgs() *Config {
	config := new(Config)
	defineFlags(config)
	// TODO: think about making configPath a field in the Config struct
	configPath := flag.String("config", "", "path to config file. E.g: /path/to/config.json")
	flag.Parse()
	if *configPath != "" {
		if config.hasNonDefaultValue() {
			log.Fatal("You can't mix -config option with other options")
		}
		config.mergeWithFile(*configPath)
	}
	return config
}

func defineFlags(config *Config) {
	flag.StringVar(&config.address, "address", DEFAULT_CONFIG.address, "address to listen on. E.g: 0.0.0.0:8000")
	flag.StringVar(&config.driver, "driver", DEFAULT_CONFIG.driver, `database driver. One of: memory, sqlite3, mysql, or postgres.
	Default is memory`)
	flag.StringVar(&config.dataSource, "data-source", DEFAULT_CONFIG.dataSource,
		"data source name. E.g: postgres://user:password@localhost/dbname")
	flag.IntVar(&config.minKeyLength, "min-key-length", DEFAULT_CONFIG.minKeyLength,
		"minimum hashids key length. E.g: 8")
	flag.StringVar(&config.keySalt, "key-salt", DEFAULT_CONFIG.keySalt, "hashids key salt. E.g: kj8ioIxZ")
	flag.BoolVar(&config.createDefaultRules, "create-default-rules", DEFAULT_CONFIG.createDefaultRules, "Create default rules?")
	flag.StringVar(&config.singleDomainUrlPath, "single-domain-url-path", DEFAULT_CONFIG.singleDomainUrlPath,
		"Run in single domain mode, use localhost/goslow for configuration")
}

// TODO: options in JSON config should be named the same as cmd options
// i.e min-key-length, not minKeyLength
func (config *Config) mergeWithFile(path string) *Config {
	validateConfig(path)
	unmarshal(path, config)
	return config
}

func unmarshal(path string, i interface{}) {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatalf("Can't read config %s: %v", path, err)
	}
	err = json.Unmarshal(bytes, i)
	if err != nil {
		log.Fatalf("Bad JSON in %s: %v", path, err)
	}
}

func validateConfig(path string) {
	m := make(map[string]interface{})
	unmarshal(path, &m)
	knownFields := getConfigFieldNames()
	key, foundUnknown := findUnknownField(m, knownFields)
	if foundUnknown {
		log.Fatalf("Found unknown key <%s> in config %s. Allowed keys: %s", key, path, knownFields)
	}
}

func getConfigFieldNames() []string {
	return getStructFieldNames(reflect.TypeOf(Config{}))
}

func getStructFieldNames(typ reflect.Type) []string {
	fields := make([]string, 0, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		fields = append(fields, typ.Field(i).Name)
	}
	return fields
}

func findUnknownField(m map[string]interface{}, knownFields []string) (key string, foundUnknown bool) {
	for _, key := range getMapKeys(m) {
		if !contains(knownFields, key) {
			return key, true
		}
	}
	return "", false
}

func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}

func contains(items []string, elem string) bool {
	for _, item := range items {
		if item == elem {
			return true
		}
	}
	return false
}

func (config *Config) hasNonDefaultValue() bool {
	return *config != *DEFAULT_CONFIG
}
