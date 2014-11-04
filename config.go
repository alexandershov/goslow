package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"reflect"
)

const (
	DEFAULT_HOST           = "localhost"
	DEFAULT_ADDRESS        = ":5103"
	DEFAULT_DB             = ""
	DEFAULT_MIN_KEY_LENGTH = 6
	DEFAULT_KEY_SALT       = ""
)

type Config struct {
	Host         string
	Address      string
	Db           string
	MinKeyLength int
	KeySalt      string
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
	flag.StringVar(&config.Host, "host", DEFAULT_HOST, "deployment host. E.g: localhost")
	flag.StringVar(&config.Address, "address", DEFAULT_ADDRESS, "address to listen on. E.g: 0.0.0.0:8000")
	flag.StringVar(&config.Db, "db", DEFAULT_DB, `database connection string. E.g: postgres://user:password@localhost/dbname.
	Goslow will use the in-memory store if you don't specify the connection string`)
	flag.IntVar(&config.MinKeyLength, "min-key-length", DEFAULT_MIN_KEY_LENGTH,
		"minimum hashids key length. E.g: 8")
	flag.StringVar(&config.KeySalt, "key-salt", DEFAULT_KEY_SALT, "hashids key salt. E.g: kj8ioIxZ")
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
	return config.Host != DEFAULT_HOST || config.Address != DEFAULT_ADDRESS ||
		config.Db != DEFAULT_DB || config.MinKeyLength != DEFAULT_MIN_KEY_LENGTH ||
		config.KeySalt != DEFAULT_KEY_SALT
}
