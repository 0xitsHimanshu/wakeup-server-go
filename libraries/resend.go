package libraries

import (
	"log"
	"os"
	"sync"

	"github.com/resend/resend-go/v3"
)

var (
	client *resend.Client
	clientOnce sync.Once
)

func getClient() *resend.Client {
	clientOnce.Do(func ()  {
		RESEND_API_KEY := os.Getenv("RESEND_API_KEY")
		log.Println("Initializing Resend client")
		client = resend.NewClient(RESEND_API_KEY)
	})
	return client
}

func SendEmail(param *resend.SendEmailRequest) error {
	client := getClient()
	_, err := client.Emails.Send(param)
	if err != nil {
		return err
	}
	return nil
}