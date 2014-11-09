package main

import (
	"flag"
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
	flag.Parse()
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
