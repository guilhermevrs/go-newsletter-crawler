package retrieval

import (
	"io"
	"io/ioutil"
	"log"
	"time"

	"golang.org/x/oauth2"
	"newsletter.crawler/internal/helpers"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-message/mail"
)

type EmailInfo struct {
	Date      string
	From      string
	Subject   string
	BodyHtml  string
	BodyPlain string
}

type EmailRetriever struct {
	imapHandler *helpers.ImapHandler
	pgHandler   *helpers.PgHandler
}

// NewEmailRetrieval constructs a new EmailRetriever
func NewEmailRetriever() *EmailRetriever {
	pgHandler := helpers.NewPgHandler()
	pgHandler.InitilizeSchema()
	return &EmailRetriever{
		imapHandler: new(helpers.ImapHandler),
		pgHandler:   pgHandler,
	}
}

// login returns a client ready to be used
func (er *EmailRetriever) login() {
	log.Println("Connecting to the server...")
	err := er.imapHandler.EstablishConnection()
	if err != nil {
		log.Fatal(err)
	}

	username := "guilhermevrs" // TODO: Get that from ENV / CLI

	// Checking DB for Token
	var token *oauth2.Token
	token, err = er.pgHandler.GetToken(username)
	if err != nil {
		log.Fatal(err)
	}

	// Login
	var isNew bool
	log.Println("Authenticating...")
	isNew, err = er.imapHandler.AuthenticateWithGmail(username, token)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Logged in")

	if isNew {
		er.pgHandler.SaveToken(username, token)
	}
}

// logout logs the client out
func (er *EmailRetriever) logout() {
	er.imapHandler.Logout()
}

// selectMailbox selects the mailbox for working
func (er *EmailRetriever) selectMailbox() {
	er.imapHandler.SelectMailbox("Newsletter") // TODO: Coming from ENV or CLI
}

// searchItemsId gets the ids of messages related to the search
func (er *EmailRetriever) searchItemsId() []uint32 {
	// Search by start date
	ids, err := er.imapHandler.SearchNewMessages(time.Date(2021, time.January, 20, 0, 0, 0, 0, time.UTC)) // TODO: >Get this data from db
	if err != nil {
		log.Fatal(err)
	}
	return ids
}

// fetchByIds returns a channel containing the messages
func (er *EmailRetriever) fetchByIds(ids []uint32, messages *chan *imap.Message) <-chan error {
	log.Println("Fetching messages...")
	seqset := new(imap.SeqSet)
	seqset.AddNum(ids...)

	done := make(chan error, 1)
	go func() {
		done <- er.imapHandler.FetchEntireMessage(seqset, messages)
	}()

	return done
}

// extractEmailInfo extracts info from Message
func extractEmailInfo(msgObj *imap.Message) EmailInfo {
	section := &imap.BodySectionName{}
	bodyLiteral := msgObj.GetBody(section)
	if bodyLiteral == nil {
		log.Fatal("Server didn't returned message body")
	}

	reader, err := mail.CreateReader(bodyLiteral)
	if err != nil {
		log.Fatal(err)
	}

	var bodyHtml string
	var bodyTextPlain string
	for {
		p, err := reader.NextPart()
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatal(err)
		}

		switch h := p.Header.(type) {
		case *mail.InlineHeader:
			ct, _, _ := h.ContentType()
			bodyBytes, _ := ioutil.ReadAll(p.Body)
			if ct == "text/html" {
				bodyHtml = string(bodyBytes)
			} else if ct == "text/plain" {
				bodyTextPlain = string(bodyBytes)
			}
		}
	}

	date, _ := reader.Header.Date()
	from, _ := reader.Header.AddressList("From")
	subject, _ := reader.Header.Subject()
	return EmailInfo{
		Date:      date.String(),
		From:      from[0].Address,
		Subject:   subject,
		BodyHtml:  bodyHtml,
		BodyPlain: bodyTextPlain,
	}
}

// Execute return the list of emails, in the given mailbox, that corresponds to the search
func (retriever EmailRetriever) Execute() <-chan *EmailInfo {
	retriever.login()
	// Logout in the end
	defer retriever.logout()

	retriever.selectMailbox()
	if retriever.imapHandler.Mailbox.Messages == 0 {
		log.Println("No messages in the mailbox")
		return nil
	}

	searchIds := retriever.searchItemsId()
	if len(searchIds) == 0 {
		log.Println("No message corresponds to the search")
		return nil
	}

	messageObjectList := make(chan *imap.Message, 10)
	done := retriever.fetchByIds(searchIds, &messageObjectList)

	infoChannel := make(chan *EmailInfo, 10)
	for msgObj := range messageObjectList {
		info := extractEmailInfo(msgObj)
		infoChannel <- &info
	}

	if err := <-done; err != nil {
		log.Fatal(err)
	}

	return infoChannel
}
