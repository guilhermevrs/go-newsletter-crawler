package main

import (
	"log"
	"net/mail"
	"time"

	"newsletter.crawler/db"
	utilsImap "newsletter.crawler/imap"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
)

func main() {
	log.Println("Start DB")

	db.InitilizeSchema()

	log.Println("Connecting to the server")

	// Connect to IMAP server
	imapClient, err := client.DialTLS("imap.gmail.com:993", nil)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Connected")
	// Logout in the end
	defer imapClient.Logout()

	// Login
	utilsImap.AuthenticateWithGmail(imapClient, "guilhermevrs")
	if err != nil {
		log.Println("Didnt work!")
		log.Fatal(err)
	}
	log.Println("Logged in")
	imapClient.Check()

	// List mailboxes

	// Select Newsletter
	mbox, err := imapClient.Select("Newsletter", false)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Flags for Newsletter:", mbox.Flags)

	// Search by start date
	criteria := imap.NewSearchCriteria()
	log.Println("Going to search...")
	criteria.WithoutFlags = []string{imap.SeenFlag}
	criteria.SentSince = time.Date(2021, time.January, 20, 0, 0, 0, 0, time.UTC)
	ids, err := imapClient.Search(criteria)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("IDs found:", ids)

	if len(ids) > 0 {
		seqset := new(imap.SeqSet)
		seqset.AddNum(ids...)

		messages := make(chan *imap.Message, 10)
		done := make(chan error, 1)
		go func() {
			done <- imapClient.Fetch(seqset, []imap.FetchItem{imap.FetchEnvelope}, messages)
		}()

		log.Println("Unseen messages received since:")
		for msg := range messages {
			log.Println("* " + msg.Envelope.Subject)
		}

		if err := <-done; err != nil {
			log.Fatal(err)
		}
	}

	// Get the last message
	if mbox.Messages == 0 {
		log.Fatal("No message in mailbox")
	}
	seqset := new(imap.SeqSet)
	seqset.AddRange(mbox.Messages, mbox.Messages)

	// Get the whole message body
	section := &imap.BodySectionName{}
	items := []imap.FetchItem{section.FetchItem()}

	messages := make(chan *imap.Message, 1)
	done := make(chan error, 1)
	go func() {
		done <- imapClient.Fetch(seqset, items, messages)
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
