// Copyright 2026 Quest Financial Technologies S.à r.l.-S., Luxembourg
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.


// Package static provides embedded static assets (favicons, touch icons) and
// registers HTTP handlers to serve them.
package static

import (
	"embed"
	"net/http"
)

//go:embed favicon.png apple-touch-icon.png
var assets embed.FS

// RegisterHandlers adds favicon and apple-touch-icon routes to the given mux.
func RegisterHandlers(mux *http.ServeMux) {
	serve := func(name, contentType string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			data, err := assets.ReadFile(name)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", contentType)
			w.Header().Set("Cache-Control", "public, max-age=86400")
			_, _ = w.Write(data)
		}
	}

	mux.HandleFunc("/favicon.png", serve("favicon.png", "image/png"))
	mux.HandleFunc("/favicon.ico", serve("favicon.png", "image/png"))
	mux.HandleFunc("/apple-touch-icon.png", serve("apple-touch-icon.png", "image/png"))
	mux.HandleFunc("/apple-touch-icon-precomposed.png", serve("apple-touch-icon.png", "image/png"))
}
