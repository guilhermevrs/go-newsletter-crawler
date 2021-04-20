package steps

import (
	"log"

	"github.com/emersion/go-imap/client"
	utilsImap "newsletter.crawler/imap"
)

func ListMailboxesStep(imapClient *client.Client) error {
	mailboxes, err := utilsImap.ListMailboxes(imapClient)

	if err == nil {
		log.Fatal(err)
	} else {
		log.Println("Mailboxes:")
		for m := range mailboxes {
			log.Println("* " + m.Name)
		}
	}
	return err
}
