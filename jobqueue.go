package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/boltdb/bolt"
)

type JobStatus int

const (
	Queued JobStatus = iota
	Active
	Succeeded
	Failed
)

func (status JobStatus) GetName() string {
	values := []string{
		"Queued",
		"Active",
		"Succeeded",
		"Failed",
	}
	return values[status]
}

type Job struct {
	Number  int
	Args    map[string]string
	Status  JobStatus
	Updated time.Time
}

type JobQueue struct {
	Output chan Job

	name    string
	db      *bolt.DB
	current int
	open    bool
	notify  chan interface{}
	wg      sync.WaitGroup
}

func NewJobQueue(db *bolt.DB, name string) (queue *JobQueue, err error) {
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(name))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		return nil
	})
	if err != nil {
		return
	}

	current := 1
	err = db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(name))
		jobCount := bucket.Stats().KeyN
		cursor := bucket.Cursor()

		var job Job
		queuedCount := 0

		for k, v := cursor.Last(); k != nil; k, v = cursor.Prev() {
			err := json.Unmarshal(v, &job)
			if err != nil {
				return err
			}
			if job.Status != Queued {
				break
			}
			queuedCount++
		}
		current = (jobCount - queuedCount) + 1
		return nil
	})
	if err != nil {
		return
	}

	output := make(chan Job)
	notify := make(chan interface{})

	queue = &JobQueue{
		name:    name,
		db:      db,
		notify:  notify,
		current: current,
		open:    true,
		Output:  output,
	}
	go queue.readWorker()
	queue.notifyWorker()
	return
}

func (queue *JobQueue) AddJob(args map[string]string) (Job, error) {
	job := Job{
		Args: args,
	}
	err := queue.db.Batch(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(queue.name))

		id, _ := bucket.NextSequence()
		job.Number = int(id)
		job.Status = Queued
		job.Updated = time.Now()

		buf, err := json.Marshal(job)
		if err != nil {
			return err
		}

		return bucket.Put(itob(job.Number), buf)
	})
	log.Printf("Added job %d", job.Number)

	queue.notifyWorker()
	return job, err
}

func (queue *JobQueue) FinishJob(job Job, success bool) {
	newStatus := Succeeded
	if !success {
		newStatus = Failed
	}
	queue.wg.Add(1)
	go func() {
		defer queue.wg.Done()
		queue.db.Batch(func(tx *bolt.Tx) error {
			bucket := tx.Bucket([]byte(queue.name))
			key := itob(job.Number)

			buf := bucket.Get(key)
			err := json.Unmarshal(buf, &job)
			if err != nil {
				return err
			}

			job.Status = newStatus
			buf, err = json.Marshal(job)
			if err != nil {
				return err
			}

			return bucket.Put(key, buf)
		})
	}()
}

func (queue *JobQueue) Close() {
	queue.open = false
	close(queue.notify)
	queue.wg.Wait()
}

func (queue *JobQueue) notifyWorker() {
	if queue.open {
		select {
		case queue.notify <- nil:
		default:
		}
	}
}

func (queue *JobQueue) readWorker() {
	defer close(queue.Output)

	var buf []byte
	var job Job
	for {
		err := queue.db.Update(func(tx *bolt.Tx) error {
			bucket := tx.Bucket([]byte(queue.name))
			key := itob(queue.current)
			buf = bucket.Get(key)

			if buf == nil {
				return nil
			}

			err := json.Unmarshal(buf, &job)
			if err != nil {
				panic(err)
			}
			job.Status = Active

			buf, err = json.Marshal(job)
			if err != nil {
				panic(err)
			}
			return bucket.Put(key, buf)
		})
		if err != nil {
			panic(err)
		}
		if buf == nil {
			if _, ok := <-queue.notify; ok {
				continue
			} else {
				break
			}
		}

		queue.Output <- job
		queue.current++
	}
}

func itob(v int) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))
	return b
}
