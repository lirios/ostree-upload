// SPDX-FileCopyrightText: 2020 Pier Luigi Fiorini <pierluigi.fiorini@gmail.com>
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package receiver

import "github.com/lirios/ostree-upload/internal/ostree"

// AppState represents the ostree-receiver context
type AppState struct {
	Queue  *Queue
	Repo   *ostree.Repo
	Config *Config
}
