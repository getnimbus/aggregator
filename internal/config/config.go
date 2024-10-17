package config

import (
	"encoding/json"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/syndtr/goleveldb/leveldb"
	leveldbErrors "github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/valyala/fasthttp"

	"aggregator/internal/aggregator"
	"aggregator/internal/env"
	"aggregator/internal/log"
)

var (
	logger            = log.Module("config")
	locker            = sync.Mutex{}
	defaultPhishingDb = "https://cfg.rpchub.io/agg/scam-addresses.json"

	_Config = &Config{
		RequestTimeout:           90,
		MaxRetries:               3,
		PhishingDb:               []string{defaultPhishingDb},
		PhishingDbUpdateInterval: 3600,
	}
)

type Config struct {
	Proxy                    string                       `json:"proxy,omitempty"`
	RequestTimeout           int64                        `json:"request_timeout,omitempty"`
	MaxRetries               int                          `json:"max_retries,omitempty"`
	Nodes                    map[string][]aggregator.Node `json:"nodes"`
	PhishingDb               []string                     `json:"phishing_db"`
	PhishingDbUpdateInterval int64                        `json:"phishing_db_update_interval"`
	AuthorityDB              []AuthorityDB                `json:"authority_db"`
}

type AuthorityDB struct {
	Name   string `json:"name"`
	Url    string `json:"url"`
	Enable bool   `json:"enable"`
}

func (c Config) HasChain(chain string) bool {
	if len(chain) > 0 {
		if v, ok := c.Nodes[chain]; ok {
			if len(v) > 0 {
				return true
			}
		}
	}
	return false
}

func Clone() Config {
	locker.Lock()
	defer locker.Unlock()

	cfg := *_Config
	cfg.Nodes = map[string][]aggregator.Node{}

	for key, nodes := range _Config.Nodes {
		cfg.Nodes[key] = []aggregator.Node{}
		for _, node := range nodes {
			cfg.Nodes[key] = append(cfg.Nodes[key], node)
		}
	}
	return cfg
}

func Default() *Config {
	locker.Lock()
	defer locker.Unlock()

	return _Config
}

func SetDefault(cfg *Config) {
	locker.Lock()
	defer locker.Unlock()

	_Config = cfg
}

func LoadDefault() *Config {
	var cfg *Config

	retries := 0
	for {
		statusCode, data, err := (&fasthttp.Client{}).GetTimeout(nil, env.Config.DefaultConfigUrl, time.Second*30)
		if err == nil && statusCode == 200 {
			err = json.Unmarshal(data, &cfg)
			if err == nil {
				logger.Info("Load default config success from url " + env.Config.DefaultConfigUrl)
				break
			}
		}
		if err != nil || statusCode != 200 {
			retries++
			errStr := ""
			if err != nil {
				errStr = err.Error()
			}
			logger.Error("Load default config failed", "statusCode", statusCode, "error", errStr, "retries", retries)

			if retries >= 5 {
				logger.Warn("Load default config failed, See the documents for more details [https://docs.rpchub.io/]", errStr)
				break
			} else {
				time.Sleep(time.Second * 3)
			}
		}
	}

	return cfg
}

func Load() error {
	cfg := LoadDefault()

	db, err := leveldb.OpenFile("data/db", nil)
	if err != nil {
		logger.Error("Load Config failed", "error", err.Error())
		return err
	}
	defer db.Close()

	data, err := db.Get(aggregator.KeyDbConfig, nil)
	if err != nil && !errors.Is(err, leveldbErrors.ErrNotFound) {
		logger.Error("Load Config failed", "error", err.Error())
		return err
	}

	var cfgLocal *Config
	if data != nil {
		err = json.Unmarshal(data, &cfgLocal)
		if err != nil {
			logger.Error("Load Config failed", "error", err.Error())
			return err
		}

		if cfg != nil {
			for k, v := range cfg.Nodes {
				if cfgLocal.Nodes[k] == nil {
					cfgLocal.Nodes[k] = v
				}
			}

			dbs := cfg.AuthorityDB
			for i := 0; i < len(dbs); i++ {
				for _, adbLocal := range cfgLocal.AuthorityDB {
					if dbs[i].Name == adbLocal.Name {
						dbs[i].Enable = adbLocal.Enable
					}
				}
			}
			cfgLocal.AuthorityDB = dbs
		}
	}

	if cfgLocal != nil {
		_Config = cfgLocal
	} else {
		_Config = cfg
	}

	data, _ = json.Marshal(_Config)
	err = db.Put(aggregator.KeyDbConfig, data, nil)

	return err
}

func Save() error {
	db, err := leveldb.OpenFile("data/db", nil)
	if err != nil {
		logger.Error("Save Config failed", "error", err.Error())
		return err
	}
	defer db.Close()

	data, err := json.Marshal(Default())
	if err != nil {
		logger.Error("Save Config failed", "error", err.Error())
		return err
	}

	err = db.Put(aggregator.KeyDbConfig, data, nil)
	if err != nil {
		logger.Error("Save Config failed", "error", err.Error())
		return err
	}

	return nil
}

func Chains() []string {
	var chains []string
	for key, _ := range Default().Nodes {
		chains = append(chains, key)
	}
	sort.Strings(chains)
	return chains
}
