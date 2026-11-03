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

// Added by mia·nube on 2026-11-03.

package files

import "testing"

// removeFaultStorage passes every operation to the wrapped backend except
// Remove, which fails with a fixed error without touching the stored bytes.
type removeFaultStorage struct {
	FileStorage
	err error
}

func (r *removeFaultStorage) Remove(string) error {
	return r.err
}

// FailRemoveForTest makes every storage Remove fail with removeErr for the rest
// of the test t, and restores the previous backend when t ends. It exists so
// tests outside this package can prove that a failed storage delete propagates
// to their caller instead of being reported as an erasure.
func FailRemoveForTest(t testing.TB, removeErr error) {
	t.Helper()
	previous := storage
	storage = &removeFaultStorage{FileStorage: previous, err: removeErr}
	t.Cleanup(func() { storage = previous })
}
