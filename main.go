package main

import (
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"time"

	fsnotify "github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v2"

	"github.com/Luzifer/rconfig"
	"github.com/mitchellh/go-homedir"
	"github.com/xuyu/goredis"
)

const hashSize = 40

var (
	cfg = struct {
		RedisConnectString string        `flag:"redis-connect" vardefault:"redis-connect" env:"REDIS_CONNECT" description:"Connection string for the redis server to use"`
		RedisKey           string        `flag:"redis-key,k" vardefault:"redis-key" description:"Key to write the data to"`
		TTL                time.Duration `flag:"ttl" vardefault:"ttl" description:"When to expire the key (0=never)"`
		WatchInterval      time.Duration `flag:"watch-interval" vardefault:"watch-interval" description:"How often to look for changed key"`

		MkConfig bool `flag:"create-config" default:"false" description:"Create a default configuration file"`

		VersionAndExit bool `flag:"version" default:"false" description:"Prints current version and exits"`
	}{}

	defaultConfig = map[string]string{
		"redis-connect":  "",
		"redis-key":      "io.luzifer.redpaste",
		"ttl":            "0",
		"watch-interval": "2s",
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

	if len(rconfig.Args()) < 2 {
		usage()
	}
}

func usage() {
	fmt.Println("Usage: redpaste <set/get>")
	fmt.Println("       redpaste <watch/edit> <file>")
	os.Exit(1)
}

func getAndDecode(rc *goredis.Redis) []byte {
	hash, err := rc.GetRange(cfg.RedisKey, 0, hashSize-1)
	if err != nil {
		log.Fatalf("Unable to read key: %s", err)
	}

	data, err := rc.GetRange(cfg.RedisKey, hashSize, -1)
	if err != nil {
		log.Fatalf("Unable to read key: %s", err)
	}

	if fmt.Sprintf("%x", sha1.Sum([]byte(data))) != hash {
		log.Fatalf("Detected a checksum mismatch. Please try again.")
	}

	rawData, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		log.Fatalf("Unable to decode base64 content: %s", err)
	}

	return rawData
}

func encodeAndSet(rc *goredis.Redis, data []byte) {
	value := base64.StdEncoding.EncodeToString(data)
	hash := fmt.Sprintf("%x", sha1.Sum([]byte(value)))

	if err := rc.Set(cfg.RedisKey, hash+value, int(cfg.TTL.Seconds()), 0, false, false); err != nil {
		log.Fatalf("Unable to write key: %s", err)
	}
}

func main() {
	initConfig()
	action := rconfig.Args()[1]

	rc, err := goredis.DialURL(cfg.RedisConnectString)
	if err != nil {
		log.Fatalf("Unable to connect to redis: %s", err)
	}

	switch action {
	case "get":
		os.Stdout.Write(getAndDecode(rc))
		os.Exit(0)

	case "set":
		data, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			log.Fatalf("Unable to read data from stdin: %s", err)
		}

		encodeAndSet(rc, data)

	case "watch":
		if len(rconfig.Args()) != 3 {
			usage()
		}

		var storedHash string

		for range time.Tick(cfg.WatchInterval) {
			hash, err := rc.GetRange(cfg.RedisKey, 0, hashSize)
			if err != nil {
				log.Fatalf("Unable to read key: %s", err)
			}

			if hash == storedHash {
				continue
			}

			if err := ioutil.WriteFile(rconfig.Args()[2], getAndDecode(rc), 0755); err != nil {
				log.Fatalf("Unable to write file '%s': %s", rconfig.Args()[2], err)
			}
			storedHash = hash
		}

	case "edit":
		if len(rconfig.Args()) != 3 {
			usage()
		}

		editor := os.Getenv("EDITOR")
		if editor == "" {
			log.Fatalf("Evironment variable $EDITOR is unset")
		}

		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			log.Fatalf("Unable to create file system watcher")
		}
		defer watcher.Close()

		if _, err := os.Stat(rconfig.Args()[2]); err != nil {
			if err := ioutil.WriteFile(rconfig.Args()[2], []byte{}, 0644); err != nil {
				log.Fatalf("Unable to create file '%s': %s", rconfig.Args()[2], err)
			}
		}

		doneChan := make(chan error)
		go func() {
			cmd := exec.Command(editor, rconfig.Args()[2])
			cmd.Stdout = os.Stdout
			cmd.Stdin = os.Stdin
			cmd.Stderr = os.Stderr
			cmd.Env = os.Environ()
			doneChan <- cmd.Run()
		}()

		if err := watcher.Add(rconfig.Args()[2]); err != nil {
			log.Fatalf("Unable to watch file: %s", err)
		}

		for {
			select {
			case err := <-doneChan:
				if err != nil {
					os.Exit(1)
				} else {
					os.Exit(0)
				}

			case event := <-watcher.Events:
				switch {
				case event.Op&fsnotify.Remove == fsnotify.Remove:
					watcher.Add(event.Name)
				case event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create:
					data, err := ioutil.ReadFile(rconfig.Args()[2])
					if err != nil {
						log.Fatalf("Unable to read file contents: %s", err)
					}
					encodeAndSet(rc, data)
				}
			}
		}

	default:
		usage()
	}

}
