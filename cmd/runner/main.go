package main

import (
	"github.com/Mirai3103/remote-compiler/internal/core"
	"github.com/Mirai3103/remote-compiler/internal/core/sandbox"
	"log"
	"os"
	"os/signal"

	// "runtime" // Không còn cần thiết nếu dùng signal.Notify
	"syscall"
	"time"

	natsClient "github.com/Mirai3103/remote-compiler/internal/nats" // Điều chỉnh import path nếu cần
	"github.com/Mirai3103/remote-compiler/internal/worker"          // Điều chỉnh import path nếu cần

	appConfig "github.com/Mirai3103/remote-compiler/internal/config"
	"github.com/nats-io/nats.go"
)

var globalConfig *appConfig.Config // Biến toàn cục để giữ config
func main() {
	log.Println("Starting Runner Service...")
	cfg, err := appConfig.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	globalConfig = cfg
	natsURL := globalConfig.NATS.URL
	nc, err := nats.Connect(natsURL,
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(5),
		nats.ReconnectWait(2*time.Second),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			log.Printf("NATS disconnected: %v", err)
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Printf("NATS reconnected to %s", nc.ConnectedUrl())
		}),
		nats.ClosedHandler(func(nc *nats.Conn) {
			log.Printf("NATS connection closed.")
		}),
	)
	if err != nil {
		log.Fatalf("Error connecting to NATS: %v", err)
	}
	defer nc.Close()
	log.Printf("Connected to NATS server: %s", natsURL)
	sandboxExecutor := sandbox.NewExecutor(cfg.Runner)
	if sandboxExecutor == nil {
		panic("Failed to create sandbox executor")
	}

	publisher := natsClient.NewPublisher(nc)
	runner := core.NewRunner(sandboxExecutor, publisher, &cfg.Runner)

	jobHandler := worker.NewJobHandler(publisher, runner, &cfg.Runner) // jobHandler là *worker.JobHandler

	// Khi gọi NewSubscriber, jobHandler (*worker.JobHandler)
	// tương thích với natsClient.SubmissionProcessor interface
	// vì nó có method HandleSubmission(models.Submission)
	subscriber := natsClient.NewSubscriber(nc, jobHandler)
	subSubscription, err := subscriber.SubscribeToSubmissions()
	if err != nil {
		log.Fatalf("Error setting up NATS subscription: %v", err)
	}
	defer func() {
		if err := subSubscription.Unsubscribe(); err != nil {
			log.Printf("Error unsubscribing: %v", err)
		}
		// Consider nc.Drain() for graceful shutdown of NATS connection
		if err := nc.Drain(); err != nil {
			log.Printf("Error draining NATS connection: %v", err)
		}
	}()

	log.Println("Runner Service is now listening for submissions on NATS.")

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs

	log.Println("Shutting down Runner Service...")
}
