package main

import (
	"log"
	"os"

	yaml "gopkg.in/yaml.v2"
)

type Tasks struct {
	Name       string                 `yaml:"name"`
	Action     string                 `yaml:"action"`
	Foreach    string                 `yaml:"foreach"`
	Txn        string                 `yaml:"txn"`
	Deps       []string               `yaml:"deps"`
	Params     map[string]interface{} `yaml:"params"`
	OnSuccess  []interface{}          `yaml:"on-success"`
	OnError    []interface{}          `yaml:"on-error"`
	OnRollback []interface{}          `yaml:"on-rollback"`
}

type Config struct {
	ApiVersion string   `yaml:"apiVersion"`
	Action     string   `yaml:"action"`
	Name       string   `yaml:"description"`
	Type       string   `yaml:"type"`
	InputField []string `yaml:"inputField"`
	Tasks      []Tasks  `yaml:"tasks"`
	OnSuccess  []string `yaml:"on-success"`
	OnError    []string `yaml:"on-error"`
	OnRollback []string `yaml:"on-rollback"`
}

func ParseYml(f_name string) Config {
	var config Config
	File, err := os.ReadFile(f_name)
	if err != nil {
		log.Printf("读取配置文件失败 #%v", err)
	}
	err = yaml.Unmarshal(File, &config)
	if err != nil {
		log.Fatalf("解析失败: %v", err)
	}

	// fmt.Println(config)

	return config
}
