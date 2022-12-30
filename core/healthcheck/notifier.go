package healthcheck

import retailcrm "github.com/retailcrm/api-client-go/v2"

func DefaultNotifyFunc(apiURL, apiKey, msg string) error {
	client := retailcrm.New(apiURL, apiKey)
	_, err := client.NotificationsSend(retailcrm.NotificationsSendRequest{
		UserGroups: []retailcrm.UserGroupType{retailcrm.UserGroupSuperadmins},
		Type:       retailcrm.NotificationTypeError,
		Message:    msg,
	})
	return err
}
