package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/boltdb/bolt"
)

func respondDeploy(args map[string]string, queue *JobQueue) (response string, err error) {
	job, err := queue.AddJob(args)
	if err != nil {
		return
	}

	result := DeployResponse{
		Job: job,
	}
	buf, err := json.Marshal(result)
	if err != nil {
		return
	}
	response = string(buf)
	return
}

func respondRestart(args map[string]string, db *bolt.DB, queue *JobQueue) (response string, err error) {
	jobNumber, err := strconv.Atoi(args["job"])
	var job Job
	err = db.View(func(tx *bolt.Tx) error {
		jobs := tx.Bucket([]byte("jobs"))
		rawJob := jobs.Get(itob(jobNumber))
		err := json.Unmarshal(rawJob, &job)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return
	}

	job, err = queue.AddJob(job.Args)
	if err != nil {
		return
	}

	result := DeployResponse{
		Job: job,
	}
	buf, err := json.Marshal(result)
	if err != nil {
		return
	}
	response = string(buf)
	return
}

func respondStatus(args map[string]string, db *bolt.DB) (response string, err error) {
	statuses := make([]Job, 0)

	var job Job
	err = db.View(func(tx *bolt.Tx) error {
		jobs := tx.Bucket([]byte("jobs"))
		if jobId, singleJob := args["job"]; singleJob {
			jobNumber, err := strconv.Atoi(jobId)
			if err != nil {
				return err
			}

			rawJob := jobs.Get(itob(jobNumber))
			err = json.Unmarshal(rawJob, &job)
			if err != nil {
				return err
			}

			statuses = append(statuses, job)
		} else {
			cursor := jobs.Cursor()
			for k, v := cursor.Last(); k != nil; k, v = cursor.Prev() {
				err = json.Unmarshal(v, &job)
				if err != nil {
					return err
				}
				statuses = append(statuses, job)
			}
		}
		return nil
	})
	if err != nil {
		return
	}

	result := StatusResponse{
		Statuses: statuses,
	}
	buf, err := json.Marshal(result)
	if err != nil {
		return
	}
	response = string(buf)
	return
}

func respondLogs(args map[string]string, db *bolt.DB) (response string, err error) {
	jobNumber, err := strconv.Atoi(args["job"])
	if err != nil {
		return
	}

	var job Job
	logs := make([]string, 0)

	err = db.View(func(tx *bolt.Tx) error {
		jobs := tx.Bucket([]byte("jobs"))
		rawJob := jobs.Get(itob(jobNumber))
		err := json.Unmarshal(rawJob, &job)
		if err != nil {
			return err
		}

		lb := tx.Bucket([]byte("logs"))
		jobLogs := lb.Bucket([]byte(fmt.Sprintf("job-%d", jobNumber)))
		cursor := jobLogs.Cursor()

		for k, msg := cursor.First(); k != nil; k, msg = cursor.Next() {
			logs = append(logs, string(msg))
		}

		return nil
	})
	if err != nil {
		return
	}

	result := LogsResponse{
		Job:  job,
		Logs: logs,
	}
	serialized, err := json.Marshal(result)
	if err != nil {
		return
	}
	response = string(serialized)
	return
}

func jobWorker(id int, queue *JobQueue, db *bolt.DB) {
	logger := log.New(os.Stdout, fmt.Sprintf("worker%d: ", id), log.LstdFlags)
	logger.Println("Worker started.")
	for job := range queue.Output {
		func() {
			logger.Printf("Starting job #%d", job.Number)

			writer, err := NewDbWriter(db, 500*time.Millisecond, "logs",
				fmt.Sprintf("job-%d", job.Number))
			if err != nil {
				logger.Printf("Error opening database writer: %v", err)
			}
			defer writer.Close()

			jobLogger := log.New(writer, "", log.LstdFlags)
			err = RunJob(job, jobLogger)
			if err == nil {
				logger.Printf("Job #%d succeeeded", job.Number)
				queue.FinishJob(job, true)
			} else {
				logger.Printf("Job #%d failed: %v", job.Number, err)
				queue.FinishJob(job, false)
			}
		}()
	}
	logger.Println("Worker stopped.")
}

func acceptLoop(listener net.Listener, wg *sync.WaitGroup) <-chan net.Conn {
	output := make(chan net.Conn)
	wg.Add(1)

	go func() {
		defer wg.Done()
		defer close(output)

		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			output <- conn
		}
	}()

	return output
}

func executeCommand(command ClientCommand, queue *JobQueue, db *bolt.DB) (response string, err error) {
	switch command.Command {
	case "deploy":
		response, err = respondDeploy(command.Args, queue)
	case "restart":
		response, err = respondRestart(command.Args, db, queue)
	case "logs":
		response, err = respondLogs(command.Args, db)
	case "status":
		response, err = respondStatus(command.Args, db)
	case "shutdown":
		response = ""
		err = fmt.Errorf("Shutdown")
	default:
		err = fmt.Errorf("Unknown command: %s", command.Command)
	}
	return
}

func RunServer(args []string, options map[string]string) int {
	sigs := make(chan os.Signal, 8)
	defer close(sigs)
	signal.Notify(sigs, os.Interrupt, os.Kill)

	listen, err := net.Listen("unix", SOCKET_PATH)
	if err != nil {
		log.Printf("Error starting server: %v", err)
		return 1
	}
	defer os.Remove(SOCKET_PATH)
	defer listen.Close()

	err = os.Chmod(SOCKET_PATH, 0666)
	if err != nil {
		log.Printf("Error setting up socket: %v", err)
		return 1
	}

	db, err := bolt.Open(DB_PATH, 0600, nil)
	if err != nil {
		log.Printf("Error opening database: %v", err)
		return 1
	}
	defer db.Close()

	queue, err := NewJobQueue(db, "jobs")
	if err != nil {
		log.Printf("Error creating job queue: %v", err)
		return 1
	}

	var wg sync.WaitGroup
	for i := 1; i <= 1; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			jobWorker(i, queue, db)
		}(i)
	}

	conns := acceptLoop(listen, &wg)

	running := true
	var conn net.Conn
	for running {
		select {
		// continue with the rest of the loop
		case conn = <-conns:
		// all the signals we've installed are interrupt/kill/terminate etc
		case <-sigs:
			log.Println("Received shutdown signal")
			running = false
			continue
		}
		scanner := bufio.NewScanner(conn)

		var command ClientCommand
		scanner.Scan()
		err = json.Unmarshal(scanner.Bytes(), &command)
		if err != nil {
			log.Printf("Error: %v", err)
			continue
		}

		response, err := executeCommand(command, queue, db)
		if err != nil {
			if err.Error() == "Shutdown" {
				log.Println("Shutdown command received")
				running = false
			} else if strings.HasPrefix(err.Error(), "Unknown command") {
				log.Println(err.Error())
			} else {
				panic(err)
			}
		}

		fmt.Fprintf(conn, "%s\n", response)

		conn.Close()
	}
	listen.Close()

	queue.Close()
	wg.Wait()

	return 0
}
