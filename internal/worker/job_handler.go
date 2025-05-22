package worker

import (
	"context"
	"github.com/Mirai3103/remote-compiler/internal/config"
	"github.com/Mirai3103/remote-compiler/internal/core"
	"github.com/Mirai3103/remote-compiler/internal/models" // Điều chỉnh import path nếu cần
	natsClient "github.com/Mirai3103/remote-compiler/internal/nats"
	"log"
	"time"
)

type JobHandler struct {
	natsPublisher *natsClient.Publisher
	runner        *core.Runner
	jobSemaphore  chan struct{}
}

func NewJobHandler(publisher *natsClient.Publisher, runner *core.Runner, runnerCfg *config.RunnerConfig) *JobHandler {
	var sem chan struct{}
	maxJobs := runnerCfg.MaxConcurrentJobs
	if maxJobs > 0 {
		sem = make(chan struct{}, maxJobs)
		log.Printf("JobHandler initialized with MaxConcurrentJobs: %d", maxJobs)
	} else {
		log.Printf("JobHandler initialized with unlimited concurrent jobs (MaxConcurrentJobs is %d)", maxJobs)
	}

	return &JobHandler{
		natsPublisher: publisher,
		runner:        runner,
		jobSemaphore:  sem,
	}
}

// HandleSubmission processes a single submission.
// This method signature matches the SubmissionProcessor interface in the nats package.
func (h *JobHandler) HandleSubmission(submission models.Submission) {
	if h.jobSemaphore != nil {
		log.Printf("JobHandler: Attempting to acquire semaphore for SubmissionID: %s. Current active jobs: %d/%d",
			submission.ID, len(h.jobSemaphore), cap(h.jobSemaphore)) // Lưu ý: len(channel) là số item đang có, cap(channel) - len(channel) là số chỗ trống.
		// Khi một goroutine chiếm slot, nó sẽ ghi vào channel.
		// Số goroutine đang chờ (nếu channel đầy) không dễ thấy trực tiếp.
		// Để log chính xác số job đang chạy, bạn cần một atomic counter riêng.
		// Hoặc hiểu len(h.jobSemaphore) là số lượng "token" đã được dùng nếu channel đầy.
		// Cách đơn giản hơn để hiểu: cap(h.jobSemaphore) là tổng slot, khi acquire thì 1 slot bị chiếm.
		now := time.Now()
		h.jobSemaphore <- struct{}{} // Acquire a slot.
		log.Printf("JobHandler: Semaphore acquired for SubmissionID: %s. Time taken: %s", submission.ID, time.Since(now))
		log.Printf("JobHandler: Semaphore acquired for SubmissionID: %s.", submission.ID)
		defer func() {
			<-h.jobSemaphore // Release the slot khi xử lý xong
			log.Printf("JobHandler: Semaphore released for SubmissionID: %s.", submission.ID)
		}()
	} else {
		log.Printf("JobHandler: Processing SubmissionID: %s without concurrency limit.", submission.ID)
	}
	log.Printf("JobHandler: Received SubmissionID: %s. Delegating to Core Runner.", submission.ID)
	// Nên tạo context sau khi đã chiếm được slot từ semaphore nếu bạn muốn timeout chỉ áp dụng cho ProcessSubmission.
	submissionCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute) // Timeout này từ code gốc
	defer cancel()

	h.runner.ProcessSubmission(submissionCtx, submission)
	log.Printf("JobHandler: Core Runner finished processing SubmissionID: %s.", submission.ID)
}
