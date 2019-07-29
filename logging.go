package main

import (
	"sync"
	"time"

	"github.com/boltdb/bolt"
)

type DbWriter struct {
	input chan string
	wg    sync.WaitGroup
}

func NewDbWriter(db *bolt.DB, delay time.Duration, logBucket, jobBucket string) (*DbWriter, error) {
	input := make(chan string, 64)
	writer := DbWriter{
		input: input,
	}

	err := db.Update(func(tx *bolt.Tx) error {
		lb, err := tx.CreateBucketIfNotExists([]byte(logBucket))
		if err != nil {
			return err
		}
		_, err = lb.CreateBucketIfNotExists([]byte(jobBucket))
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	go dbWorker(db, delay, logBucket, jobBucket, input)
	return &writer, nil
}

func (writer *DbWriter) Write(data []byte) (int, error) {
	writer.input <- string(data[:])
	return len(data), nil
}

func (writer *DbWriter) Close() {
	close(writer.input)
	writer.wg.Wait()
}

func dbWorker(db *bolt.DB, delay time.Duration, logBucket, jobBucket string, input <-chan string) {

	buffer := make([]string, 64)
	open := true
	ticker := time.NewTicker(delay)
	defer ticker.Stop()

	for open {
		reading := true
		for reading {
			select {
			case msg, ok := <-input:
				if !ok {
					open = false
					reading = false
				} else {
					buffer = append(buffer, msg)
				}
			case _ = <-ticker.C:
				reading = false
			}
		}
		err := db.Batch(func(tx *bolt.Tx) error {
			lb := tx.Bucket([]byte(logBucket))
			jb := lb.Bucket([]byte(jobBucket))

			bid, _ := jb.NextSequence()
			id := int(bid)

			for i, msg := range buffer {
				err := jb.Put(itob(id+i), []byte(msg))
				if err != nil {
					return err
				}
			}

			return nil
		})
		if err != nil {
			panic(err)
		}
	}
}
