package nats

import (
	"encoding/json"
	"log"

	"github.com/Mirai3103/remote-compiler/internal/models"
	"github.com/nats-io/nats.go"
)

const (
	SubmissionResultSubject = "submission.result"
)

type Publisher struct {
	nc *nats.Conn
}

func NewPublisher(nc *nats.Conn) *Publisher {
	return &Publisher{nc: nc}
}

func (p *Publisher) PublishSubmissionResult(result models.SubmissionResult) error {
	data, err := json.Marshal(result)
	if err != nil {
		log.Printf("Error marshalling submission result: %v", err)
		return err
	}

	if err := p.nc.Publish(SubmissionResultSubject, data); err != nil {
		log.Printf("Error publishing submission result to NATS: %v", err)
		return err
	}
	log.Printf("Published result for SubmissionID: %s, TestCaseID: %s to NATS topic %s", result.SubmissionID, result.TestCaseID, SubmissionResultSubject)
	return nil
}
