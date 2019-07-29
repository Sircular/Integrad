package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/kr/text"
)

func mapEnv(env []string) func(string) string {
	envMap := make(map[string]string)
	for _, v := range env {
		split := strings.Split(v, "=")
		envMap[split[0]] = split[1]
	}
	return func(key string) string {
		return envMap[key]
	}
}

func RunJob(job Job, logger *log.Logger) error {

	source := job.Args["source"]
	var build BuildConfig

	var err error
	if version, ok := job.Args["git"]; ok {
		build, err = GitSourceVersion(source, "/tmp/integrad", version, logger)
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("VCS version must be provided")
	}
	defer os.RemoveAll(build.Source)
	defer os.RemoveAll(build.Build)

	err = RunDeploy(build, logger)
	if err != nil {
		return err
	}

	return nil
}

func RunDeploy(build BuildConfig, logger *log.Logger) error {

	config, err := LoadConfig(build)
	if err != nil {
		logger.Printf("Error loading configuration:\n %v",
			text.Indent(err.Error(), "    "))
		return err
	}
	env := os.Environ()
	for k, v := range config.Env {
		env = append(env, k+"="+v)
	}

	for i, cmd := range config.Build {
		logger.Printf("Running build command %d/%d: %s", i+1, len(config.Build), cmd)
		output, err := RunCommandEnv(build.Source, env, SHELL, "-c", cmd)
		if trimmed := strings.TrimSpace(output); len(trimmed) > 0 {
			logger.Println(text.Indent(trimmed, "    "))
		}
		if err != nil {
			logger.Printf("Error while running command: %v", err)
			return err
		}
	}

	lookup := mapEnv(env)
	for source, dest := range config.Deploy {
		if !filepath.IsAbs(source) {
			source = filepath.Join(build.Build, source)
		}
		source = os.Expand(source, lookup)
		dest = os.Expand(dest, lookup)
		logger.Printf("Deploying '%s' to %s'", source, dest)
		err = MoveAll(source, dest)
		if err != nil {
			logger.Printf("Error while moving file: %v", err)
			return err
		}
	}

	for i, cmd := range config.Post {
		logger.Printf("Running post-build command %d/%d: %s", i+1, len(config.Build), cmd)
		output, err := RunCommandEnv(build.Build, env, SHELL, "-c", cmd)
		logger.Println(text.Indent(output, "    "))
		if err != nil {
			logger.Printf("Error while running command: %v", err)
			return err
		}
	}

	if err == nil {
		logger.Println("Deploy succeeded.")
	} else {
		logger.Println("Deploy failed.")
	}
	return err
}

func RunCommand(cwd, name string, args ...string) (string, error) {
	return RunCommandEnv(cwd, os.Environ(), name, args...)
}

func RunCommandEnv(cwd string, env []string, name string, args ...string) (string, error) {
	var buffer bytes.Buffer
	writer := bufio.NewWriter(&buffer)

	cmd := exec.Command(name, args...)
	cmd.Dir = cwd
	cmd.Env = env
	cmd.Stdout = writer
	cmd.Stderr = writer

	err := cmd.Run()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			ee.Stderr = buffer.Bytes()
		}
	}
	return buffer.String(), err
}

func MoveAll(source, dest string) error {
	var wg sync.WaitGroup
	walk := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		destPath := filepath.Join(dest, path[len(source):])
		perms := info.Mode().Perm()

		if info.IsDir() {
			os.MkdirAll(destPath, perms)
		} else {
			wg.Add(1)
			go func() {
				defer wg.Done()

				os.MkdirAll(filepath.Dir(destPath), 0755)

				read, err := os.Open(path)
				if err != nil {
					return
				}
				defer read.Close()

				write, err := os.Create(destPath)
				if err != nil {
					log.Panic(err)
				}
				defer write.Chmod(perms)
				defer write.Close()

				_, err = io.Copy(write, read)
			}()
		}
		return nil
	}
	err := filepath.Walk(source, walk)
	wg.Wait()
	if err != nil {
		return err
	}

	return nil
}
