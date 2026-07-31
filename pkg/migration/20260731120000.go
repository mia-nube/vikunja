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
//
// Added by mia·nube on 2026-07-31: link attachments (see MIA-NUBE-PATCHES.md).

package migration

import (
	"fmt"

	"src.techknowlogick.com/xormigrate"
	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

type taskAttachment20260731120000 struct {
	LinkProvider string `xorm:"varchar(50) null index"`
	LinkRef      string `xorm:"text null"`
	LinkName     string `xorm:"text null"`
	LinkSize     int64  `xorm:"bigint null"`
	LinkMime     string `xorm:"varchar(255) null"`
}

func (taskAttachment20260731120000) TableName() string {
	return "task_attachments"
}

func init() {
	migrations = append(migrations, &xormigrate.Migration{
		ID:          "20260731120000",
		Description: "mia-nube: add link-attachment columns and make task_attachments.file_id nullable",
		Migrate: func(tx *xorm.Engine) error {
			if err := tx.Sync2(taskAttachment20260731120000{}); err != nil {
				return fmt.Errorf("could not add the link attachment columns: %w", err)
			}

			// A link attachment has no stored blob, so file_id must be allowed to be
			// null. Sync2 will not relax an existing NOT NULL, so do it explicitly.
			// Every existing row keeps its file_id; nothing is rewritten.
			var query string
			switch tx.Dialect().URI().DBType {
			case schemas.MYSQL:
				query = "ALTER TABLE task_attachments MODIFY file_id BIGINT NULL"
			case schemas.SQLITE:
				// SQLite cannot drop a NOT NULL constraint in place, and rebuilding the
				// table here would be a far larger, riskier change than the feature
				// warrants. Link attachments store 0 rather than NULL in file_id, which
				// satisfies NOT NULL and is never read for a link row (IsLink gates every
				// access). So there is nothing to do.
				return nil
			default:
				query = "ALTER TABLE task_attachments ALTER COLUMN file_id DROP NOT NULL"
			}

			if _, err := tx.Exec(query); err != nil {
				return fmt.Errorf("could not make task_attachments.file_id nullable: %w", err)
			}
			return nil
		},
		Rollback: func(_ *xorm.Engine) error {
			// Deliberately a no-op rather than a column drop. Rolling back would
			// destroy every link attachment in the table with no way to reconstruct
			// them, and the added columns are harmless to an older binary, which
			// simply ignores them.
			return nil
		},
	})
}
