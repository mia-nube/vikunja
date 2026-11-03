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

// Added by mia·nube on 2026-11-03: the link-attachment migration must add its
// columns and index without dropping the indexes task_attachments already has.

package migration

import (
	"testing"
	"time"

	"code.vikunja.io/api/pkg/db"

	"github.com/stretchr/testify/require"
	"xorm.io/xorm"
)

type taskAttachmentsBefore20260731120000 struct {
	ID          int64     `xorm:"bigint autoincr not null unique pk"`
	TaskID      int64     `xorm:"bigint not null index"`
	FileID      int64     `xorm:"bigint not null"`
	CreatedByID int64     `xorm:"bigint not null"`
	Created     time.Time `xorm:"created"`
}

func (taskAttachmentsBefore20260731120000) TableName() string {
	return "task_attachments"
}

func taskAttachmentIndexColumns20260731120000(t *testing.T, x *xorm.Engine) map[string]bool {
	t.Helper()
	tables, err := x.DBMetas()
	require.NoError(t, err)
	indexed := map[string]bool{}
	for _, table := range tables {
		if table.Name != "task_attachments" {
			continue
		}
		for _, index := range table.Indexes {
			for _, col := range index.Cols {
				indexed[col] = true
			}
		}
	}
	return indexed
}

func TestLinkAttachmentMigrationKeepsExistingIndexes20260731120000(t *testing.T) {
	x, err := db.CreateTestEngine()
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, x.DropTables(taskAttachmentsBefore20260731120000{}))
	})
	require.NoError(t, x.DropTables(taskAttachmentsBefore20260731120000{}))
	require.NoError(t, x.Sync2(taskAttachmentsBefore20260731120000{}))
	require.True(t, taskAttachmentIndexColumns20260731120000(t, x)["task_id"], "precondition: the task_id index exists")

	var migrate func(*xorm.Engine) error
	for _, m := range migrations {
		if m.ID == "20260731120000" {
			migrate = m.Migrate
		}
	}
	require.NotNil(t, migrate, "migration 20260731120000 is registered")
	require.NoError(t, migrate(x))

	indexed := taskAttachmentIndexColumns20260731120000(t, x)
	require.True(t, indexed["task_id"], "the existing task_id index must survive the link-attachment migration")
	require.True(t, indexed["link_provider"], "the migration adds the link_provider index")

	columns, err := x.DBMetas()
	require.NoError(t, err)
	for _, table := range columns {
		if table.Name == "task_attachments" {
			require.NotNil(t, table.GetColumn("link_ref"), "the migration adds link_ref")
		}
	}
}
