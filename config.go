package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"reflect"
)


type Config struct {
	Host            string
	Address         string
	Driver              string
	DataSource          string
	MinKeyLength    int
	KeySalt         string
	AddDefaultRules bool
}

var DEFAULT_CONFIG *Config = &Config{
	Host: "localhost",
	Address: ":5103",
	Driver: "memory",
	DataSource: "",
	MinKeyLength: 6,
	KeySalt: "",
	AddDefaultRules: false,
}

func NewConfigFromArgs() *Config {
	config := new(Config)
	DefineFlags(config)
	// TODO: think about making configPath a field in the Config struct
	configPath := flag.String("config", "", "path to config file. E.g: /path/to/config.json")
	flag.Parse()
	if *configPath != "" {
		if config.HasNonDefaultValue() {
			log.Fatal("You can't mix -config option with other options")
		}
		return AddConfigFromFile(*configPath, config)
	}
	return config
}

func DefineFlags(config *Config) {
	flag.StringVar(&config.Host, "host", DEFAULT_CONFIG.Host, "deployment host. E.g: localhost")
	flag.StringVar(&config.Address, "address", DEFAULT_CONFIG.Address, "address to listen on. E.g: 0.0.0.0:8000")
	flag.StringVar(&config.Driver, "driver", DEFAULT_CONFIG.Driver, `database driver. One of: memory, sqlite3, mysql, or postgres.
	Default is memory`)
	flag.StringVar(&config.DataSource, "data-source", DEFAULT_CONFIG.DataSource,
		"data source name. E.g: postgres://user:password@localhost/dbname")
	flag.IntVar(&config.MinKeyLength, "min-key-length", DEFAULT_CONFIG.MinKeyLength,
		"minimum hashids key length. E.g: 8")
	flag.StringVar(&config.KeySalt, "key-salt", DEFAULT_CONFIG.KeySalt, "hashids key salt. E.g: kj8ioIxZ")
	flag.BoolVar(&config.AddDefaultRules, "add-default-rules", DEFAULT_CONFIG.AddDefaultRules, "Add default rules?")
}

func AddConfigFromFile(path string, config *Config) *Config {
	ValidateConfig(path)
	Unmarshal(path, config)
	return config
}

func Unmarshal(path string, i interface{}) {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatalf("Can't read config %s: %v", path, err)
	}
	err = json.Unmarshal(bytes, i)
	if err != nil {
		log.Fatalf("Bad JSON in %s: %v", path, err)
	}
}

func ValidateConfig(path string) {
	m := make(map[string]interface{})
	Unmarshal(path, &m)
	knownFields := GetConfigFieldNames()
	key, foundUnknown := FindUnknownField(m, knownFields)
	if foundUnknown {
		log.Fatalf("Found unknown key <%s> in config %s. Allowed keys: %s", key, path, knownFields)
	}
}

func GetConfigFieldNames() []string {
	return GetStructFieldNames(reflect.TypeOf(Config{}))
}

func GetStructFieldNames(typ reflect.Type) []string {
	fields := make([]string, 0, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		fields = append(fields, typ.Field(i).Name)
	}
	return fields
}

func FindUnknownField(m map[string]interface{}, knownFields []string) (key string, foundUnknown bool) {
	for _, key := range GetMapKeys(m) {
		if !Contains(knownFields, key) {
			return key, true
		}
	}
	return "", false
}

func GetMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}

func Contains(items []string, elem string) bool {
	for _, item := range items {
		if item == elem {
			return true
		}
	}
	return false
}

func (config *Config) HasNonDefaultValue() bool {
	return *config != *DEFAULT_CONFIG
}
