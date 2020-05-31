// SPDX-FileCopyrightText: 2020 Pier Luigi Fiorini <pierluigi.fiorini@gmail.com>
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package receiver

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

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

	// Parse up to 10 MiB
	r.ParseMultipartForm(10 * 1024 * 1024)

	// Read form values
	//rev := r.FormValue("rev")
	objectName := r.FormValue("object_name")
	checksum := r.FormValue("checksum")

	// Get the file
	file, _, err := r.FormFile("file")
	if err != nil {
		logger.Errorf("Unable to read file field: %v", err)
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	defer file.Close()

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
	if _, err := io.Copy(objectFile, file); err != nil {
		logger.Errorf("Failed to write %s: %v", objectName, err)
		objectFile.Close()
		os.Remove(objectPath)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Close files immediately to avoid "too many open files" error
	file.Close()
	objectFile.Close()

	// Verify checksum
	dstChecksum, err := common.CalculateChecksum(objectPath)
	if err != nil {
		logger.Errorf("Unable to calculate checksum of %s: %v", objectPath, err)
		os.Remove(objectPath)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if checksum != dstChecksum {
		logger.Errorf("Invalid checksum for %s", objectName)
		os.Remove(objectPath)
		msg := fmt.Sprintf("Checksum %s instead of %s for %s", dstChecksum, checksum, objectName)
		http.Error(w, msg, http.StatusInternalServerError)
		return
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
