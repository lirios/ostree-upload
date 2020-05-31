// SPDX-FileCopyrightText: 2020 Pier Luigi Fiorini <pierluigi.fiorini@gmail.com>
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package receiver

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"

	"github.com/lirios/ostree-upload/internal/logger"
)

func receiverContext(appState *AppState) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), KeyQueue, appState.Queue)
			ctx = context.WithValue(ctx, KeyRepository, appState.Repo)
			next.ServeHTTP(w, r.WithContext(ctx))
		}
		return http.HandlerFunc(fn)
	}
}

func v1Router(appState *AppState) http.Handler {
	r := chi.NewRouter()

	r.Use(receiverContext(appState))
	r.Get("/info", InfoHandler)
	r.Post("/queue", CreateEntryHandler)
	r.Delete("/queue/{queueID}", DeleteEntryHandler)
	r.Get("/queue/{queueID}", ObjectsHandler)
	r.Put("/queue/{queueID}", UploadHandler)
	r.Get("/done/{queueID}", DoneHandler)

	return r
}

func router(appState *AppState) http.Handler {
	r := chi.NewRouter()

	// A good base middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5, "gzip"))

	// Set a timeout value on the request context (ctx), that will signal
	// through ctx.Done() that the request has timed out and further
	// processing should be stopped.
	r.Use(middleware.Timeout(60 * time.Second))

	// Protected routes
	r.Group(func(r chi.Router) {
		// Seek, verify and validate tokens
		r.Use(TokenVerifier(appState))

		// API
		r.Mount("/api/v1", v1Router(appState))
	})

	// Public routes
	r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{}"))
	})

	return r
}

// StartServer starts the server
func StartServer(address string, appState *AppState) error {
	logger.Actionf("Starting server on %v", address)
	return http.ListenAndServe(address, router(appState))
}
