package config

import (
	"io/ioutil"

	"github.com/go-yaml/yaml"
)

type ConfigData struct {
	ForwardSSHAgent bool     `yaml:"forward_ssh_agent"`
	SSHHostKeys     []string `yaml:"ssh_host_keys"`
	StatusPage      bool     `yaml:"status_page"`
	AllowPull       bool     `yaml:"allow_pull"`
	GinMode         string   `yaml:"gin_mode"`
	Listen          string   `yaml:"listen"`
	ApiKey          string   `yaml:"api_key"`
}

var Config ConfigData

func init() {
	var err error

	dat, err := ioutil.ReadFile("./config.yml")
	if err != nil {
		panic(err)
	}

	err = yaml.Unmarshal([]byte(dat), &Config)
	if err != nil {
		panic(err)
	}
}
