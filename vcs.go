package main

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/kr/text"
)

func GitSourceVersion(sourcePath, targetParent, version string, logger *log.Logger) (config BuildConfig, err error) {
	// id := version[:8]
	id := version

	config = BuildConfig{
		Source: filepath.Join(targetParent, "source-"+id),
		Build:  filepath.Join(targetParent, "build-"+id),
	}

	logger.Printf("Fetching commit %s...", id)
	err = os.MkdirAll(config.Source, 0755)
	if err == nil {
		err = os.MkdirAll(config.Build, 0755)
	}
	if err == nil {
		_, err = RunCommand(config.Source, "git", "clone", sourcePath, ".")
	}
	if err == nil {
		_, err = RunCommand(config.Source, "git", "checkout", version)
	}
	if err == nil {
		logger.Println("Fetch successful.")
	} else {
		if ee, ok := err.(*exec.ExitError); ok {
			logger.Printf("Fetch failed: \n%s",
				text.Indent(string(ee.Stderr), "    "))
		} else {
			logger.Printf("Fetch failed: \n%s",
				text.Indent(err.Error(), "    "))
		}
	}

	return
}
