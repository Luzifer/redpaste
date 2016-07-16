package main

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/Luzifer/rconfig"
	"github.com/mitchellh/go-homedir"
	"github.com/xuyu/goredis"
)

var (
	cfg = struct {
		RedisConnectString string        `flag:"redis-connect" vardefault:"redis-connect" env:"REDIS_CONNECT" description:"Connection string for the redis server to use"`
		RedisKey           string        `flag:"redis-key,k" vardefault:"redis-key" description:"Key to write the data to"`
		TTL                time.Duration `flag:"ttl" vardefault:"ttl" description:"When to expire the key (0=never)"`

		MkConfig bool `flag:"create-config" default:"false" description:"Create a default configuration file"`

		VersionAndExit bool `flag:"version" default:"false" description:"Prints current version and exits"`
	}{}

	defaultConfig = map[string]string{
		"redis-connect": "",
		"redis-key":     "io.luzifer.redpaste",
		"ttl":           "0",
	}

	version = "dev"
)

func writeDefaultConfig() error {
	c := getDefaultConfig()

	cfgDir, err := homedir.Expand("~/.config")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		return err
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	cfgFile, err := homedir.Expand("~/.config/redpaste.yml")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(cfgFile, data, 0600)
}

func getDefaultConfig() map[string]string {
	c := map[string]string{}
	cfgFile, err := homedir.Expand("~/.config/redpaste.yml")
	if err != nil {
		log.Fatalf("Unable to determine config file location: %s", err)
	}
	if _, err := os.Stat(cfgFile); err != nil {
		return defaultConfig
	}

	data, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		log.Fatalf("Unable to read config file: %s", err)
	}
	if err := yaml.Unmarshal(data, &c); err != nil {
		log.Fatalf("Unable to parse config file: %s", err)
	}

	for k, v := range defaultConfig {
		if _, ok := c[k]; !ok {
			c[k] = v
		}
	}
	return c
}

func initConfig() {
	rconfig.SetVariableDefaults(getDefaultConfig())
	if err := rconfig.Parse(&cfg); err != nil {
		log.Fatalf("Unable to parse commandline options: %s", err)
	}

	if cfg.VersionAndExit {
		fmt.Printf("redpaste %s\n", version)
		os.Exit(0)
	}

	if cfg.MkConfig {
		if err := writeDefaultConfig(); err != nil {
			log.Fatalf("Unable to write default config file: %s", err)
		}
		fmt.Println("Wrote default configuration at ~/.config/redpaste.yml")
		os.Exit(0)
	}

	if cfg.RedisConnectString == "" {
		log.Fatalf("You need to specify redis connection string")
	}

	if len(rconfig.Args()) != 2 {
		fmt.Println("Usage: redpaste <set/get>")
		os.Exit(1)
	}
}

func main() {
	initConfig()
	action := rconfig.Args()[1]

	rc, err := goredis.DialURL(cfg.RedisConnectString)
	if err != nil {
		log.Fatalf("Unable to connect to redis: %s", err)
	}

	if action == "get" {
		data, err := rc.Get(cfg.RedisKey)
		if err != nil {
			log.Fatalf("Unable to read key: %s", err)
		}
		rawData, err := base64.StdEncoding.DecodeString(string(data))
		if err != nil {
			log.Fatalf("Unable to decode base64 content: %s", err)
		}
		os.Stdout.Write(rawData)
		os.Exit(0)
	}

	data, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		log.Fatalf("Unable to read data from stdin: %s", err)
	}

	rc.Set(cfg.RedisKey, base64.StdEncoding.EncodeToString(data), int(cfg.TTL.Seconds()), 0, false, false)
}
