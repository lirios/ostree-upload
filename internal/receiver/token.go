// SPDX-FileCopyrightText: 2020 Pier Luigi Fiorini <pierluigi.fiorini@gmail.com>
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package receiver

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strings"
	"time"
)

// Token represents an API token
type Token struct {
	Token   string `yaml:"token"`
	Created string `yaml:"created"`
}

// GenerateToken generates a new reandom API token
func GenerateToken() (*Token, error) {
	key := make([]byte, 64)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}

	tokenString := base64.StdEncoding.EncodeToString(key)

	return &Token{Token: tokenString, Created: time.Now().UTC().Format(time.RFC3339)}, nil
}

func tokenFromHeader(r *http.Request) string {
	bearer := r.Header.Get("Authorization")
	if len(bearer) > 7 && strings.ToUpper(bearer[0:6]) == "BEARER" {
		return bearer[7:]
	}
	return ""
}

// TokenVerifier HTTP middleware handler will verify token in a HTTP request
// Checks if the HTTP request has 'Authorization: BEARER T' header.
func TokenVerifier(appState *AppState) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			tokenString := tokenFromHeader(r)
			if tokenString == "" {
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}

			// Check if the token is valid
			found := false
			for _, token := range appState.Config.Tokens {
				if token.Token == tokenString {
					found = true
					break
				}
			}
			if !found {
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}
