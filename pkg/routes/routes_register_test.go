// Vikunja is a to-do list application to facilitate your life.
// Copyright 2018-present Vikunja and contributors. All rights reserved.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

// Added by mia·nube on 2026-08-03.

package routes

import (
	"testing"

	"code.vikunja.io/api/pkg/config"
	apiv2 "code.vikunja.io/api/pkg/routes/api/v2"

	"github.com/labstack/echo/v5"
)

// TestV2SchemaRegistryHasNoDuplicateTypeNames builds the v2 OpenAPI schema
// registry, which is the only thing that detects a schema-name conflict.
//
// It exists because that conflict is invisible to every other check. `go build`,
// `go vet` and even running the binary's `version` command are all clean; the
// failure appears only when the router is assembled — i.e. at startup, as a crash
// loop, after a successful deploy.
//
// The concrete case: huma keys its registry by a type's BARE NAME, ignoring the
// package path, so a second exported `Provider` reaching the API surface panicked
// with
//
//	duplicate name: Provider, new type: linkattachments.Provider,
//	                existing type: openid.Provider
//
// Deliberately scoped to the v2 registration rather than the whole of
// RegisterRoutes: the full router needs a database and a keyvalue store, and a
// test that panics for want of those would go red for reasons unrelated to what
// it claims to check — which is indistinguishable from the bug it exists to
// catch. This needs neither.
func TestV2SchemaRegistryHasNoDuplicateTypeNames(t *testing.T) {
	config.InitDefaultConfig()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("registering the v2 API panicked, so the server cannot start: %v", r)
		}
	}()

	e := echo.New()
	apiv2.RegisterAll(apiv2.NewAPI(e, e.Group("/api/v2")))
}
