package main

import (
	"errors"
	"log"
	"net/mail"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-oauthdialog"
	"github.com/emersion/go-sasl"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/go-pg/pg/v10"
	"github.com/go-pg/pg/v10/orm"
)

type Oauth2 struct {
	ID    int64
	User  string
	Token *oauth2.Token
}

func authenticate(c *client.Client, cfg *oauth2.Config, username string) error {
	DBToken := getToken(username)

	var accessToken string
	if DBToken == nil {

		supports, err := c.SupportAuth(sasl.Xoauth2)
		if !supports {
			return errors.New("XOAUTH2 not supported by the server")
		}

		// Ask for the user to login with his Google account
		code, err := oauthdialog.Open(cfg)
		if err != nil {
			return err
		}

		// Get a token from the returned code
		// This token can be saved in a secure store to be reused later
		token, err := cfg.Exchange(oauth2.NoContext, code)
		if err != nil {
			return err
		}

		// Adds the token to DB
		addToken(username, token)
		accessToken = token.AccessToken
	} else {
		accessToken = DBToken.AccessToken
	}

	// Login to the IMAP server with XOAUTH2
	saslClient := sasl.NewXoauth2Client(username, accessToken)
	return c.Authenticate(saslClient)
}

func addToken(user string, token *oauth2.Token) {
	execOnDB(func(db *pg.DB) error {
		_, err := db.Model(&Oauth2{
			User:  user,
			Token: token,
		}).Insert()
		return err
	})
}

func getToken(user string) *oauth2.Token {
	oauth := new(Oauth2)
	execOnDB(func(db *pg.DB) error {
		log.Println("Trying to get the token from DB for user", user)
		err := db.Model(oauth).Where("oauth2.user = ?", user).Select()
		if err == pg.ErrNoRows {
			log.Println("No tokens found")
			err = nil
		}
		return err
	})
	return oauth.Token
}

func createSchema(db *pg.DB) error {
	models := []interface{}{
		(*Oauth2)(nil),
	}

	for _, model := range models {
		log.Println("Creating schema")
		err := db.Model(model).CreateTable(&orm.CreateTableOptions{
			IfNotExists: true,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func execOnDB(fn func(db *pg.DB) error) error {
	db := pg.Connect(&pg.Options{
		User:     "admin",
		Password: "secret",
	})
	defer db.Close()

	err := fn(db)
	if err != nil {
		panic(err)
	}

	return nil
}

func main() {
	log.Println("Start DB")
	execOnDB(createSchema)

	log.Println("Connecting to the server")

	// Connect to server
	c, err := client.DialTLS("imap.gmail.com:993", nil)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Connected")
	// Don't forget to logout
	defer c.Logout()

	// Login
	conf := &oauth2.Config{
		ClientID:     "866717686120-v856kp4tcircmuicpbvhq0nlrj7gcbil.apps.googleusercontent.com",
		ClientSecret: "0WmjOHmcCwSeHU2m8YrPD5j_",
		Scopes:       []string{"https://mail.google.com"},
		Endpoint:     google.Endpoint,
	}
	authenticate(c, conf, "guilhermevrs")
	log.Println("Logged in")

	// List mailboxes
	mailboxes := make(chan *imap.MailboxInfo, 10)
	done := make(chan error, 1)
	go func() {
		done <- c.List("", "*", mailboxes)
	}()
	log.Println("Mailboxes:")
	for m := range mailboxes {
		log.Println("* " + m.Name)
	}
	if err := <-done; err != nil {
		log.Fatal(err)
	}

	// Select Newsletter
	mbox, err := c.Select("Newsletter", false)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Flags for Newsletter:", mbox.Flags)

	// Get the last 4 messages
	from := uint32(1)
	to := mbox.Messages
	if mbox.Messages > 3 {
		// We're using unsigned integers here, only subtract if the result is > 0
		from = mbox.Messages - 3
	}
	seqset := new(imap.SeqSet)
	seqset.AddRange(from, to)

	messages := make(chan *imap.Message, 10)
	done = make(chan error, 1)
	go func() {
		done <- c.Fetch(seqset, []imap.FetchItem{imap.FetchEnvelope}, messages)
	}()
	log.Println("Last 4 messages:")
	for msg := range messages {
		log.Println("* " + msg.Envelope.Subject)
	}
	if err := <-done; err != nil {
		log.Fatal(err)
	}

	// Get the last message
	if mbox.Messages == 0 {
		log.Fatal("No message in mailbox")
	}
	seqset = new(imap.SeqSet)
	seqset.AddRange(mbox.Messages, mbox.Messages)

	// Get the whole message body
	section := &imap.BodySectionName{}
	items := []imap.FetchItem{section.FetchItem()}

	messages = make(chan *imap.Message, 1)
	done = make(chan error, 1)
	go func() {
		done <- c.Fetch(seqset, items, messages)
	}()

	log.Println("Last message:")
	msg := <-messages
	r := msg.GetBody(section)
	if r == nil {
		log.Fatal("Server didn't returned message body")
	}

	if err := <-done; err != nil {
		log.Fatal(err)
	}

	m, err := mail.ReadMessage(r)
	if err != nil {
		log.Fatal(err)
	}

	header := m.Header
	log.Println("Date:", header.Get("Date"))
	log.Println("From:", header.Get("From"))
	log.Println("To:", header.Get("To"))
	log.Println("Subject:", header.Get("Subject"))

	/* body, err := ioutil.ReadAll(m.Body)
	if err != nil {
		log.Fatal(err)
	}
	log.Println(string(body)) */

	log.Println("Done!")
}
