package models

import "time"

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

// Accounts list.
type Accounts []Account
