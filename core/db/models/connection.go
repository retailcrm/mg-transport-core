package models

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
