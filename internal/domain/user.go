package domain

import "time"

const (
	AuthProviderGoogle    = "google"
	AuthProviderMagicLink = "magic_link"
)

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	Name         string    `json:"name"`
	AvatarURL    *string   `json:"avatar_url,omitempty"`
	AuthProvider string    `json:"-"`
	GoogleID     *string   `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
