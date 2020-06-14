// SPDX-FileCopyrightText: 2020 Pier Luigi Fiorini <pierluigi.fiorini@gmail.com>
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package push

import (
	"fmt"
	"os"

	"github.com/lirios/ostree-upload/internal/common"
	"github.com/lirios/ostree-upload/internal/logger"
	"github.com/lirios/ostree-upload/internal/ostree"
)

// Pusher allows you to push missing objects to an OSTree repository
type Pusher struct {
	repo     *ostree.Repo
	branches map[string]string
}

// NewPusher creates a new Pusher object
func NewPusher(repoPath string, refs []string) (*Pusher, error) {
	// Check if the repository path exist
	repo, err := ostree.OpenRepo(repoPath)
	if err != nil {
		return nil, err
	}

	// Enumerate branches to push
	branches := map[string]string{}
	if len(refs) == 0 {
		revisions, err := repo.ListRevisions()
		if err != nil {
			return nil, err
		}

		for branch, rev := range revisions {
			branches[branch] = rev
		}
	} else {
		for _, ref := range refs {
			rev, err := repo.ResolveRev(ref)
			if err != nil {
				return nil, err
			}

			branches[ref] = rev
		}
	}

	return &Pusher{repo, branches}, nil
}

// FindNeededCommits finds the commits of the local repository that the remove one doesn't have
func (p *Pusher) FindNeededCommits(remoteRev, localRev string) ([]string, error) {
	commits := []string{}

	parent := localRev

	for parent != remoteRev {
		logger.Debugf("Parent commit %s", parent)
		commits = append(commits, parent)

		newParent, err := p.repo.GetParentRev(parent)
		if err != nil {
			return nil, err
		}

		if newParent == "" {
			break
		} else {
			parent = newParent
		}
	}

	if remoteRev != "" && parent != remoteRev {
		return nil, fmt.Errorf("remote commit %v not descendent of commit %v", remoteRev, localRev)
	}

	return commits, nil
}

// FindObjectsForCommits finds the objects corresponding to the revisions that needs to be pushed to the receiver
func (p *Pusher) FindObjectsForCommits(revs []string) (common.Objects, error) {
	objects := make(common.Objects, 1024)

	for _, rev := range revs {
		revObjects, err := p.repo.TraverseCommit(rev, 0)
		if err != nil {
			return nil, err
		}

		for _, objectName := range revObjects {
			path := p.repo.GetObjectPath(objectName)
			if _, err := os.Stat(path); err != nil {
				return nil, err
			}

			checksum, err := common.CalculateChecksum(path)
			if err != nil {
				return nil, err
			}

			object := common.Object{Rev: rev, ObjectName: objectName, ObjectPath: path, Checksum: checksum}
			objects[objectName] = object
		}

	}

	return objects, nil
}

// CheckUpdate returns a map whose key is a branch and the value contains the corresponding
// revision in the remote and local repositories
func (p *Pusher) CheckUpdate(remoteRefs map[string]string) (map[string]common.RevisionPair, error) {
	updateRefs := make(map[string]common.RevisionPair)

	for branch, rev := range p.branches {
		remoteRev := remoteRefs[branch]
		if rev != remoteRev {
			updateRefs[branch] = common.RevisionPair{Server: remoteRev, Client: rev}
		}
	}

	return updateRefs, nil
}

// Prune prunes the repository
func (p *Pusher) Prune() error {
	total, pruned, size, err := p.repo.Prune(false, false)
	if err != nil {
		return err
	}

	logger.Infof("Pruned %d/%d objects, %d bytes deleted", pruned, total, size)

	return nil
}

// FindObjectsToPush finds which objects need to be pushed
func (p *Pusher) FindObjectsToPush(updateRefs map[string]common.RevisionPair) (common.Objects, error) {
	var commits []string

	for branch, revs := range updateRefs {
		logger.Actionf("Finding commits on branch \"%s\"...", branch)
		neededCommits, err := p.FindNeededCommits(revs.Server, revs.Client)
		if err != nil {
			return nil, err
		}
		commits = append(commits, neededCommits...)
	}

	logger.Action("Enumerating objects to send (this might take a while)...")
	neededObjects, err := p.FindObjectsForCommits(commits)
	if err != nil {
		return nil, err
	}

	return neededObjects, nil
}
