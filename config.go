package main

import (
	"flag"
	"log"
)

type Config struct {
	address             string
	driver              string
	dataSource          string
	minSiteLength       int
	siteSalt            string
	createDefaultRules  bool
	singleDomainUrlPath string
}

var DEFAULT_CONFIG *Config = &Config{
	address:             ":5103",
	driver:              "memory",
	dataSource:          "",
	minSiteLength:       6,
	siteSalt:            "",
	createDefaultRules:  false,
	singleDomainUrlPath: "/goslow/",
}

func NewConfigFromArgs() *Config {
	config := new(Config)
	config.defineFlags()
	flag.Parse()
	config.checkFlags()
	return config
}

func (config *Config) defineFlags() {
	flag.StringVar(&config.address, "address", DEFAULT_CONFIG.address, "address to listen on. E.g: 0.0.0.0:8000")
	flag.StringVar(&config.driver, "driver", DEFAULT_CONFIG.driver, `database driver. One of: memory, sqlite3, mysql, or postgres.
	Default is memory`)
	flag.StringVar(&config.dataSource, "data-source", DEFAULT_CONFIG.dataSource,
		"data source name. E.g: postgres://user:password@localhost/dbname")
	flag.IntVar(&config.minSiteLength, "min-site-length", DEFAULT_CONFIG.minSiteLength,
		"minimum length of the randomly generated site names. E.g: 8")
	flag.StringVar(&config.siteSalt, "site-salt", DEFAULT_CONFIG.siteSalt, "random names salt. E.g: kj8ioIxZ")
	flag.BoolVar(&config.createDefaultRules, "create-default-rules", DEFAULT_CONFIG.createDefaultRules, "Create default rules?")
	flag.StringVar(&config.singleDomainUrlPath, "single-domain-url-path", DEFAULT_CONFIG.singleDomainUrlPath,
		"run in single domain mode, use localhost/goslow for configuration")
}

func (config *Config) checkFlags() {
	if config.createDefaultRules && config.singleDomainUrlPath != "" {
		log.Fatal("You can't use both --single-domain-url-path and --create-default-rules options")
	}

}
