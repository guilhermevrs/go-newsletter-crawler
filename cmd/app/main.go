package main

import (
	"fmt"
	"io"
	"os"

	"newsletter.crawler/internal/app/retrieval"
)

func main() {
	retriever := retrieval.NewEmailRetriever()
	channel := retriever.RetrieveEmails()

	for email := range channel {
		file, _ := os.Create(fmt.Sprintf("%v_%v_%v.txt", email.Date, email.From, email.Subject))
		defer file.Close()

		_, _ = io.WriteString(file, email.Body)
		file.Sync()
	}
}
