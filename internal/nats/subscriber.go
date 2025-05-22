package nats

import (
	"encoding/json"
	"log"

	"github.com/Mirai3103/remote-compiler/internal/models"
	"github.com/nats-io/nats.go"
	// KHÔNG import "runner-service/internal/worker" ở đây nữa
)

const (
	SubmissionCreatedSubject = "submission.created"
	QueueGroup               = "runner-service-group"
)

// SubmissionProcessor defines the interface for handling submissions.
// Any type that implements HandleSubmission can be used by the NATS subscriber.
type SubmissionProcessor interface {
	HandleSubmission(submission models.Submission)
}

type Subscriber struct {
	nc                *nats.Conn
	submissionHandler SubmissionProcessor // Thay đổi ở đây: dùng interface
}

// NewSubscriber bây giờ nhận một SubmissionProcessor
func NewSubscriber(nc *nats.Conn, handler SubmissionProcessor) *Subscriber {
	return &Subscriber{
		nc:                nc,
		submissionHandler: handler, // Gán interface
	}
}

func (s *Subscriber) SubscribeToSubmissions() (*nats.Subscription, error) {
	subscription, err := s.nc.QueueSubscribe(SubmissionCreatedSubject, QueueGroup, func(msg *nats.Msg) {
		log.Printf("Received a message on subject: %s, queue: %s", msg.Subject, msg.Sub.Queue)
		var sub models.Submission
		err := json.Unmarshal(msg.Data, &sub)
		if err != nil {
			log.Printf("Error unmarshalling submission data: %v. Message data: %s", err, string(msg.Data))
			return
		}

		// Gọi method của interface
		go s.submissionHandler.HandleSubmission(sub)
	})

	if err != nil {
		log.Printf("Error subscribing to NATS subject %s: %v", SubmissionCreatedSubject, err)
		return nil, err
	}

	log.Printf("Subscribed to NATS subject: %s, queue group: %s", SubmissionCreatedSubject, QueueGroup)
	return subscription, nil
}
