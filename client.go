package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"
)

const DATE_LAYOUT = "2006-01-02 03:04:05"

type ClientCommand struct {
	Command string
	Args    map[string]string
}

type StatusResponse struct {
	Statuses []Job
}

type LogsResponse struct {
	Job  Job
	Logs []string
}

type DeployResponse struct {
	Job Job
}

func StatusCommand(args []string, options map[string]string) int {
	jobArgs := make(map[string]string)
	jobNumber, singleJob := options["job"]
	if singleJob {
		jobArgs["job"] = jobNumber
	}
	command := ClientCommand{
		Command: "status",
		Args:    jobArgs,
	}

	var response StatusResponse
	err := sendCommand(command, &response)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return 1
	}

	if singleJob {
		job := response.Statuses[0]
		fmt.Printf("Job status for Job #%d: %s as of %s\n",
			job.Number, job.Status.GetName(), job.Updated.Format(DATE_LAYOUT))
	} else {
		fmt.Printf("%6s %10s %20s\n", "JOB", "STATUS", "UPDATED")
		for _, job := range response.Statuses {
			name := fmt.Sprintf("#%d", job.Number)
			fmt.Printf("%6s %10s %20s\n",
				name, job.Status.GetName(), job.Updated.Format(DATE_LAYOUT))
		}
	}

	return 0
}

func LogsCommand(args []string, options map[string]string) int {
	command := ClientCommand{
		Command: "logs",
		Args: map[string]string{
			"job": args[0],
		},
	}
	var response LogsResponse

	err := sendCommand(command, &response)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return 1
	}

	fmt.Printf("Logs for Job #%d\n\n", response.Job.Number)

	for _, msg := range response.Logs {
		fmt.Print(msg)
	}

	return 0
}

func DeployCommand(args []string, options map[string]string) int {
	absPath, err := filepath.Abs(args[0])
	if err != nil {
		fmt.Printf("Error: %v\n", absPath)
	}

	if _, ok := options["git"]; !ok {
		fmt.Printf("Error: a git commit must be provided.")
		return 1
	}

	command := ClientCommand{
		Command: "deploy",
		Args: map[string]string{
			"source": absPath,
			"git":    options["git"],
		},
	}
	var response DeployResponse

	err = sendCommand(command, &response)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return 1
	}

	fmt.Printf("Created job #%d.\n", response.Job.Number)

	return 0
}

func RestartCommand(args []string, options map[string]string) int {
	command := ClientCommand{
		Command: "restart",
		Args: map[string]string{
			"job": args[0],
		},
	}
	var response DeployResponse

	err := sendCommand(command, &response)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return 1
	}

	fmt.Printf("Created job #%d.\n", response.Job.Number)

	return 0
}

func ShutdownCommand(args []string, options map[string]string) int {
	command := ClientCommand{
		Command: "shutdown",
		Args:    map[string]string{},
	}

	err := sendCommand(command, nil)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return 1
	}

	return 0
}

func sendCommand(command ClientCommand, response interface{}) error {
	conn, err := net.Dial("unix", SOCKET_PATH)
	if err != nil {
		return err
	}
	defer conn.Close()

	buffer, err := json.Marshal(command)
	if err != nil {
		return err
	}

	fmt.Fprintf(conn, "%s\n", string(buffer))

	scanner := bufio.NewScanner(conn)
	scanner.Scan()
	rawResponse := scanner.Bytes()

	if response != nil {
		err = json.Unmarshal(rawResponse, &response)
		if err != nil {
			return err
		}
	}
	return nil
}
