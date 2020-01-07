package main

import (
	"os"
	"strings"

	"github.com/teris-io/cli"
)

var ENV_PREFIX = "INTEGRAD_"

var SOCKET_PATH string
var DB_PATH string
var SHELL string

func main() {

	SOCKET_PATH = getEnvConfig("SOCKET", "/var/integrad/integrad.sock")
	DB_PATH = getEnvConfig("DB", "/var/integrad/integrad.db")
	SHELL = getEnvConfig("SHELL", "bash")

	status := cli.NewCommand("status", "view status of jobs").
		WithOption(cli.NewOption("job", "job ID").WithChar('j').WithType(cli.TypeInt)).
		WithAction(StatusCommand)

	logs := cli.NewCommand("logs", "view the logs for a job").
		WithArg(cli.NewArg("job", "job ID")).
		WithAction(LogsCommand)

	shutdown := cli.NewCommand("shutdown", "shutdown the server").
		WithAction(ShutdownCommand)

	deploy := cli.NewCommand("deploy", "deploy a project").
		WithOption(cli.NewOption("git", "git branch or commit hash").WithChar('g')).
		WithArg(cli.NewArg("source", "location of the project source")).
		WithAction(DeployCommand)

	restart := cli.NewCommand("restart", "restart a job").
		WithArg(cli.NewArg("job", "job ID").WithType(cli.TypeInt)).
		WithAction(RestartCommand)

	server := cli.NewCommand("server", "run the integrad server").
		WithAction(RunServer)

	app := cli.New("The world's tiniest CI system").
		WithCommand(server).
		WithCommand(shutdown).
		WithCommand(deploy).
		WithCommand(status).
		WithCommand(restart).
		WithCommand(logs)

	os.Exit(app.Run(os.Args, os.Stdout))
}

func getEnvConfig(name string, defaultValue string) string {
	if val := os.Getenv(ENV_PREFIX + name); strings.TrimSpace(val) != "" {
		return val
	} else {
		return defaultValue
	}
}
