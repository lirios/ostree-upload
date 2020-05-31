// SPDX-FileCopyrightText: 2020 Pier Luigi Fiorini <pierluigi.fiorini@gmail.com>
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package receiver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/golang/gddo/httputil/header"
	"github.com/lirios/ostree-upload/internal/logger"
)

// Based on this blog post: https://www.alexedwards.net/blog/how-to-properly-parse-a-json-request-body

// MalformedRequest represents a malformed request error and contains the
// HTTP status code and message
type MalformedRequest struct {
	Status  int
	Message string
}

func (mr *MalformedRequest) Error() string {
	return mr.Message
}

// HTTPError sends an HTTP error back to the client
func HTTPError(w http.ResponseWriter, code int) {
	http.Error(w, http.StatusText(code), code)
}

// DecodeJSONBody decodes the body and returns an error or nil if it succeeds
func DecodeJSONBody(w http.ResponseWriter, r *http.Request, dst interface{}) error {
	// If the Content-Type header is present, check that it has the value application/json
	if r.Header.Get("Content-Type") != "" {
		value, _ := header.ParseValueAndParams(r.Header, "Content-Type")
		if value != "application/json" {
			msg := "Content-Type header is not application/json"
			return &MalformedRequest{Status: http.StatusUnsupportedMediaType, Message: msg}
		}
	}

	// Enforce a maximum read from the response body: a body larger
	// than that will now result in Decode() returning a "http: request body too large" error
	r.Body = http.MaxBytesReader(w, r.Body, 10*1024*1024)
	defer r.Body.Close()

	// Decode the request and return an error for unknown fields
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	// Decode
	if dst != nil {
		err := dec.Decode(&dst)
		if err != nil {
			var syntaxError *json.SyntaxError
			var unmarshalTypeError *json.UnmarshalTypeError

			switch {
			case errors.As(err, &syntaxError):
				msg := fmt.Sprintf("Request body contains badly-formed JSON (at position %d)", syntaxError.Offset)
				return &MalformedRequest{Status: http.StatusBadRequest, Message: msg}

			case errors.Is(err, io.ErrUnexpectedEOF):
				msg := fmt.Sprintf("Request body contains badly-formed JSON")
				return &MalformedRequest{Status: http.StatusBadRequest, Message: msg}

			case errors.As(err, &unmarshalTypeError):
				msg := fmt.Sprintf("Request body contains an invalid value for the %q field (at position %d)", unmarshalTypeError.Field, unmarshalTypeError.Offset)
				return &MalformedRequest{Status: http.StatusBadRequest, Message: msg}

			case strings.HasPrefix(err.Error(), "json: unknown field "):
				fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
				msg := fmt.Sprintf("Request body contains unknown field %s", fieldName)
				return &MalformedRequest{Status: http.StatusBadRequest, Message: msg}

			case errors.Is(err, io.EOF):
				msg := "Request body must not be empty"
				return &MalformedRequest{Status: http.StatusBadRequest, Message: msg}

			case err.Error() == "http: request body too large":
				msg := "Request body must not be larger than 10 MiB"
				return &MalformedRequest{Status: http.StatusRequestEntityTooLarge, Message: msg}

			default:
				return err
			}
		}
	}

	// Call decode again, using a pointer to an empty anonymous struct as
	// the destination. If the request body only contained a single JSON
	// object this will return an io.EOF error. So if we get anything else,
	// we know that there is additional data in the request body.
	err := dec.Decode(&struct{}{})
	if err != io.EOF {
		msg := "Request body must only contain a single JSON object"
		return &MalformedRequest{Status: http.StatusBadRequest, Message: msg}
	}

	return nil
}

// EncodeJSONReply encodes a JSON reply
func EncodeJSONReply(w http.ResponseWriter, r *http.Request, object interface{}) {
	js, err := json.Marshal(object)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

// HandleDecodeError sends the error to the client
func HandleDecodeError(w http.ResponseWriter, err error) {
	var mr *MalformedRequest
	if errors.As(err, &mr) {
		http.Error(w, mr.Message, mr.Status)
	} else {
		logger.Error(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
