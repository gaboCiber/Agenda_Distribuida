package consensus

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"

	"go.etcd.io/bbolt"
)

const (
	raftDBFile = "raft.db"
)

// RaftStateDB handles the persistence of Raft's state.
type RaftStateDB struct {
	db *bbolt.DB
}

// NewRaftStateDB creates or opens a database for storing Raft state.
func NewRaftStateDB(baseDir string) (*RaftStateDB, error) {
	dbPath := filepath.Join(baseDir, raftDBFile)
	if err := os.MkdirAll(baseDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create raft db dir: %w", err)
	}

	db, err := bbolt.Open(dbPath, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open raft db: %w", err)
	}

	// We need buckets to store our key-value pairs.
	// Let's create a 'raft_state' bucket for term/votedFor and a 'raft_log' bucket.
	err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("raft_state"))
		if err != nil {
			return fmt.Errorf("failed to create raft_state bucket: %w", err)
		}
		_, err = tx.CreateBucketIfNotExists([]byte("raft_log"))
		if err != nil {
			return fmt.Errorf("failed to create raft_log bucket: %w", err)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return &RaftStateDB{db: db}, nil
}

// Close closes the database.
func (rsd *RaftStateDB) Close() error {
	return rsd.db.Close()
}

// Encode serializes an object into a byte slice.
func encode(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Decode deserializes a byte slice into an object.
func decode(data []byte, v interface{}) error {
	return gob.NewDecoder(bytes.NewReader(data)).Decode(v)
}

// SaveState persists the Raft state (term, votedFor, and log).
func (rsd *RaftStateDB) SaveState(currentTerm int, votedFor string, log []LogEntry) error {
	return rsd.db.Update(func(tx *bbolt.Tx) error {
		// Save term and votedFor in the 'raft_state' bucket.
		stateBucket := tx.Bucket([]byte("raft_state"))

		termBytes, err := encode(currentTerm)
		if err != nil {
			return fmt.Errorf("failed to encode currentTerm: %w", err)
		}
		if err := stateBucket.Put([]byte("currentTerm"), termBytes); err != nil {
			return err
		}

		votedForBytes, err := encode(votedFor)
		if err != nil {
			return fmt.Errorf("failed to encode votedFor: %w", err)
		}
		if err := stateBucket.Put([]byte("votedFor"), votedForBytes); err != nil {
			return err
		}

		// Save the log in the 'raft_log' bucket.
		// For simplicity, we'll save the entire log for now.
		// A more optimized approach might save individual entries.
		logBucket := tx.Bucket([]byte("raft_log"))
		logBytes, err := encode(log)
		if err != nil {
			return fmt.Errorf("failed to encode log: %w", err)
		}
		if err := logBucket.Put([]byte("log"), logBytes); err != nil {
			return err
		}

		return nil
	})
}

// LoadState loads the Raft state from the database.
func (rsd *RaftStateDB) LoadState() (currentTerm int, votedFor string, log []LogEntry, err error) {
	err = rsd.db.View(func(tx *bbolt.Tx) error {
		// Load term and votedFor from the 'raft_state' bucket.
		stateBucket := tx.Bucket([]byte("raft_state"))

		termBytes := stateBucket.Get([]byte("currentTerm"))
		if termBytes != nil {
			if err := decode(termBytes, &currentTerm); err != nil {
				return err
			}
		}

		votedForBytes := stateBucket.Get([]byte("votedFor"))
		if votedForBytes != nil {
			if err := decode(votedForBytes, &votedFor); err != nil {
				return err
			}
		}

		// Load the log from the 'raft_log' bucket.
		logBucket := tx.Bucket([]byte("raft_log"))
		logBytes := logBucket.Get([]byte("log"))
		if logBytes != nil {
			if err := decode(logBytes, &log); err != nil {
				return err
			}
		}

		return nil
	})
	return
}
