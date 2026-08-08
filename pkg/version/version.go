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

// Modified by mia·nube on 2026-08-08: adds Commit, the source revision this
// binary was built from, so the AGPL §13 source offer can name the exact
// revision a user is interacting with instead of restating it as prose.

package version

import "code.vikunja.io/api/pkg/swagger"

// This package holds the version info
// It is an own package to avoid import cycles

// Version sets the version to be printed to the user. Gets overwritten by "make release" or "make build" with last git commit or tag.
var Version = "dev"

// Commit is the full source revision this binary was built from. It is set at
// link time (-X code.vikunja.io/api/pkg/version.Commit=<sha>) by a build that
// knows which revision it is building, and is deliberately EMPTY otherwise.
//
// Empty is meaningful, not a placeholder: a build that cannot name its own
// revision must not claim one. Consumers therefore treat "" as "this build has
// no verifiable source revision" and omit the source offer rather than
// advertising a revision nobody could check.
var Commit = ""

func init() {
	// Additional swagger information
	swagger.SwaggerInfo.Version = Version
}
