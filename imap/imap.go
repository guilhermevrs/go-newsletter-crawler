package imap

import (
	"errors"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-oauthdialog"
	"github.com/emersion/go-sasl"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"newsletter.crawler/db"
)

// GenerateOauthToken generates a new token for OAuth
func GenerateOauthToken(client *client.Client, cfg *oauth2.Config) (*oauth2.Token, error) {
	supports, err := client.SupportAuth(sasl.Xoauth2)
	if !supports {
		return nil, errors.New("XOAUTH2 not supported by the server")
	}

	// Ask for the user to login with his Google account
	code, err := oauthdialog.Open(cfg)
	if err != nil {
		return nil, err
	}

	// Get a token from the returned code
	// This token can be saved in a secure store to be reused later
	token, err := cfg.Exchange(oauth2.NoContext, code)
	if err != nil {
		return nil, err
	}
	return token, nil
}

// Authenticate will authenticate the client for the given user using the provided token
func Authenticate(client *client.Client, token *oauth2.Token, username string) error {
	// Login to the IMAP server with XOAUTH2
	saslClient := sasl.NewXoauth2Client(username, token.AccessToken)
	return client.Authenticate(saslClient)
}

func isAuthenticated(c *client.Client) bool {
	return c.State() != imap.NotAuthenticatedState
}

// AuthenticateWithGmail authenticates the client with GMail IMAP
func AuthenticateWithGmail(client *client.Client, username string) error {
	token := db.GetToken(username)

	var authError error
	if token != nil {
		authError = Authenticate(client, token, username)
		if authError != nil || !isAuthenticated(client) {
			token = nil
		}
	}

	if token == nil {
		conf := &oauth2.Config{
			ClientID:     "866717686120-v856kp4tcircmuicpbvhq0nlrj7gcbil.apps.googleusercontent.com",
			ClientSecret: "0WmjOHmcCwSeHU2m8YrPD5j_",
			Scopes:       []string{"https://mail.google.com"},
			Endpoint:     google.Endpoint,
		}
		var genError error
		token, genError = GenerateOauthToken(client, conf)
		if genError != nil {
			panic(genError)
		}
	}

	authError = Authenticate(client, token, username)

	if authError == nil && !isAuthenticated(client) {
		authError = errors.New("Could not authenticate")
	}

	return authError

}
