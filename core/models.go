package core

import "time"

// Connection model.
type Connection struct {
	ID        int    `gorm:"primary_key"`
	ClientID  string `gorm:"column:client_id; type:varchar(70); not null; unique" json:"clientId,omitempty"`
	Key       string `gorm:"column:api_key; type:varchar(100); not null" json:"api_key,omitempty" binding:"required,max=100"`
	URL       string `gorm:"column:api_url; type:varchar(255); not null" json:"api_url,omitempty" binding:"required,validateCrmURL,max=255"` // nolint:lll
	GateURL   string `gorm:"column:mg_url; type:varchar(255); not null;" json:"mg_url,omitempty" binding:"max=255"`
	GateToken string `gorm:"column:mg_token; type:varchar(100); not null; unique" json:"mg_token,omitempty" binding:"max=100"`
	CreatedAt time.Time
	UpdatedAt time.Time
	Active    bool      `json:"active,omitempty"`
	Accounts  []Account `gorm:"foreignkey:ConnectionID"`
}

// Account model.
type Account struct {
	ID                  int    `gorm:"primary_key"`
	ConnectionID        int    `gorm:"column:connection_id" json:"connectionId,omitempty"`
	Channel             uint64 `gorm:"column:channel; not null; unique" json:"channel,omitempty"`
	ChannelSettingsHash string `gorm:"column:channel_settings_hash; type:varchar(70)" binding:"max=70"`
	Name                string `gorm:"column:name; type:varchar(40)" json:"name,omitempty" binding:"max=40"`
	Lang                string `gorm:"column:lang; type:varchar(2)" json:"lang,omitempty" binding:"max=2"`
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// User model.
type User struct {
	ID           int    `gorm:"primary_key"`
	ExternalID   string `gorm:"column:external_id; type:varchar(255); not null; unique"`
	UserPhotoURL string `gorm:"column:user_photo_url; type:varchar(255)" binding:"max=255"`
	UserPhotoID  string `gorm:"column:user_photo_id; type:varchar(100)" binding:"max=100"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// TableName will return table name for User
// It will not work if User is not embedded, but mapped as another type
//      type MyUser User // will not work
// but
//      type MyUser struct { // will work
//          User
//      }
func (User) TableName() string {
	return "mg_user"
}

// Accounts list.
type Accounts []Account
