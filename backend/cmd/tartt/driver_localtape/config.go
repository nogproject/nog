package driver_localtape

import (
	yaml "gopkg.in/yaml.v2"
)

type config struct {
	Stores []storeConfig `yaml:"stores"`
}

type storeConfig struct {
	Name      string          `yaml:"name"`
	Localtape localtapeConfig `yaml:"localtape"`
}

type localtapeConfig struct {
	Tardir string `yaml:"tardir"`
}

func parseConfig(cfgYml []byte) (*config, error) {
	var cfg config
	if err := yaml.Unmarshal(cfgYml, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (cfg *config) findTardir(storeName string) string {
	for _, s := range cfg.Stores {
		if s.Name == storeName {
			return s.Localtape.Tardir
		}
	}
	return ""
}
