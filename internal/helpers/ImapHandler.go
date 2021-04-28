package helpers

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-oauthdialog"
	"github.com/emersion/go-sasl"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type ImapHandler struct {
	client  *client.Client
	Mailbox *imap.MailboxStatus
}

// generateOauthToken generates a new token for OAuth
func (ih *ImapHandler) generateOauthToken(cfg *oauth2.Config) (*oauth2.Token, error) {
	supports, err := ih.client.SupportAuth(sasl.Xoauth2)
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
func (ih *ImapHandler) authenticate(token *oauth2.Token, username string) error {
	// Login to the IMAP server with XOAUTH2
	saslClient := sasl.NewXoauth2Client(username, token.AccessToken)
	return ih.client.Authenticate(saslClient)
}

// isAuthenticated indicates if the client is correctly authenticated
func (ih *ImapHandler) isAuthenticated() bool {
	return ih.client.State() != imap.NotAuthenticatedState
}

// EstablishConnection starts a TLS tunnel
func (ih *ImapHandler) EstablishConnection() error {
	// Connect to IMAP server
	log.Println("Establishing connection...")
	imapClient, err := client.DialTLS("imap.gmail.com:993", nil)
	if err != nil {
		return err
	}
	log.Println("Connection established")
	ih.client = imapClient
	return nil
}

// AuthenticateWithGmail authenticates the client with GMail IMAP
func (ih *ImapHandler) AuthenticateWithGmail(username string) error {
	token := GetToken(username)

	saveToken := true
	var authError error
	if token != nil {
		log.Println("Token found in DB")

		authError = ih.authenticate(token, username)
		if authError != nil || !ih.isAuthenticated() {
			log.Println("Token invalid, creating a new one...")
			token = nil
		} else {
			saveToken = false
		}
	}

	if token == nil {
		conf := &oauth2.Config{ // TODO: Use ENV here
			ClientID:     "866717686120-v856kp4tcircmuicpbvhq0nlrj7gcbil.apps.googleusercontent.com",
			ClientSecret: "0WmjOHmcCwSeHU2m8YrPD5j_",
			Scopes:       []string{"https://mail.google.com"},
			Endpoint:     google.Endpoint,
		}
		var genError error
		token, genError = ih.generateOauthToken(conf)
		if genError != nil {
			panic(genError)
		} else {
			authError = ih.authenticate(token, username)
		}
	}

	if authError == nil && !ih.isAuthenticated() {
		authError = errors.New("could not authenticate")
	} else if saveToken {
		log.Println("Saving token in DB...")
		SaveToken(username, token)
	}

	return authError
}

// ListMailboxes returns the accessible mailboxes
func (ih *ImapHandler) ListMailboxes() (chan *imap.MailboxInfo, error) {
	mailboxes := make(chan *imap.MailboxInfo, 10)
	done := make(chan error, 1)
	go func() {
		done <- ih.client.List("", "*", mailboxes)
	}()
	log.Println("Mailboxes:")
	for m := range mailboxes {
		log.Println("* " + m.Name)
	}
	if err := <-done; err != nil {
		return nil, err
	}

	return mailboxes, nil
}

// Logout logs the client out
func (ih *ImapHandler) Logout() error {
	log.Println("Logging out...")
	return ih.client.Logout()
}

// SelectMailbox selects the current mailbox
func (ih *ImapHandler) SelectMailbox(mailbox string) error {
	log.Println(fmt.Sprintf("Selecting inbox %v...", mailbox))
	mbox, err := ih.client.Select("Newsletter", false)
	if err != nil {
		return err
	}
	log.Println(fmt.Sprintf("%v selected", mailbox))
	ih.Mailbox = mbox
	return nil
}

// SearchNewMessages searches for new messages on the server
func (ih *ImapHandler) SearchNewMessages(since time.Time) ([]uint32, error) {
	// Search by start date
	criteria := imap.NewSearchCriteria()
	log.Println(fmt.Sprintf("Searching new messages since %v...", since))
	criteria.WithoutFlags = []string{imap.SeenFlag}
	criteria.SentSince = since
	ids, err := ih.client.Search(criteria)
	if err != nil {
		return nil, err
	}
	log.Println("IDs found:", ids)
	return ids, nil
}

// FetchEntireMessage fetches the whole message from uid seqset
func (ih *ImapHandler) FetchEntireMessage(seqset *imap.SeqSet, messages *chan *imap.Message) error {
	var section imap.BodySectionName
	return ih.client.Fetch(seqset, []imap.FetchItem{
		imap.FetchEnvelope,
		section.FetchItem(),
	}, *messages)
}
