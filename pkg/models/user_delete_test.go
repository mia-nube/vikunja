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

package models

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
	"time"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/files"
	"code.vikunja.io/api/pkg/notifications"
	"code.vikunja.io/api/pkg/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm"
)

// Added by mia·nube on 2026-11-03: the TestDeleteUserMiaNube* tests below pin
// the patched deletion semantics (see the notice in user_delete.go).

func insertDeletionTestUser(t *testing.T, s *xorm.Session, name string) *user.User {
	t.Helper()
	u := &user.User{Username: name, Email: name + "@example.com", Status: user.StatusActive}
	_, err := s.Insert(u)
	require.NoError(t, err)
	return u
}

func insertDeletionTestFile(t *testing.T, s *xorm.Session, owner *user.User, name string) *files.File {
	t.Helper()
	content := []byte("residue " + name)
	f, err := files.CreateWithSession(s, bytes.NewReader(content), name, uint64(len(content)), owner)
	require.NoError(t, err)
	return f
}

func assertStoredFileAbsent(t *testing.T, fileID int64) {
	t.Helper()
	db.AssertMissing(t, "files", map[string]interface{}{"id": fileID})
	_, err := files.FileStat(&files.File{ID: fileID})
	assert.True(t, os.IsNotExist(err), "stored bytes of file %d must be gone, stat error: %v", fileID, err)
}

// insertCustomisedSavedFilter creates a saved filter through the normal create
// path (which creates its views) and then customises every view: a renamed
// title, an extra bucket, a task in that bucket and a task position.
func insertCustomisedSavedFilter(t *testing.T, s *xorm.Session, owner *user.User, name string) (*SavedFilter, []*ProjectView) {
	t.Helper()
	sf := &SavedFilter{Title: "personal filter " + name, Filters: &TaskCollection{Filter: "done = false"}}
	require.NoError(t, sf.Create(s, owner))

	task := &Task{Title: "filtered task " + name, ProjectID: 1, CreatedByID: 1, Index: 9000 + owner.ID}
	_, err := s.Insert(task)
	require.NoError(t, err)

	views := []*ProjectView{}
	require.NoError(t, s.Where("project_id = ?", getProjectIDFromSavedFilterID(sf.ID)).Find(&views))
	require.NotEmpty(t, views, "creating a saved filter creates its views")
	for _, v := range views {
		_, err = s.ID(v.ID).Cols("title").Update(&ProjectView{Title: "customised " + name})
		require.NoError(t, err)
		bucket := &Bucket{Title: "custom bucket " + name, ProjectViewID: v.ID}
		_, err = s.Insert(bucket)
		require.NoError(t, err)
		_, err = s.Insert(&TaskBucket{TaskID: task.ID, BucketID: bucket.ID, ProjectViewID: v.ID})
		require.NoError(t, err)
		_, err = s.Insert(&TaskPosition{TaskID: task.ID, ProjectViewID: v.ID, Position: 42})
		require.NoError(t, err)
	}
	return sf, views
}

func TestDeleteUserMiaNubeSharedData(t *testing.T) {
	t.Run("deleting a mere member never moves ownership to them and keeps the control project", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		notifications.Fake()

		control := insertDeletionTestUser(t, s, "control-owner")
		member := insertDeletionTestUser(t, s, "deleted-member")
		project := &Project{Title: "control project", OwnerID: control.ID}
		_, err := s.Insert(project)
		require.NoError(t, err)
		task := &Task{Title: "control task", ProjectID: project.ID, CreatedByID: control.ID, Index: 1}
		_, err = s.Insert(task)
		require.NoError(t, err)
		_, err = s.Insert(&ProjectUser{ProjectID: project.ID, UserID: member.ID, Permission: PermissionRead})
		require.NoError(t, err)

		require.NoError(t, DeleteUser(s, member))
		require.NoError(t, s.Commit())

		db.AssertMissing(t, "users", map[string]interface{}{"id": member.ID})
		db.AssertExists(t, "projects", map[string]interface{}{"id": project.ID, "owner_id": control.ID}, false)
		db.AssertMissing(t, "projects", map[string]interface{}{"owner_id": member.ID})
		db.AssertExists(t, "tasks", map[string]interface{}{"id": task.ID, "created_by_id": control.ID}, false)
		db.AssertMissing(t, "users_projects", map[string]interface{}{"user_id": member.ID})
	})

	t.Run("an owned project others can reach refuses the deletion without reassigning it", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		notifications.Fake()

		owner := insertDeletionTestUser(t, s, "deleted-owner")
		other := insertDeletionTestUser(t, s, "other-member")
		project := &Project{Title: "shared owned project", OwnerID: owner.ID}
		_, err := s.Insert(project)
		require.NoError(t, err)
		_, err = s.Insert(&ProjectUser{ProjectID: project.ID, UserID: other.ID, Permission: PermissionWrite})
		require.NoError(t, err)
		require.NoError(t, s.Commit())

		s2 := db.NewSession()
		defer s2.Close()
		err = DeleteUser(s2, owner)
		require.Error(t, err, "an unresolved shared-owner constraint must report the deletion as incomplete")
		assert.Contains(t, err.Error(), "cannot be deleted without reassigning")
		require.NoError(t, s2.Rollback())

		db.AssertExists(t, "users", map[string]interface{}{"id": owner.ID}, false)
		db.AssertExists(t, "projects", map[string]interface{}{"id": project.ID, "owner_id": owner.ID}, false)
		db.AssertExists(t, "users_projects", map[string]interface{}{"project_id": project.ID, "user_id": other.ID, "permission": PermissionWrite}, false)
	})

	t.Run("the only admin of a team others remain in refuses the deletion", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		notifications.Fake()

		admin := insertDeletionTestUser(t, s, "deleted-team-admin")
		other := insertDeletionTestUser(t, s, "other-team-member")
		team := &Team{Name: "shared team", CreatedByID: admin.ID}
		_, err := s.Insert(team)
		require.NoError(t, err)
		_, err = s.Insert(&TeamMember{TeamID: team.ID, UserID: admin.ID, Admin: true})
		require.NoError(t, err)
		_, err = s.Insert(&TeamMember{TeamID: team.ID, UserID: other.ID, Admin: false})
		require.NoError(t, err)
		require.NoError(t, s.Commit())

		s2 := db.NewSession()
		defer s2.Close()
		err = DeleteUser(s2, admin)
		require.Error(t, err)
		require.NoError(t, s2.Rollback())

		db.AssertExists(t, "users", map[string]interface{}{"id": admin.ID}, false)
		db.AssertExists(t, "team_members", map[string]interface{}{"team_id": team.ID, "user_id": other.ID, "admin": false}, false)
	})

	t.Run("every residue category on kept data is absent after a successful deletion", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		files.InitTestFileFixtures(t)
		s := db.NewSession()
		defer s.Close()
		notifications.Fake()

		control := insertDeletionTestUser(t, s, "residue-control")
		gone := insertDeletionTestUser(t, s, "residue-deleted")

		project := &Project{Title: "control shared project", OwnerID: control.ID}
		_, err := s.Insert(project)
		require.NoError(t, err)
		task := &Task{Title: "control shared task", ProjectID: project.ID, CreatedByID: control.ID, Index: 1}
		_, err = s.Insert(task)
		require.NoError(t, err)
		_, err = s.Insert(&ProjectUser{ProjectID: project.ID, UserID: gone.ID, Permission: PermissionWrite})
		require.NoError(t, err)

		comment := &TaskComment{TaskID: task.ID, AuthorID: gone.ID, Comment: "personal comment"}
		_, err = s.Insert(comment)
		require.NoError(t, err)
		_, err = s.Insert(&Reaction{UserID: control.ID, EntityID: comment.ID, EntityKind: ReactionKindComment, Value: "x"})
		require.NoError(t, err)

		attachmentFile := insertDeletionTestFile(t, s, gone, "attachment.txt")
		attachment := &TaskAttachment{TaskID: task.ID, FileID: attachmentFile.ID, CreatedByID: gone.ID}
		_, err = s.Insert(attachment)
		require.NoError(t, err)
		_, err = s.ID(task.ID).Cols("cover_image_attachment_id").Update(&Task{CoverImageAttachmentID: attachment.ID})
		require.NoError(t, err)

		label := &Label{Title: "personal label", CreatedByID: gone.ID}
		_, err = s.Insert(label)
		require.NoError(t, err)
		_, err = s.Insert(&LabelTask{TaskID: task.ID, LabelID: label.ID})
		require.NoError(t, err)

		_, err = s.Insert(&LinkSharing{Hash: "residue-link-share-hash", ProjectID: project.ID, SharedByID: gone.ID})
		require.NoError(t, err)
		_, err = s.Insert(&Webhook{TargetURL: "https://example.com/hook", Events: []string{"task.created"}, ProjectID: project.ID, CreatedByID: gone.ID})
		require.NoError(t, err)

		soloTeam := &Team{Name: "solo team", CreatedByID: gone.ID}
		_, err = s.Insert(soloTeam)
		require.NoError(t, err)
		_, err = s.Insert(&TeamMember{TeamID: soloTeam.ID, UserID: gone.ID, Admin: true})
		require.NoError(t, err)
		keptTeam := &Team{Name: "kept team", CreatedByID: gone.ID}
		_, err = s.Insert(keptTeam)
		require.NoError(t, err)
		_, err = s.Insert(&TeamMember{TeamID: keptTeam.ID, UserID: gone.ID, Admin: true})
		require.NoError(t, err)
		_, err = s.Insert(&TeamMember{TeamID: keptTeam.ID, UserID: control.ID, Admin: true})
		require.NoError(t, err)

		_, err = s.Insert(&Session{ID: "00000000-0000-4000-8000-0000000000aa", UserID: gone.ID, TokenHash: "residue-session-token-hash"})
		require.NoError(t, err)
		_, err = s.Insert(&user.Token{UserID: gone.ID, Token: "residue-user-token", Kind: user.TokenPasswordReset})
		require.NoError(t, err)
		_, err = s.Insert(&user.TOTP{UserID: gone.ID, Secret: "residue-totp-secret"})
		require.NoError(t, err)
		_, err = s.Insert(&OAuthCode{UserID: gone.ID, Code: "residue-oauth-code", ExpiresAt: time.Now().Add(time.Hour), ClientID: "c", RedirectURI: "r", CodeChallenge: "x", CodeChallengeMethod: "S256"})
		require.NoError(t, err)
		_, err = s.Insert(&TimeEntry{UserID: gone.ID, TaskID: task.ID, StartTime: time.Now()})
		require.NoError(t, err)
		_, err = s.Insert(&TaskUnreadStatus{TaskID: task.ID, UserID: gone.ID})
		require.NoError(t, err)
		_, err = s.Insert(&notifications.DatabaseNotification{NotifiableID: gone.ID, Notification: json.RawMessage(`{}`), Name: "residue"})
		require.NoError(t, err)

		avatar := insertDeletionTestFile(t, s, gone, "avatar.png")
		export := insertDeletionTestFile(t, s, gone, "export.zip")
		_, err = s.ID(gone.ID).Cols("avatar_file_id", "export_file_id").Update(&user.User{AvatarFileID: avatar.ID, ExportFileID: export.ID})
		require.NoError(t, err)

		require.NoError(t, DeleteUser(s, gone))
		require.NoError(t, s.Commit())

		db.AssertMissing(t, "users", map[string]interface{}{"id": gone.ID})
		db.AssertMissing(t, "task_comments", map[string]interface{}{"author_id": gone.ID})
		db.AssertMissing(t, "reactions", map[string]interface{}{"entity_id": comment.ID, "entity_kind": ReactionKindComment})
		db.AssertMissing(t, "task_attachments", map[string]interface{}{"created_by_id": gone.ID})
		assertStoredFileAbsent(t, attachmentFile.ID)
		assertStoredFileAbsent(t, avatar.ID)
		assertStoredFileAbsent(t, export.ID)
		db.AssertMissing(t, "labels", map[string]interface{}{"created_by_id": gone.ID})
		db.AssertMissing(t, "label_tasks", map[string]interface{}{"label_id": label.ID})
		db.AssertMissing(t, "users_projects", map[string]interface{}{"user_id": gone.ID})
		db.AssertMissing(t, "link_shares", map[string]interface{}{"shared_by_id": gone.ID})
		db.AssertMissing(t, "webhooks", map[string]interface{}{"created_by_id": gone.ID})
		db.AssertMissing(t, "teams", map[string]interface{}{"id": soloTeam.ID})
		db.AssertMissing(t, "teams", map[string]interface{}{"created_by_id": gone.ID})
		db.AssertMissing(t, "team_members", map[string]interface{}{"user_id": gone.ID})
		db.AssertMissing(t, "sessions", map[string]interface{}{"user_id": gone.ID})
		db.AssertMissing(t, "user_tokens", map[string]interface{}{"user_id": gone.ID})
		db.AssertMissing(t, "totp", map[string]interface{}{"user_id": gone.ID})
		db.AssertMissing(t, "oauth_codes", map[string]interface{}{"user_id": gone.ID})
		db.AssertMissing(t, "time_entries", map[string]interface{}{"user_id": gone.ID})
		db.AssertMissing(t, "task_unread_statuses", map[string]interface{}{"user_id": gone.ID})
		db.AssertMissing(t, "notifications", map[string]interface{}{"notifiable_id": gone.ID})

		// Unrelated owner and shared content are untouched.
		db.AssertExists(t, "projects", map[string]interface{}{"id": project.ID, "owner_id": control.ID}, false)
		db.AssertExists(t, "tasks", map[string]interface{}{"id": task.ID, "cover_image_attachment_id": 0}, false)
		db.AssertExists(t, "teams", map[string]interface{}{"id": keptTeam.ID, "created_by_id": 0}, false)
		db.AssertExists(t, "team_members", map[string]interface{}{"team_id": keptTeam.ID, "user_id": control.ID, "admin": true}, false)
		db.AssertExists(t, "users", map[string]interface{}{"id": control.ID}, false)
	})

	t.Run("saved filters go with their views and what hangs off them; another user's filter stays", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		notifications.Fake()

		gone := insertDeletionTestUser(t, s, "filter-deleted")
		control := insertDeletionTestUser(t, s, "filter-control")

		goneFilter, goneViews := insertCustomisedSavedFilter(t, s, gone, "deleted")
		controlFilter, controlViews := insertCustomisedSavedFilter(t, s, control, "control")

		require.NoError(t, DeleteUser(s, gone))
		require.NoError(t, s.Commit())

		db.AssertMissing(t, "saved_filters", map[string]interface{}{"id": goneFilter.ID})
		db.AssertMissing(t, "project_views", map[string]interface{}{"project_id": getProjectIDFromSavedFilterID(goneFilter.ID)})
		for _, v := range goneViews {
			for _, table := range []string{"buckets", "task_buckets", "task_positions"} {
				db.AssertMissing(t, table, map[string]interface{}{"project_view_id": v.ID})
			}
		}

		db.AssertExists(t, "saved_filters", map[string]interface{}{"id": controlFilter.ID, "owner_id": control.ID}, false)
		for _, v := range controlViews {
			db.AssertExists(t, "project_views", map[string]interface{}{"id": v.ID, "title": "customised control"}, false)
			db.AssertExists(t, "buckets", map[string]interface{}{"project_view_id": v.ID, "title": "custom bucket control"}, false)
			db.AssertExists(t, "task_buckets", map[string]interface{}{"project_view_id": v.ID}, false)
			db.AssertExists(t, "task_positions", map[string]interface{}{"project_view_id": v.ID, "position": 42}, false)
		}
	})

	t.Run("a storage-delete error fails the deletion", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		files.InitTestFileFixtures(t)
		s := db.NewSession()
		defer s.Close()
		notifications.Fake()

		owner := insertDeletionTestUser(t, s, "storage-fault-owner")
		project := &Project{Title: "personal project", OwnerID: owner.ID}
		_, err := s.Insert(project)
		require.NoError(t, err)
		task := &Task{Title: "personal task", ProjectID: project.ID, CreatedByID: owner.ID, Index: 1}
		_, err = s.Insert(task)
		require.NoError(t, err)
		f := insertDeletionTestFile(t, s, owner, "personal.txt")
		_, err = s.Insert(&TaskAttachment{TaskID: task.ID, FileID: f.ID, CreatedByID: owner.ID})
		require.NoError(t, err)
		require.NoError(t, s.Commit())

		files.FailRemoveForTest(t, &os.PathError{Op: "remove", Path: "attachment", Err: os.ErrPermission})

		s2 := db.NewSession()
		defer s2.Close()
		err = DeleteUser(s2, owner)
		require.Error(t, err, "a failed storage delete must not be reported as an erasure")
		assert.ErrorIs(t, err, os.ErrPermission)
		require.NoError(t, s2.Rollback())

		db.AssertExists(t, "users", map[string]interface{}{"id": owner.ID}, false)
		db.AssertExists(t, "files", map[string]interface{}{"id": f.ID}, false)
	})
}

func TestDeleteUser(t *testing.T) {
	t.Run("normal", func(t *testing.T) {
		// Modified by mia·nube on 2026-11-03: user 6 owns projects 6–11 that are
		// shared with other users and teams. Upstream handed them to another
		// member; the patched deletion refuses and changes nothing instead.
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		notifications.Fake()

		u := &user.User{ID: 6}
		err := DeleteUser(s, u)

		require.Error(t, err)
		assert.True(t, IsErrUserOwnsSharedResources(err))
		require.NoError(t, s.Rollback())
		db.AssertExists(t, "users", map[string]interface{}{"id": u.ID}, false)
		db.AssertExists(t, "projects", map[string]interface{}{"id": 24, "owner_id": u.ID}, false)
		for _, projectID := range []int64{6, 7, 8, 9, 10, 11} {
			db.AssertExists(t, "projects", map[string]interface{}{"id": projectID, "owner_id": u.ID}, false)
		}
	})
	t.Run("user with no projects", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		notifications.Fake()

		u := &user.User{ID: 4}
		err := DeleteUser(s, u)

		require.NoError(t, err)
		// No assertions for deleted projects since that user doesn't have any
	})
	t.Run("user with a default project", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		notifications.Fake()

		u := &user.User{ID: 16}
		err := DeleteUser(s, u)

		require.NoError(t, err)
		require.NoError(t, s.Commit())
		db.AssertMissing(t, "users", map[string]interface{}{"id": u.ID})
		db.AssertMissing(t, "projects", map[string]interface{}{"id": 37}) // only user16 had access to this project, and it was their default
	})
	t.Run("disabled user", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		notifications.Fake()

		u := &user.User{ID: 17}
		err := DeleteUser(s, u)

		require.NoError(t, err)
		require.NoError(t, s.Commit())
		db.AssertMissing(t, "users", map[string]interface{}{"id": u.ID})
	})
	t.Run("disabled user with task attachment they created", func(t *testing.T) {
		// Regression test: the cascade calls TaskAttachment.Delete -> ReadOne,
		// which loads the attachment's creator via user.GetUserByID. If the
		// creator is the disabled user being deleted, that lookup must not
		// surface ErrAccountDisabled out of the cascade.
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		notifications.Fake()

		project := &Project{
			Title:   "disabled user project",
			OwnerID: 17,
		}
		_, err := s.Insert(project)
		require.NoError(t, err)

		task := &Task{
			Title:       "disabled user task",
			ProjectID:   project.ID,
			CreatedByID: 17,
			Index:       1,
		}
		_, err = s.Insert(task)
		require.NoError(t, err)

		_, err = s.Insert(&TaskAttachment{
			TaskID:      task.ID,
			FileID:      1,
			CreatedByID: 17,
		})
		require.NoError(t, err)

		err = DeleteUser(s, &user.User{ID: 17})
		require.NoError(t, err)
		require.NoError(t, s.Commit())
		db.AssertMissing(t, "users", map[string]interface{}{"id": 17})
		db.AssertMissing(t, "projects", map[string]interface{}{"id": project.ID})
		db.AssertMissing(t, "tasks", map[string]interface{}{"id": task.ID})
	})
	t.Run("cleans up task assignments and subscriptions", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		notifications.Fake()

		task := &Task{
			Title:       "user cleanup",
			ProjectID:   19,
			CreatedByID: 7,
			Index:       4,
		}
		_, err := s.Insert(task)
		require.NoError(t, err)

		_, err = s.Insert(&TaskAssginee{TaskID: task.ID, UserID: 4})
		require.NoError(t, err)

		_, err = s.Insert(&Subscription{EntityType: SubscriptionEntityTask, EntityID: task.ID, UserID: 4})
		require.NoError(t, err)

		_, err = s.Insert(&Subscription{EntityType: SubscriptionEntityProject, EntityID: 19, UserID: 4})
		require.NoError(t, err)

		_, err = s.Insert(&TeamMember{TeamID: 9, UserID: 4})
		require.NoError(t, err)

		err = DeleteUser(s, &user.User{ID: 4})
		require.NoError(t, err)

		require.NoError(t, s.Commit())
		db.AssertMissing(t, "task_assignees", map[string]interface{}{"user_id": 4})
		db.AssertMissing(t, "subscriptions", map[string]interface{}{"user_id": 4})
		db.AssertMissing(t, "team_members", map[string]interface{}{"user_id": 4})
	})
}
