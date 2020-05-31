// SPDX-FileCopyrightText: 2020 Pier Luigi Fiorini <pierluigi.fiorini@gmail.com>
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"log"
	"os"

	"github.com/lirios/ostree-upload/internal/cmd"
)

func init() {
	// Set logger flags
	log.SetFlags(0)
}

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
