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

package v1

import (
	"errors"
	"net/http"
	"strconv"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/models"
	auth2 "code.vikunja.io/api/pkg/modules/auth"
	webfiles "code.vikunja.io/api/pkg/web/files"

	"github.com/labstack/echo/v5"
)

// UploadTaskAttachment handles everything needed for the upload of a task attachment
// @Summary Upload a task attachment
// @Description Upload a task attachment. You can pass multiple files with the files form param.
// @tags task
// @Accept mpfd
// @Produce json
// @Param id path int true "Task ID"
// @Param files formData string true "The file, as multipart form file. You can pass multiple."
// @Security JWTKeyAuth
// @Success 200 {object} models.Message "Attachments were uploaded successfully."
// @Failure 403 {object} models.Message "No access to the task."
// @Failure 404 {object} models.Message "The task does not exist."
// @Failure 500 {object} models.Message "Internal error"
// @Router /tasks/{id}/attachments [put]
func UploadTaskAttachment(c *echo.Context) error {

	var taskAttachment models.TaskAttachment
	if err := c.Bind(&taskAttachment); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "No task ID provided").Wrap(err)
	}

	auth, err := auth2.GetAuthFromClaims(c)
	if err != nil {
		return err
	}

	s := db.NewSession()
	defer s.Close()

	// Multipart form
	form, err := c.MultipartForm()
	if err != nil {
		_ = s.Rollback()
		if errors.Is(err, http.ErrNotMultipart) {
			return echo.NewHTTPError(http.StatusBadRequest, "No multipart form provided").Wrap(err)
		}
		return err
	}

	fileHeaders := form.File["files"]
	uploads := make([]*models.AttachmentToUpload, 0, len(fileHeaders))
	var openErrors []error
	for _, file := range fileHeaders {
		f, err := file.Open()
		if err != nil {
			openErrors = append(openErrors, err)
			continue
		}
		defer f.Close()
		uploads = append(uploads, &models.AttachmentToUpload{Reader: f, Filename: file.Filename, Size: uint64(file.Size)})
	}

	success, failures, err := models.UploadTaskAttachments(s, auth, taskAttachment.TaskID, uploads)
	if err != nil {
		_ = s.Rollback()
		return err
	}

	if err := s.Commit(); err != nil {
		_ = s.Rollback()
		return err
	}

	return c.JSON(http.StatusOK, webfiles.BuildUploadResult(success, append(openErrors, failures...)))
}

// Modified by mia·nube on 2026-08-03: added the create-link handler, and made the
// download handler redirect a link attachment to its provider instead of trying
// to serve bytes Vikunja does not have.

// CreateLinkTaskAttachment attaches a file held by an external system to a task
// @Summary Attach a linked file to a task
// @Description Attaches a reference to a file held by an external system, rather than uploading a copy of it. Requires write access to the task, exactly as uploading does. The provider must be configured on this instance.
// @tags task
// @Accept json
// @Produce json
// @Param id path int true "Task ID"
// @Param link body models.LinkToAttach true "The file reference to attach."
// @Security JWTKeyAuth
// @Success 201 {object} models.TaskAttachment "The created link attachment."
// @Failure 400 {object} models.Message "The reference or the provider is missing or unknown."
// @Failure 403 {object} models.Message "No write access to the task."
// @Failure 404 {object} models.Message "The task does not exist."
// @Failure 500 {object} models.Message "Internal error"
// @Router /tasks/{id}/attachments/link [put]
func CreateLinkTaskAttachment(c *echo.Context) error {

	taskID, err := strconv.ParseInt(c.Param("task"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "No task ID provided").Wrap(err)
	}

	// The body is bound into its own transport struct, never into the attachment
	// model: binding it into the model would let a client set columns the server
	// alone may decide — the row id, the timestamps, the creator.
	link := &models.LinkToAttach{}
	if err := c.Bind(link); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid link attachment").Wrap(err)
	}

	auth, err := auth2.GetAuthFromClaims(c)
	if err != nil {
		return err
	}

	s := db.NewSession()
	defer s.Close()

	created, err := models.CreateLinkTaskAttachment(s, auth, taskID, link)
	if err != nil {
		_ = s.Rollback()
		return err
	}

	if err := s.Commit(); err != nil {
		_ = s.Rollback()
		return err
	}

	return c.JSON(http.StatusCreated, created)
}

// GetTaskAttachment returns a task attachment to download for the user
// @Summary Get one attachment.
// @Description Get one attachment for download. **Returns json on error.**
// @tags task
// @Produce octet-stream
// @Param id path int true "Task ID"
// @Param attachmentID path int true "Attachment ID"
// @Param preview_size query string false "The size of the preview image. Can be sm = 100px, md = 200px, lg = 400px or xl = 800px. If provided, a preview image will be returned if the attachment is an image."
// @Security JWTKeyAuth
// @Success 200 {file} blob "The attachment file."
// @Failure 403 {object} models.Message "No access to this task."
// @Failure 404 {object} models.Message "The task does not exist."
// @Failure 500 {object} models.Message "Internal error"
// @Router /tasks/{id}/attachments/{attachmentID} [get]
func GetTaskAttachment(c *echo.Context) error {

	var taskAttachment models.TaskAttachment
	if err := c.Bind(&taskAttachment); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "No task ID provided").Wrap(err)
	}

	auth, err := auth2.GetAuthFromClaims(c)
	if err != nil {
		return err
	}

	s := db.NewSession()
	defer s.Close()

	previewSize := models.GetPreviewSizeFromString(c.QueryParam("preview_size"))
	attachment, preview, err := models.LoadTaskAttachmentForDownload(s, auth, taskAttachment.TaskID, taskAttachment.ID, previewSize)
	if err != nil {
		_ = s.Rollback()
		return err
	}

	if err := s.Commit(); err != nil {
		_ = s.Rollback()
		return err
	}

	// A link attachment holds no bytes. Vikunja does not fetch them on the user's
	// behalf either — it sends the user's browser to the provider, so the request
	// arrives there as that user and is answered under that user's permissions.
	// Vikunja therefore never needs, and never holds, a credential for the
	// provider.
	if attachment.IsLink() {
		redirectTo, err := webfiles.LinkAttachmentRedirect(attachment)
		if err != nil {
			return err
		}
		return c.Redirect(http.StatusFound, redirectTo)
	}

	webfiles.WriteAttachmentDownload(c.Response(), c.Request(), attachment, preview)
	return nil
}
