// SPDX-FileCopyrightText: 2020 Pier Luigi Fiorini <pierluigi.fiorini@gmail.com>
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package receiver

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lirios/ostree-upload/internal/common"
	"github.com/lirios/ostree-upload/internal/ostree"
)

// ContextKey is a type that represent the key of a context
type ContextKey int

const (
	// KeyQueue is the context key for the update queue
	KeyQueue ContextKey = iota

	// KeyRepository is the context key for the ostree.Repo instance
	KeyRepository ContextKey = iota
)

// Name of the temporary directory inside the OSTree repository
const tempDirName = "tmp/ostree-upload"

// CreateTempDirectory creates a temporary directory inside the repository, used to store the objects during the upload
func CreateTempDirectory(r *ostree.Repo) error {
	tempPath := filepath.Join(r.Path(), tempDirName)

	// Check if the temporary directory already exist
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		return err
	}

	// Create temporary directory
	if err := os.Mkdir(tempPath, 0755); err != nil {
		return err
	}

	return nil
}

// GetTempObjectPath returns the path to the OSTree object passed as argument
// from the temporary directory
func GetTempObjectPath(r *ostree.Repo, objectName string) string {
	return filepath.Join(r.Path(), tempDirName, objectName)
}

// UpdateRefs points branches to the new checksum
func UpdateRefs(r *ostree.Repo, refs map[string]common.RevisionPair) error {
	for branch, revPair := range refs {
		if err := r.SetRefImmediate("", branch, revPair.Client); err != nil {
			return fmt.Errorf("Failed to set branch %s from %s to %s: %v", branch, revPair.Server, revPair.Client, err)
		}
	}

	return nil
}
