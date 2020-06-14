// SPDX-FileCopyrightText: 2020 Pier Luigi Fiorini <pierluigi.fiorini@gmail.com>
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package receiver

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/chilts/sid"
	"github.com/go-chi/chi"

	"github.com/lirios/ostree-upload/internal/common"
	"github.com/lirios/ostree-upload/internal/logger"
	"github.com/lirios/ostree-upload/internal/ostree"
)

// InfoHandler returns repository mode and resolve all branches
func InfoHandler(w http.ResponseWriter, r *http.Request) {
	// Get from context
	ctx := r.Context()
	repo, ok := ctx.Value(KeyRepository).(*ostree.Repo)
	if !ok {
		logger.Error("Unable to retrieve repository object from context")
		http.Error(w, "no repository found", http.StatusUnprocessableEntity)
		return
	}

	// Decode request
	err := DecodeJSONBody(w, r, nil)
	if err != nil {
		HandleDecodeError(w, err)
		return
	}

	// Repository mode
	mode, err := repo.GetMode()
	if err != nil {
		logger.Errorf("Failed to get repository mode: %v", err)
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}

	// List server-side revisions
	refs, err := repo.ListRevisions()
	if err != nil {
		logger.Errorf("Failed to list revisions: %v", err)
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}

	object := common.InfoResponse{Mode: mode, Revs: refs}
	EncodeJSONReply(w, r, object)
}

// CreateEntryHandler creates a new queue entry ready for the upload
func CreateEntryHandler(w http.ResponseWriter, r *http.Request) {
	// Get from context
	ctx := r.Context()
	queue, ok := ctx.Value(KeyQueue).(*Queue)
	if !ok {
		logger.Error("Unable to retrieve queue object from context")
		http.Error(w, "no queue found", http.StatusUnprocessableEntity)
		return
	}

	// Decode request
	var req common.QueueRequest
	err := DecodeJSONBody(w, r, &req)
	if err != nil {
		HandleDecodeError(w, err)
		return
	}

	// Forbid an update of the same branches
	err = queue.Walk(func(entry *QueueEntry) error {
		for branch := range entry.UpdateRefs {
			if _, ok := req.Refs[branch]; ok {
				return fmt.Errorf("branch \"%s\" is already being updated", branch)
			}
		}

		return nil
	})
	if err != nil {
		logger.Errorf("Failed to walk the queue: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// New queue entry
	queueID := sid.IdBase64()
	queueEntry := &QueueEntry{ID: queueID, UpdateRefs: req.Refs, Objects: req.Objects}
	if err := queue.AddEntry(queueEntry); err != nil {
		logger.Errorf("Failed to add entry \"%s\" to the queue: %v", queueID, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	object := common.UpdateResponse{QueueID: queueID}
	EncodeJSONReply(w, r, object)
}

// DeleteEntryHandler deletes the entry from the queue
func DeleteEntryHandler(w http.ResponseWriter, r *http.Request) {
	// Get from context
	ctx := r.Context()
	queue, ok := ctx.Value(KeyQueue).(*Queue)
	if !ok {
		logger.Error("Unable to retrieve queue object from context")
		http.Error(w, "no queue found", http.StatusUnprocessableEntity)
		return
	}

	// Get the entry from the queue
	queueID := chi.URLParam(r, "queueID")
	entry, err := queue.GetEntry(queueID)
	if err != nil {
		logger.Errorf("Unable to retrieve queue entry: %v", err)
		http.Error(w, fmt.Sprintf("failed to get entry from queue: %v", err), http.StatusNotFound)
		return
	}
	if entry == nil {
		logger.Error("Unable to find queue entry")
		http.Error(w, "queue entry not found", http.StatusNotFound)
		return
	}

	// Delete
	if err := queue.RemoveEntry(entry); err != nil {
		logger.Errorf("Unable to remove entry from queue: %v")
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
}

// ObjectsHandler reads the complete list of missing objects passed by the client
// and returns the list of objects that were not previously upload
func ObjectsHandler(w http.ResponseWriter, r *http.Request) {
	// Get from context
	ctx := r.Context()
	queue, ok := ctx.Value(KeyQueue).(*Queue)
	if !ok {
		logger.Error("Unable to retrieve queue object from context")
		http.Error(w, "no queue found", http.StatusUnprocessableEntity)
		return
	}
	repo, ok := ctx.Value(KeyRepository).(*ostree.Repo)
	if !ok {
		logger.Error("Unable to retrieve repository object from context")
		http.Error(w, "no repository found", http.StatusUnprocessableEntity)
		return
	}

	// Get the entry from the queue
	queueID := chi.URLParam(r, "queueID")
	entry, err := queue.GetEntry(queueID)
	if err != nil {
		logger.Errorf("Unable to retrieve queue entry: %v", err)
		http.Error(w, fmt.Sprintf("failed to get entry from queue: %v", err), http.StatusNotFound)
		return
	}
	if entry == nil {
		logger.Error("Unable to find queue entry")
		http.Error(w, "queue entry not found", http.StatusNotFound)
		return
	}

	// Decode request
	err = DecodeJSONBody(w, r, nil)
	if err != nil {
		HandleDecodeError(w, err)
		return
	}

	// List of missing objects we will receive from the client
	missingObjects := []string{}
	for _, objectName := range entry.Objects {
		tempPath := GetTempObjectPath(repo, objectName)
		objectPath := repo.GetObjectPath(objectName)

		if _, err := os.Stat(tempPath); os.IsNotExist(err) {
			if _, err := os.Stat(objectPath); os.IsNotExist(err) {
				missingObjects = append(missingObjects, objectName)
			}
		}
	}

	// Reply
	object := common.ObjectsResponse{Objects: missingObjects}
	EncodeJSONReply(w, r, object)
}

// UploadHandler receives objects from the client
func UploadHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	// Get from context
	ctx := r.Context()
	queue, ok := ctx.Value(KeyQueue).(*Queue)
	if !ok {
		logger.Error("Unable to retrieve queue object from context")
		http.Error(w, "no queue found", http.StatusUnprocessableEntity)
		return
	}
	repo, ok := ctx.Value(KeyRepository).(*ostree.Repo)
	if !ok {
		logger.Error("Unable to retrieve repository object from context")
		http.Error(w, "no repository found", http.StatusUnprocessableEntity)
		return
	}

	// Get the entry from the queue
	queueID := chi.URLParam(r, "queueID")
	entry, err := queue.GetEntry(queueID)
	if err != nil {
		logger.Errorf("Unable to retrieve queue entry: %v", err)
		http.Error(w, fmt.Sprintf("failed to get entry from queue: %v", err), http.StatusNotFound)
		return
	}
	if entry == nil {
		logger.Error("Unable to find queue entry")
		http.Error(w, "queue entry not found", http.StatusNotFound)
		return
	}

	// Prevent from uploading when it's finalizing
	if entry.Finalizing {
		http.Error(w, "cannot upload objects when the update process is almost done", http.StatusUnprocessableEntity)
		return
	}

	var mr *multipart.Reader
	var part *multipart.Part

	if mr, err = r.MultipartReader(); err != nil {
		logger.Errorf("Multipart error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Save checksums here for later comparison
	checksums := map[string]string{}

	// Read all parts
	for {
		if part, err = mr.NextPart(); err != nil {
			if err == io.EOF {
				// Exit when we read all the parts
				w.WriteHeader(200)
			} else {
				logger.Errorf("Error reading part: %v", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		if part.FormName() == "file" {
			// Receive file
			objectName := part.FileName()
			logger.Debugf("Receiving \"%s\"...", objectName)

			// Create the destination file
			objectPath := GetTempObjectPath(repo, objectName)
			if _, err := os.Stat(objectPath); os.IsExist(err) {
				msg := fmt.Sprintf("temporary file for object \"%s\" already exist", objectName)
				logger.Errorf("Unable to complete upload: %s")
				http.Error(w, msg, http.StatusUnprocessableEntity)
				return
			}
			objectFile, err := os.Create(objectPath)
			if err != nil {
				logger.Errorf("Unable to create %s: %v", objectName, err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			defer objectFile.Close()

			// Write chunks and calculate checksum for a verification later
			if _, err = io.Copy(objectFile, part); err != nil {
				logger.Errorf("Failed to copy part to \"%s\": %v", objectName, err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			objectFile.Close()
			checksum, err := common.CalculateChecksum(objectPath)
			if err != nil {
				logger.Errorf("Failed to calculate checksum of \"%s\": %v", objectName, err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			checksums[objectName] = checksum
		} else if part.FormName() == "checksum" {
			// Read checksum calculate by the client
			value := &bytes.Buffer{}
			if _, err = io.Copy(value, part); err != nil {
				logger.Errorf("Failed to read checksum: %v", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			args := strings.Split(value.String(), ":")
			if len(args) != 2 {
				logger.Error("Failed to receive checksum: bad format")
				http.Error(w, "bad checksum format", http.StatusUnprocessableEntity)
				return
			}
			objectName := args[0]
			checksum := args[1]
			if objectName == "" || checksum == "" {
				logger.Error("Failed to receive checksum: empty object name or checksum")
				http.Error(w, "empty object name or checksum", http.StatusUnprocessableEntity)
				return
			}

			// If the checksum doesn't match we remove the object and report the error,
			// so that the next time the object will be uploaded again
			if checksums[objectName] != checksum {
				//os.Remove(GetTempObjectPath(repo, objectName))
				logger.Errorf("Object \"%s\" has a bad checksum (%s vs %s)", objectName, checksums[objectName], checksum)
				http.Error(w, fmt.Sprintf("bad checksum for %s", objectName), http.StatusUnprocessableEntity)
				return
			}
		} else {
			logger.Errorf("Received unsupported form field %s", part.FormName())
			http.Error(w, fmt.Sprintf("unsupported form field %s", part.FormName()), http.StatusUnprocessableEntity)
			return
		}
	}
}

// DoneHandler signals that the upload is complete
func DoneHandler(w http.ResponseWriter, r *http.Request) {
	// Get from context
	ctx := r.Context()
	queue, ok := ctx.Value(KeyQueue).(*Queue)
	if !ok {
		logger.Error("Unable to retrieve queue object from context")
		http.Error(w, "no queue found", http.StatusUnprocessableEntity)
		return
	}
	repo, ok := ctx.Value(KeyRepository).(*ostree.Repo)
	if !ok {
		logger.Error("Unable to retrieve repository object from context")
		http.Error(w, "no repository found", http.StatusUnprocessableEntity)
		return
	}

	// Get the entry from the queue
	queueID := chi.URLParam(r, "queueID")
	entry, err := queue.GetEntry(queueID)
	if err != nil {
		logger.Errorf("Unable to retrieve queue entry: %v", err)
		http.Error(w, fmt.Sprintf("failed to get entry from queue: %v", err), http.StatusNotFound)
		return
	}
	if entry == nil {
		logger.Error("Unable to find queue entry")
		http.Error(w, "queue entry not found", http.StatusNotFound)
		return
	}

	// Prevent from finalizing twice
	if entry.Finalizing {
		http.Error(w, "already finalizing", http.StatusInternalServerError)
		return
	}

	// Move all received objects
	entry.Finalizing = true
	logger.Infof("Queue %s: publishing %d objects", queueID, len(entry.Objects))
	for _, objectName := range entry.Objects {
		// Create path where the object will be moved to
		objectPath := repo.GetObjectPath(objectName)
		path := filepath.Dir(objectPath)
		if err := os.MkdirAll(path, 0755); err != nil {
			msg := fmt.Sprintf("failed to create directory \"%s\" for the objects: %v", path, err)
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}

		// Move from the temporary location to the proper path only if it wasn't previously moved
		if _, err := os.Stat(objectPath); os.IsNotExist(err) {
			tempPath := GetTempObjectPath(repo, objectName)
			if err := moveFile(tempPath, objectPath); err != nil {
				msg := fmt.Sprintf("unable to move \"%s\" to \"%s\": %v", tempPath, objectPath, err)
				http.Error(w, msg, http.StatusInternalServerError)
				return
			}
		}
	}

	// Update refs
	if err := UpdateRefs(repo, entry.UpdateRefs); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Remove entry
	if err := queue.RemoveEntry(entry); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
