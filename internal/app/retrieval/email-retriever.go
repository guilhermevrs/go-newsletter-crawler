package retrieval

import (
	"io/ioutil"
	"log"
	"net/mail"
	"time"

	"newsletter.crawler/internal/helpers"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
)

type EmailInfo struct {
	Date    string
	From    string
	Subject string
	Body    string
}

type EmailRetriever struct {
	imapClient *client.Client
	mailbox    *imap.MailboxStatus
}

// NewEmailRetrieval constructs a new EmailRetriever
func NewEmailRetriever() *EmailRetriever {
	helpers.InitilizeSchema()
	return &EmailRetriever{}
}

// login returns a client ready to be used
func (er *EmailRetriever) login() {
	log.Println("Connecting to the server...")
	// Connect to IMAP server
	imapClient, err := client.DialTLS("imap.gmail.com:993", nil)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Connection established")

	// Login
	log.Println("Authenticating...")
	helpers.AuthenticateWithGmail(imapClient, "guilhermevrs")
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Logged in")
	er.imapClient = imapClient
}

// logout logs the client out
func (er *EmailRetriever) logout() {
	er.imapClient.Logout()
}

// selectMailbox selects the mailbox for working
func (er *EmailRetriever) selectMailbox() {
	log.Println("Selecting inbox Newsletter...")
	mbox, err := er.imapClient.Select("Newsletter", false)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Newsletter inbox selected")
	er.mailbox = mbox
}

// searchItemsId gets the ids of messages related to the search
func (er *EmailRetriever) searchItemsId() []uint32 {
	// Search by start date
	criteria := imap.NewSearchCriteria()
	log.Println("Searching messages...")
	criteria.WithoutFlags = []string{imap.SeenFlag}
	criteria.SentSince = time.Date(2021, time.January, 20, 0, 0, 0, 0, time.UTC)
	ids, err := er.imapClient.Search(criteria)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("IDs found:", ids)
	return ids
}

// fetchByIds returns a channel containing the messages
func (er *EmailRetriever) fetchByIds(ids []uint32, messages *chan *imap.Message) <-chan error {
	log.Println("Fetching messages...")
	seqset := new(imap.SeqSet)
	seqset.AddNum(ids...)

	done := make(chan error, 1)

	var section imap.BodySectionName
	go func() {
		done <- er.imapClient.Fetch(seqset, []imap.FetchItem{
			imap.FetchEnvelope,
			section.FetchItem(),
		}, *messages)
	}()

	return done
}

// extractEmailInfo extracts info from Message
func extractEmailInfo(msgObj *imap.Message) EmailInfo {
	section := &imap.BodySectionName{}
	bodyReader := msgObj.GetBody(section)
	if bodyReader == nil {
		log.Fatal("Server didn't returned message body")
	}

	parsed, err := mail.ReadMessage(bodyReader)
	if err != nil {
		log.Fatal(err)
	}

	body, err := ioutil.ReadAll(parsed.Body)
	if err != nil {
		log.Fatal(err)
	}

	return EmailInfo{
		Date:    parsed.Header.Get("Date"),
		From:    parsed.Header.Get("From"),
		Subject: parsed.Header.Get("Subject"),
		Body:    string(body),
	}
}

// RetrieveEmails return the list of emails, in the given mailbox, that corresponds to the search
func (retriever EmailRetriever) RetrieveEmails() <-chan *EmailInfo {
	retriever.login()
	// Logout in the end
	defer retriever.logout()

	retriever.selectMailbox()
	if retriever.mailbox.Messages == 0 {
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
