package healthcheck

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	retailcrm "github.com/retailcrm/api-client-go/v2"
	"github.com/retailcrm/mg-transport-core/v2/core/util/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/h2non/gock.v1"
)

func TestDefaultNotifyFunc(t *testing.T) { // nolint:paralleltest
	apiURL := "https://test.retailcrm.pro"
	apiKey := "key"
	msg := "Notification"

	data, err := json.Marshal(retailcrm.NotificationsSendRequest{
		UserGroups: []retailcrm.UserGroupType{retailcrm.UserGroupSuperadmins},
		Type:       retailcrm.NotificationTypeError,
		Message:    msg,
	})
	require.NoError(t, err)

	defer gock.Off()
	gock.New(apiURL).
		Post("/api/v5/notifications/send").
		BodyString(url.Values{"notification": {string(data)}}.Encode()).
		Reply(http.StatusOK).
		JSON(retailcrm.SuccessfulResponse{Success: true})

	assert.NoError(t, DefaultNotifyFunc(apiURL, apiKey, msg))
	testutil.AssertNoUnmatchedRequests(t)
}

func TestDefaultNotifyFunc_Error(t *testing.T) { // nolint:paralleltest
	apiURL := "https://test.retailcrm.pro"
	apiKey := "key"
	msg := "Notification"

	data, err := json.Marshal(retailcrm.NotificationsSendRequest{
		UserGroups: []retailcrm.UserGroupType{retailcrm.UserGroupSuperadmins},
		Type:       retailcrm.NotificationTypeError,
		Message:    msg,
	})
	require.NoError(t, err)

	defer gock.Off()
	gock.New(apiURL).
		Post("/api/v5/notifications/send").
		BodyString(url.Values{"notification": {string(data)}}.Encode()).
		Reply(http.StatusForbidden).
		JSON(retailcrm.ErrorResponse{
			SuccessfulResponse: retailcrm.SuccessfulResponse{Success: false},
			ErrorMessage:       "Forbidden",
		})

	err = DefaultNotifyFunc(apiURL, apiKey, msg)
	assert.Error(t, err)
	assert.Equal(t, "Forbidden", err.Error())
	testutil.AssertNoUnmatchedRequests(t)
}
