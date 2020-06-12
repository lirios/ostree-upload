// SPDX-FileCopyrightText: 2020 Pier Luigi Fiorini <pierluigi.fiorini@gmail.com>
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package push

import (
	"fmt"
	"time"

	"github.com/lirios/ostree-upload/internal/logger"
)

// StartClient starts the client
func StartClient(url, token, path string, refs []string, prune bool) error {
	// Pusher
	pusher, err := NewPusher(path, refs)
	if err != nil {
		return err
	}

	// Client
	client, err := NewClient(url, token)
	if err != nil {
		return err
	}

	// Repository information
	logger.Action("Receiving repository information...")
	info, err := client.GetInfo()
	if err != nil {
		return fmt.Errorf("Failed to retrieve repository information: %v", err)
	}

	// See if there's something to update
	logger.Action("Looking for branches to update...")
	updateRefs, err := pusher.CheckUpdate(info.Revs)
	if err != nil {
		return fmt.Errorf("Failed to determine the branches to update: %v", err)
	}
	if len(updateRefs) == 0 {
		logger.Info("Nothing to update!")
		return nil
	}

	// Update branches
	logger.Action("About to update the following branches:")
	for branch, revPair := range updateRefs {
		if revPair.Server == "" {
			logger.Infof("\tNew branch \"%s\"\n\t\t  to: %s", branch, revPair.Client)
		} else {
			logger.Infof("\tBranch \"%s\"\n\t\tfrom: %s\n\t\t  to: %s", branch, revPair.Server, revPair.Client)
		}
	}

	if prune {
		// Prune the repository before sending any object
		logger.Action("Pruning repository (this might take a while)...")
		if err = pusher.Prune(); err != nil {
			return fmt.Errorf("Failed to prune repository: %v", err)
		}
	}

	// Collect commits and objects to upload
	objects, err := pusher.FindObjectsToPush(updateRefs)
	if err != nil {
		return fmt.Errorf("Failed to enumerate objects to upload: %v", err)
	}

	// Now extract the list object names
	objectNames := []string{}
	for objectName := range objects {
		objectNames = append(objectNames, objectName)
	}

	// Start the process
	queueID, err := client.NewQueueEntry(updateRefs, objectNames)
	if err != nil {
		return fmt.Errorf("Failed to check which branches need to be updated: %v", err)
	}

	// Check which objects we still need to upload
	wantedObjectNames, err := client.SendObjectsList(queueID)
	if err != nil {
		client.DeleteQueueEntry(queueID)
		return fmt.Errorf("Failed to retrieve the list of objects to upload: %v", err)
	}

	// Send objects
	startTime := time.Now()
	uploadDoneChan, uploadErrChan := pusher.Upload(client, queueID, wantedObjectNames, objects)
	select {
	case <-uploadDoneChan:
		break
	case err := <-uploadErrChan:
		logger.Error(err)
		if err := client.DeleteQueueEntry(queueID); err != nil {
			logger.Errorf("Failed to delete entry \"%s\" from queue: %v", queueID, err)
		}
		return err
	}
	elapsedTime := time.Since(startTime)
	logger.Infof("Upload took %s", elapsedTime)

	// Update refs
	if err := client.Done(queueID); err != nil {
		logger.Errorf("Failed to update refs: %v", err)
		if err := client.DeleteQueueEntry(queueID); err != nil {
			logger.Errorf("Failed to delete entry \"%s\" from queue: %v", queueID, err)
		}
	}

	logger.Info("Done!")

	return nil
}
