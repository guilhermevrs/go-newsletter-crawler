package main

import (
	"fmt"
	"io"
	"os"

	"newsletter.crawler/internal/app/retrieval"
)

func main() {
	// TODO: Seems that the login from token in DB is broken now...
	retriever := retrieval.NewEmailRetriever()
	channel := retriever.Execute()

	for email := range channel {
		file, _ := os.Create(fmt.Sprintf("%v_%v_%v.html", email.Date, email.From, email.Subject))
		_, _ = io.WriteString(file, email.BodyHtml)
		file.Sync()
		file.Close()
	}
	// TODO: Get the links in the MIME (the final links, to see if there is no redirection)
}
