package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"reflect"
)

const (
	DEFAULT_DB             = ""
	DEFAULT_ADDRESS        = ":5103"
	DEFAULT_KEY_SALT       = ""
	DEFAULT_MIN_KEY_LENGTH = 6
)

type Config struct {
	Db           string
	Address      string
	KeySalt      string
	MinKeyLength int
}

func main() {
	config := NewConfigFromArgs()
	log.Printf("config: %+v", config)
}

func NewConfigFromArgs() *Config {
	argsConfig := new(Config)
	DefineFlags(argsConfig)
	configPath := flag.String("config", "", "Path to config file")
	flag.Parse()
	if *configPath != "" {
		if argsConfig.HasNonDefaultValue() {
			log.Fatalf("You can't mix --config option with other options")
		}
		return NewConfigFromFile(*configPath)
	}
	return argsConfig
}

func DefineFlags(config *Config) {
	flag.StringVar(&config.Db, "db", DEFAULT_DB, "Database connection string")
	flag.StringVar(&config.Address, "address", DEFAULT_ADDRESS, "Address to listen on")
	flag.StringVar(&config.KeySalt, "key-salt", DEFAULT_KEY_SALT, "hashids key salt")
	flag.IntVar(&config.MinKeyLength, "min-key-length", DEFAULT_MIN_KEY_LENGTH,
		"minimum hashids key length")
}

func NewConfigFromFile(path string) *Config {
	ValidateConfig(path)
	config := ReadConfig(path)

	return config
}

func ReadConfig(path string) *Config {
	config := new(Config)
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
	config := new(Config)
	m := make(map[string]interface{})
	Unmarshal(path, &m)
	key, foundExtra := GetExtraKey(m, config)
	if foundExtra {
		log.Fatalf("Found unknown key <%s> in config %s. Allowed keys: %s", key, path, GetStructFieldNames(*config))
	}
}

func GetExtraKey(m map[string]interface{}, config *Config) (key string, foundExtra bool) {
	structFields := GetStructFieldNames(*config)
	for _, key := range GetMapKeys(m) {
		if !Contains(structFields, key) {
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

func GetStructFieldNames(i interface{}) []string {
	typ := reflect.TypeOf(i)
	fields := make([]string, 0, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		fields = append(fields, typ.Field(i).Name)
	}
	return fields
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
	return config.Db != DEFAULT_DB || config.Address != DEFAULT_ADDRESS ||
		config.KeySalt != DEFAULT_KEY_SALT || config.MinKeyLength != DEFAULT_MIN_KEY_LENGTH
}
