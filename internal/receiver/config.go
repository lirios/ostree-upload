// SPDX-FileCopyrightText: 2020 Pier Luigi Fiorini <pierluigi.fiorini@gmail.com>
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package receiver

import (
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v2"
)

// Config represents the configuration file
type Config struct {
	path   string
	Tokens []*Token `yaml:"tokens"`
}

// CreateConfig creates the configuration file
func CreateConfig(path string) (*Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			return nil, err
		}
		file.Write([]byte{})
		file.Close()
	}

	return OpenConfig(path)
}

// OpenConfig opens path
func OpenConfig(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	buf, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(buf, &config); err != nil {
		return nil, err
	}

	config.path = path

	return &config, nil
}

// Save saves the configuration file
func (c *Config) Save() error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	file, err := os.Create(c.path)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		return err
	}

	return nil
}
