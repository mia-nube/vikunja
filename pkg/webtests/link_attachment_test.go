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

// Added by mia·nube on 2026-11-03: end-to-end guards for link attachments
// (patches 2/3) and the /info fields patches 3/5 publish. Written on the
// upstream v2.7.1 merge, where upstream's split of the download and preview
// handlers made the automatic merge move the link redirect into the *preview*
// handler; these cases pin the redirect to the download routes of both API
// versions, and the link error codes to values upstream does not use.

package webtests

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/models"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testLinkProviderKey = "testdrive"
	testLinkResolveURL  = "https://drive.example.test/files/{ref}"
)

func configureTestLinkProvider(t *testing.T) {
	t.Helper()
	config.LinkAttachmentsProviders.Set(map[string]interface{}{
		testLinkProviderKey: map[string]interface{}{
			"name":       "Test Drive",
			"pickerurl":  "https://drive.example.test/pick",
			"resolveurl": testLinkResolveURL,
		},
	})
	t.Cleanup(func() { config.LinkAttachmentsProviders.Set(nil) })
}

func createTestLinkAttachment(t *testing.T, e *echo.Echo, token, ref string) int64 {
	t.Helper()
	rec := humaRequest(t, e, http.MethodPost, "/api/v2/tasks/1/attachments/link",
		`{"link_provider":"`+testLinkProviderKey+`","link_ref":"`+ref+`","link_name":"plan.pdf","link_size":42,"link_mime":"application/pdf"}`,
		token, "")
	require.Equal(t, http.StatusCreated, rec.Code, "body: %s", rec.Body.String())
	var created models.TaskAttachment
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	require.NotZero(t, created.ID)
	assert.Equal(t, testLinkProviderKey, created.LinkProvider)
	assert.Equal(t, "https://drive.example.test/files/"+ref, created.LinkURL)
	return created.ID
}

func TestLinkAttachments(t *testing.T) {
	t.Run("v2 download redirects a link attachment to its provider", func(t *testing.T) {
		e, err := setupTestEnv()
		require.NoError(t, err)
		configureTestLinkProvider(t)
		token := humaTokenFor(t, &testuser1)

		id := createTestLinkAttachment(t, e, token, "abc123")
		rec := humaRequest(t, e, http.MethodGet, "/api/v2/tasks/1/attachments/"+strconv.FormatInt(id, 10), "", token, "")
		assert.Equal(t, http.StatusFound, rec.Code, "body: %s", rec.Body.String())
		assert.Equal(t, "https://drive.example.test/files/abc123", rec.Header().Get("Location"))
	})

	t.Run("v1 download redirects a link attachment to its provider", func(t *testing.T) {
		e, err := setupTestEnv()
		require.NoError(t, err)
		configureTestLinkProvider(t)
		token := humaTokenFor(t, &testuser1)

		id := createTestLinkAttachment(t, e, token, "def456")
		rec := humaRequest(t, e, http.MethodGet, "/api/v1/tasks/1/attachments/"+strconv.FormatInt(id, 10), "", token, "")
		assert.Equal(t, http.StatusFound, rec.Code, "body: %s", rec.Body.String())
		assert.Equal(t, "https://drive.example.test/files/def456", rec.Header().Get("Location"))
	})

	t.Run("an uploaded attachment still downloads its bytes", func(t *testing.T) {
		e, err := setupTestEnv()
		require.NoError(t, err)
		configureTestLinkProvider(t)
		token := humaTokenFor(t, &testuser1)

		id := uploadOneAttachment(t, e, token, "bytes.txt", []byte("stored bytes"))
		rec := humaRequest(t, e, http.MethodGet, "/api/v2/tasks/1/attachments/"+strconv.FormatInt(id, 10), "", token, "")
		assert.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
		assert.Empty(t, rec.Header().Get("Location"))
		assert.Equal(t, "stored bytes", rec.Body.String())
	})

	t.Run("a link attachment has no preview", func(t *testing.T) {
		e, err := setupTestEnv()
		require.NoError(t, err)
		configureTestLinkProvider(t)
		token := humaTokenFor(t, &testuser1)

		id := createTestLinkAttachment(t, e, token, "ghi789")
		rec := humaRequest(t, e, http.MethodGet, "/api/v2/tasks/1/attachments/"+strconv.FormatInt(id, 10)+"/preview?size=sm", "", token, "")
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code, "body: %s", rec.Body.String())
		assert.Empty(t, rec.Header().Get("Location"))
		assert.Contains(t, rec.Body.String(), strconv.Itoa(models.ErrCodeTaskAttachmentNotPreviewable))
	})

	t.Run("an unknown provider is rejected with the fork's own error code", func(t *testing.T) {
		e, err := setupTestEnv()
		require.NoError(t, err)
		configureTestLinkProvider(t)
		token := humaTokenFor(t, &testuser1)

		rec := humaRequest(t, e, http.MethodPost, "/api/v2/tasks/1/attachments/link",
			`{"link_provider":"nope","link_ref":"x","link_name":"x","link_size":1,"link_mime":"text/plain"}`, token, "")
		assert.Equal(t, http.StatusBadRequest, rec.Code, "body: %s", rec.Body.String())
		assert.Contains(t, rec.Body.String(), strconv.Itoa(models.ErrCodeUnknownLinkAttachmentProvider))
	})

	t.Run("link error codes do not collide with upstream codes", func(t *testing.T) {
		assert.NotEqual(t, models.ErrCodeInvalidTaskRepeatInterval, models.ErrCodeUnknownLinkAttachmentProvider)
		assert.NotEqual(t, models.ErrCodeInvalidBulkTaskCreationCount, models.ErrCodeInvalidLinkAttachment)
		assert.NotEqual(t, models.ErrCodeUnknownLinkAttachmentProvider, models.ErrCodeInvalidLinkAttachment)
	})

	t.Run("info publishes the providers and the source commit field", func(t *testing.T) {
		e, err := setupTestEnv()
		require.NoError(t, err)
		configureTestLinkProvider(t)

		rec := humaRequest(t, e, http.MethodGet, "/api/v2/info", "", "", "")
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
		var info map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &info))
		assert.Contains(t, info, "commit")
		require.Contains(t, info, "link_attachment_providers")
		assert.Contains(t, string(info["link_attachment_providers"]), `"picker_url":"https://drive.example.test/pick"`)
		assert.NotContains(t, string(info["link_attachment_providers"]), "resolve")
	})
}
