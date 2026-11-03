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
	"errors"
	"fmt"
	"net/http"
	"time"

	"code.vikunja.io/api/pkg/cron"
	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/files"
	"code.vikunja.io/api/pkg/log"
	"code.vikunja.io/api/pkg/notifications"
	"code.vikunja.io/api/pkg/user"
	"code.vikunja.io/api/pkg/web"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// Modified by mia·nube on 2026-11-03: deleting a user no longer changes who
// owns or administers anything that belongs to someone else. Upstream walked
// every project the user could *access* and "ensured an admin" on it, which on
// a project the user was merely a member of could grant the deleted user admin
// and write that user into the project's owner_id, and on the user's own shared
// projects silently handed ownership to another member. Now only projects the
// user owns are deleted; owned projects that others can reach, and teams the
// user is the sole admin of while others remain, refuse the deletion with
// ErrUserOwnsSharedResources (nothing is reassigned). The user's residue on
// data that is kept — sessions and auth tokens, comments, attachments and their
// stored files, project memberships, link shares, created teams, labels,
// webhooks, time entries, notifications, avatar and export files, saved filters
// with their views, buckets and task positions — is removed,
// and a failed storage delete fails the deletion instead of being logged.

// User deletion must happen here in this packaage because we want to delete everything associated to this user.
// Because most of these things are managed in the models package, using them has to happen here.

// RegisterUserDeletionCron registers the cron job that actually removes users who are scheduled to delete.
func RegisterUserDeletionCron() {
	err := cron.Schedule("0 * * * *", deleteUsers)
	if err != nil {
		log.Errorf("Could not register deletion cron: %s", err.Error())
	}
}

func deleteUsers() {
	s := db.NewSession()
	users := []*user.User{}
	err := s.Where(builder.Lt{"deletion_scheduled_at": time.Now()}).
		Find(&users)
	s.Close()
	if err != nil {
		log.Errorf("Could not get users scheduled for deletion: %s", err)
		return
	}

	if len(users) == 0 {
		return
	}

	log.Debugf("Found %d users scheduled for deletion", len(users))

	now := time.Now()

	for _, u := range users {
		if !u.DeletionScheduledAt.Before(now) {
			log.Debugf("User %d is not yet scheduled for deletion. Scheduled at %s, now is %s", u.ID, u.DeletionScheduledAt, now)
			continue
		}

		func() {
			us := db.NewSession()
			defer us.Close()

			err = DeleteUser(us, u)
			if err != nil {
				_ = us.Rollback()
				log.Errorf("Could not delete u %d: %s", u.ID, err)
				return
			}

			log.Debugf("Deleted user %d", u.ID)

			err = us.Commit()
			if err != nil {
				log.Errorf("Could not commit transaction: %s", err)
			}
		}()
	}
}

// ErrUserOwnsSharedResources is returned when deleting a user would have to
// delete or reassign something other users still rely on. Nothing has been
// changed for this user when it is returned; the caller rolls back.
type ErrUserOwnsSharedResources struct {
	UserID int64
	// Projects owned by the user that other users or teams can reach.
	ProjectIDs []int64
	// Teams with other members in which the user is the only admin.
	TeamIDs []int64
}

// IsErrUserOwnsSharedResources checks if an error is ErrUserOwnsSharedResources.
func IsErrUserOwnsSharedResources(err error) bool {
	var target *ErrUserOwnsSharedResources
	return errors.As(err, &target)
}

func (err *ErrUserOwnsSharedResources) Error() string {
	return fmt.Sprintf("user %d cannot be deleted without reassigning data others rely on: shared owned projects %v, teams where they are the only admin %v", err.UserID, err.ProjectIDs, err.TeamIDs)
}

// ErrCodeUserOwnsSharedResources holds the unique world-error code of this error.
// mia·nube patch range, chosen far from upstream's 19xxx codes.
const ErrCodeUserOwnsSharedResources = 19901

// HTTPError holds the http error description
func (err *ErrUserOwnsSharedResources) HTTPError() web.HTTPError {
	return web.HTTPError{
		HTTPCode: http.StatusPreconditionFailed,
		Code:     ErrCodeUserOwnsSharedResources,
		Message:  "This user owns projects or administers teams that other users still use. Transfer or unshare them before deleting the user.",
	}
}

func countWhere(s *xorm.Session, bean any, query string, args ...any) (int64, error) {
	return s.Where(query, args...).Count(bean)
}

// ownedProjectIsShared reports whether deleting p (and with it its child
// projects) would remove something another user can reach or owns.
func ownedProjectIsShared(s *xorm.Session, p *Project, u *user.User) (bool, error) {
	checks := []struct {
		bean  any
		query string
		args  []any
	}{
		{&ProjectUser{}, "project_id = ? AND user_id != ?", []any{p.ID, u.ID}},
		{&TeamProject{}, "project_id = ?", []any{p.ID}},
		{&Project{}, "parent_project_id = ? AND owner_id != ?", []any{p.ID, u.ID}},
		{&user.User{}, "default_project_id = ? AND id != ?", []any{p.ID, u.ID}},
	}
	for _, c := range checks {
		n, err := countWhere(s, c.bean, c.query, c.args...)
		if err != nil {
			return false, err
		}
		if n > 0 {
			return true, nil
		}
	}

	// A project inside another user's project is reachable by everyone who
	// can reach that parent.
	if parentID := p.parentID(); parentID != 0 {
		n, err := countWhere(s, &Project{}, "id = ? AND owner_id = ?", parentID, u.ID)
		if err != nil {
			return false, err
		}
		if n == 0 {
			return true, nil
		}
	}

	return false, nil
}

// userDeletionPlan is what DeleteUser will remove, worked out before anything
// is changed so a refusal leaves the database untouched.
type userDeletionPlan struct {
	topLevelProjects []*Project // owned projects whose parent is not also deleted
	emptyTeams       []int64    // teams created by the user that keep no other member
	creatorTeams     []int64    // teams created by the user that keep other members
}

func planUserDeletion(s *xorm.Session, u *user.User) (plan *userDeletionPlan, err error) {
	plan = &userDeletionPlan{}
	constraint := &ErrUserOwnsSharedResources{UserID: u.ID}

	owned := []*Project{}
	if err = s.Where("owner_id = ?", u.ID).OrderBy("id asc").Find(&owned); err != nil {
		return nil, err
	}
	ownedIDs := make(map[int64]bool, len(owned))
	for _, p := range owned {
		ownedIDs[p.ID] = true
	}
	for _, p := range owned {
		shared, err := ownedProjectIsShared(s, p, u)
		if err != nil {
			return nil, err
		}
		if shared {
			constraint.ProjectIDs = append(constraint.ProjectIDs, p.ID)
			continue
		}
		if !ownedIDs[p.parentID()] {
			// Child projects are deleted by their parent's Delete.
			plan.topLevelProjects = append(plan.topLevelProjects, p)
		}
	}

	memberships := []*TeamMember{}
	if err = s.Where("user_id = ?", u.ID).Find(&memberships); err != nil {
		return nil, err
	}
	for _, m := range memberships {
		if !m.Admin {
			continue
		}
		others, err := countWhere(s, &TeamMember{}, "team_id = ? AND user_id != ?", m.TeamID, u.ID)
		if err != nil {
			return nil, err
		}
		otherAdmins, err := countWhere(s, &TeamMember{}, "team_id = ? AND user_id != ? AND admin = ?", m.TeamID, u.ID, true)
		if err != nil {
			return nil, err
		}
		if others > 0 && otherAdmins == 0 {
			constraint.TeamIDs = append(constraint.TeamIDs, m.TeamID)
		}
	}

	if len(constraint.ProjectIDs) > 0 || len(constraint.TeamIDs) > 0 {
		return nil, constraint
	}

	created := []*Team{}
	if err = s.Where("created_by_id = ?", u.ID).Find(&created); err != nil {
		return nil, err
	}
	for _, t := range created {
		others, err := countWhere(s, &TeamMember{}, "team_id = ? AND user_id != ?", t.ID, u.ID)
		if err != nil {
			return nil, err
		}
		if others == 0 {
			plan.emptyTeams = append(plan.emptyTeams, t.ID)
		} else {
			plan.creatorTeams = append(plan.creatorTeams, t.ID)
		}
	}

	return plan, nil
}

// deleteStoredFile removes a file row and its stored bytes. A file that is
// already gone counts as deleted; any other failure is returned.
func deleteStoredFile(s *xorm.Session, fileID int64) error {
	if fileID == 0 {
		return nil
	}
	err := (&files.File{ID: fileID}).Delete(s)
	if err != nil && !files.IsErrFileDoesNotExist(err) {
		return err
	}
	return nil
}

// deleteUserResidue removes what the user left on data that is kept after
// their own projects are gone.
func deleteUserResidue(s *xorm.Session, u *user.User, plan *userDeletionPlan) (err error) {
	// Comments on kept tasks, and the reactions on those comments.
	commentIDs := []int64{}
	if err = s.Table("task_comments").Where("author_id = ?", u.ID).Cols("id").Find(&commentIDs); err != nil {
		return err
	}
	if len(commentIDs) > 0 {
		if _, err = s.In("entity_id", commentIDs).And("entity_kind = ?", ReactionKindComment).Delete(&Reaction{}); err != nil {
			return err
		}
		if _, err = s.In("id", commentIDs).Delete(&TaskComment{}); err != nil {
			return err
		}
	}

	// Attachments on kept tasks, with their stored files.
	attachments := []*TaskAttachment{}
	if err = s.Where("created_by_id = ?", u.ID).Find(&attachments); err != nil {
		return err
	}
	for _, a := range attachments {
		if !a.IsLink() {
			if err = deleteStoredFile(s, a.FileID); err != nil {
				return err
			}
		}
		if _, err = s.Where("cover_image_attachment_id = ?", a.ID).
			Cols("cover_image_attachment_id").
			NoAutoTime().
			Update(&Task{CoverImageAttachmentID: 0}); err != nil {
			return err
		}
		if _, err = s.ID(a.ID).Delete(&TaskAttachment{}); err != nil {
			return err
		}
	}

	// Saved filters, with their views and everything hanging off those views.
	filterIDs := []int64{}
	if err = s.Table("saved_filters").Where("owner_id = ?", u.ID).Cols("id").Find(&filterIDs); err != nil {
		return err
	}
	if err = deleteSavedFilters(s, filterIDs); err != nil {
		return err
	}

	// Labels the user created, and their use on kept tasks.
	labelIDs := []int64{}
	if err = s.Table("labels").Where("created_by_id = ?", u.ID).Cols("id").Find(&labelIDs); err != nil {
		return err
	}
	if len(labelIDs) > 0 {
		if _, err = s.In("label_id", labelIDs).Delete(&LabelTask{}); err != nil {
			return err
		}
		if _, err = s.In("id", labelIDs).Delete(&Label{}); err != nil {
			return err
		}
	}

	// Teams: a created team nobody else is in goes; a created team others are
	// still in stays, without a creator. Team admin rights live on the
	// membership rows, so clearing the creator transfers nothing.
	for _, teamID := range plan.emptyTeams {
		for _, bean := range []any{&TeamMember{}, &TeamProject{}} {
			if _, err = s.Where("team_id = ?", teamID).Delete(bean); err != nil {
				return err
			}
		}
		if _, err = s.ID(teamID).Delete(&Team{}); err != nil {
			return err
		}
	}
	if len(plan.creatorTeams) > 0 {
		if _, err = s.In("id", plan.creatorTeams).
			Cols("created_by_id").
			NoAutoTime().
			Update(&Team{CreatedByID: 0}); err != nil {
			return err
		}
	}

	// Memberships, shares, auth state and other rows that only point at the user.
	residueRows := []struct {
		cond  builder.Cond
		model any
	}{
		{builder.Eq{"user_id": u.ID}, &ProjectUser{}},
		{builder.Eq{"shared_by_id": u.ID}, &LinkSharing{}},
		{builder.Or(builder.Eq{"created_by_id": u.ID}, builder.Eq{"user_id": u.ID}), &Webhook{}},
		{builder.Eq{"user_id": u.ID}, &TimeEntry{}},
		{builder.Eq{"user_id": u.ID}, &TaskUnreadStatus{}},
		{builder.Eq{"user_id": u.ID}, &Session{}},
		{builder.Eq{"user_id": u.ID}, &OAuthCode{}},
		{builder.Eq{"user_id": u.ID}, &user.Token{}},
		{builder.Eq{"user_id": u.ID}, &user.TOTP{}},
		{builder.Eq{"notifiable_id": u.ID}, &notifications.DatabaseNotification{}},
	}
	for _, row := range residueRows {
		if _, err = s.Where(row.cond).Delete(row.model); err != nil {
			return err
		}
	}

	// Avatar and data-export files hang off the user row itself.
	full := &user.User{}
	has, err := s.Where("id = ?", u.ID).Get(full)
	if err != nil {
		return err
	}
	if has {
		if err = deleteStoredFile(s, full.AvatarFileID); err != nil {
			return err
		}
		if err = deleteStoredFile(s, full.ExportFileID); err != nil {
			return err
		}
	}

	return nil
}

// DeleteUser completely removes a user and all their associated projects and tasks.
// This action is irrevocable.
// Public to allow deletion from the CLI.
//
// It returns *ErrUserOwnsSharedResources, before changing anything for this
// user, when the deletion would have to remove or reassign data that other
// users rely on. Like any other error, the caller must roll back the session.
func DeleteUser(s *xorm.Session, u *user.User) (err error) {
	plan, err := planUserDeletion(s, u)
	if err != nil {
		return err
	}

	// Delete any bot users owned by this user first (cascades their data too).
	var ownedBots []*user.User
	if err = s.Where("bot_owner_id = ?", u.ID).Find(&ownedBots); err != nil {
		return err
	}
	for _, bot := range ownedBots {
		if err = DeleteUser(s, bot); err != nil {
			return err
		}
	}

	for _, p := range plan.topLevelProjects {
		// The user owns every project in the plan, so Delete may remove
		// their default project too.
		if err = p.Delete(s, u); err != nil {
			return err
		}
	}

	if err = deleteUserResidue(s, u, plan); err != nil {
		return err
	}

	// Delete all related entities
	relatedEntities := []struct {
		column string
		model  any
	}{
		{"user_id", &TaskAssginee{}},
		{"user_id", &Subscription{}},
		{"user_id", &TeamMember{}},
		{"user_id", &Reaction{}},
		{"user_id", &Favorite{}},
		{"owner_id", &APIToken{}},
	}

	for _, entity := range relatedEntities {
		_, err = s.Where(entity.column+" = ?", u.ID).Delete(entity.model)
		if err != nil {
			return err
		}
	}

	// Notify before deleting the user row, because ShouldNotify will try to
	// look up the user and fail if the row is already gone.
	err = notifications.Notify(u, &user.AccountDeletedNotification{
		User: u,
	}, s)
	if err != nil {
		return err
	}

	_, err = s.Where("id = ?", u.ID).Delete(&user.User{})
	return err
}
