package models

import (
	"golang.org/x/oauth2"
)

//Oauth2 describes the Oauth2 information needed for IMAP
type Oauth2 struct {
	ID    int64
	User  string
	Token *oauth2.Token
}
