// SPDX-FileCopyrightText: 2020 Pier Luigi Fiorini <pierluigi.fiorini@gmail.com>
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package receiver

import (
	"github.com/hashicorp/go-memdb"

	"github.com/lirios/ostree-upload/internal/common"
)

// QueueEntry represents an entry in the update queue
type QueueEntry struct {
	ID         string
	UpdateRefs map[string]common.RevisionPair
	Objects    []string
}

// Queue represents the update queue
type Queue struct {
	schema *memdb.DBSchema
	db     *memdb.MemDB
}

// QueueWalkFn is a function prototype for Walk()
type QueueWalkFn func(entry *QueueEntry) error

// NewQueue creates a new Queue object
func NewQueue() (*Queue, error) {
	schema := &memdb.DBSchema{
		Tables: map[string]*memdb.TableSchema{
			"entry": {
				Name: "entry",
				Indexes: map[string]*memdb.IndexSchema{
					"id": {
						Name:         "id",
						Unique:       true,
						AllowMissing: false,
						Indexer:      &memdb.StringFieldIndex{Field: "ID"},
					},
				},
			},
		},
	}

	db, err := memdb.NewMemDB(schema)
	if err != nil {
		return nil, err
	}

	return &Queue{schema, db}, nil
}

// AddEntry adds an entry to the queue
func (q *Queue) AddEntry(entry *QueueEntry) error {
	txn := q.db.Txn(true)
	if err := txn.Insert("entry", entry); err != nil {
		txn.Abort()
		return err
	}
	txn.Commit()
	return nil
}

// RemoveEntry removes the entry from the queue
func (q *Queue) RemoveEntry(entry *QueueEntry) error {
	txn := q.db.Txn(true)
	if err := txn.Delete("entry", entry); err != nil {
		txn.Abort()
		return err
	}
	txn.Commit()
	return nil
}

// GetEntry returns the entry corresponding to the specified ID
func (q *Queue) GetEntry(ID string) (*QueueEntry, error) {
	txn := q.db.Txn(false)
	defer txn.Abort()

	raw, err := txn.First("entry", "id", ID)
	if err != nil {
		return nil, err
	}

	if raw == nil {
		return nil, memdb.ErrNotFound
	}

	return raw.(*QueueEntry), nil
}

// Walk walks through the queue entries and execute walkFn for each of them
func (q *Queue) Walk(walkFn QueueWalkFn) error {
	txn := q.db.Txn(false)
	defer txn.Abort()

	it, err := txn.Get("entry", "id")
	if err != nil {
		return err
	}

	for object := it.Next(); object != nil; object = it.Next() {
		entry := object.(*QueueEntry)
		if err := walkFn(entry); err != nil {
			return err
		}
	}

	return nil
}
