// SPDX-FileCopyrightText: 2020 Pier Luigi Fiorini <pierluigi.fiorini@gmail.com>
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package receiver

import (
	"io"
	"os"
)

func moveFile(source, destination string) error {
	src, err := os.Open(source)
	if err != nil {
		return err
	}
	defer src.Close()

	fi, err := src.Stat()
	if err != nil {
		return err
	}

	perm := fi.Mode() & os.ModePerm
	dst, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer dst.Close()

	if _, err = io.Copy(dst, src); err != nil {
		dst.Close()
		os.Remove(destination)
		return err
	}

	if err = dst.Close(); err != nil {
		return err
	}

	if err = src.Close(); err != nil {
		return err
	}

	if err = os.Remove(source); err != nil {
		return err
	}

	return nil
}
