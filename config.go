package main

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"text/template"

	"gopkg.in/yaml.v2"
)

type BuildConfig struct {
	Source string
	Build  string
}

type Config struct {
	Env    map[string]string
	Build  []string
	Deploy map[string]string
	Post   []string
}

func LoadConfig(build BuildConfig) (config Config, err error) {
	filepath := filepath.Join(build.Source, "deploy.yaml")
	file, err := os.Open(filepath)
	if err != nil {
		return
	}
	defer file.Close()

	b, err := ioutil.ReadAll(file)
	contents := string(b)

	template, err := template.New("config").Parse(contents)
	if err != nil {
		return
	}

	buffer := new(bytes.Buffer)
	err = template.Execute(buffer, build)
	if err != nil {
		return
	}

	err = yaml.Unmarshal(buffer.Bytes(), &config)
	return
}
