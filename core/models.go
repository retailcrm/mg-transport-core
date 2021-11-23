package core

import "time"

// Connection model.
type Connection struct {
	CreatedAt time.Time
	UpdatedAt time.Time
	Key       string    `gorm:"column:api_key; type:varchar(100); not null" json:"api_key,omitempty" binding:"required,max=100"`                // nolint:lll
	URL       string    `gorm:"column:api_url; type:varchar(255); not null" json:"api_url,omitempty" binding:"required,validateCrmURL,max=255"` // nolint:lll
	GateURL   string    `gorm:"column:mg_url; type:varchar(255); not null;" json:"mg_url,omitempty" binding:"max=255"`
	GateToken string    `gorm:"column:mg_token; type:varchar(100); not null; unique" json:"mg_token,omitempty" binding:"max=100"` // nolint:lll
	ClientID  string    `gorm:"column:client_id; type:varchar(70); not null; unique" json:"clientId,omitempty"`
	Accounts  []Account `gorm:"foreignkey:ConnectionID"`
	ID        int       `gorm:"primary_key"`
	Active    bool      `json:"active,omitempty"`
}

// Account model.
type Account struct {
	CreatedAt           time.Time
	UpdatedAt           time.Time
	ChannelSettingsHash string `gorm:"column:channel_settings_hash; type:varchar(70)" binding:"max=70"`
	Name                string `gorm:"column:name; type:varchar(100)" json:"name,omitempty" binding:"max=100"`
	Lang                string `gorm:"column:lang; type:varchar(2)" json:"lang,omitempty" binding:"max=2"`
	Channel             uint64 `gorm:"column:channel; not null; unique" json:"channel,omitempty"`
	ID                  int    `gorm:"primary_key"`
	ConnectionID        int    `gorm:"column:connection_id" json:"connectionId,omitempty"`
}

// User model.
type User struct {
	CreatedAt    time.Time
	UpdatedAt    time.Time
	ExternalID   string `gorm:"column:external_id; type:varchar(255); not null; unique"`
	UserPhotoURL string `gorm:"column:user_photo_url; type:varchar(255)" binding:"max=255"`
	UserPhotoID  string `gorm:"column:user_photo_id; type:varchar(100)" binding:"max=100"`
	ID           int    `gorm:"primary_key"`
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
