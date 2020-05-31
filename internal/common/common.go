// SPDX-FileCopyrightText: 2020 Pier Luigi Fiorini <pierluigi.fiorini@gmail.com>
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package common

// RevisionPair is a pair of revisions
type RevisionPair struct {
	Server string `json:"server"`
	Client string `json:"client"`
}

// Object represents an object that needs to be uploaded to the receiver
type Object struct {
	Rev        string `json:"rev"`
	ObjectName string `json:"object_name"`
	ObjectPath string `json:"object_path"`
	Checksum   string `json:"checksum"`
}

// Objects maps object names to objects
type Objects map[string]Object

// InfoResponse contains OSTree repository information
type InfoResponse struct {
	Mode string            `json:"mode"`
	Revs map[string]string `json:"revs"`
}

// QueueRequest contains local and remote branch revision
type QueueRequest struct {
	Refs    map[string]RevisionPair `json:"refs"`
	Objects []string                `json:"objects"`
}

// UpdateResponse contains the update queue identifier
type UpdateResponse struct {
	QueueID string `json:"id"`
}

// ObjectsResponse lists all missing objects
type ObjectsResponse struct {
	Objects []string `json:"objects"`
}
