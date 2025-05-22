package core

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	// Để lấy thông tin ngôn ngữ từ languages.json
	"github.com/Mirai3103/remote-compiler/internal/config"
	"github.com/Mirai3103/remote-compiler/internal/core/sandbox" // Interface Executor và các struct RunRequest, ExecuteResult
	"github.com/Mirai3103/remote-compiler/internal/models"
	"github.com/Mirai3103/remote-compiler/internal/nats" // NATS Publisher
)

// Runner orchestrates the code compilation (if needed) and execution for a submission.
type Runner struct {
	sandboxExecutor sandbox.Executor // Một instance của sandbox executor (ví dụ: FirejailExecutor)
	natsPublisher   *nats.Publisher  // Để publish kết quả từng test case
	runnerConfig    *config.RunnerConfig
}

// NewRunner creates a new Runner instance.
func NewRunner(executor sandbox.Executor, publisher *nats.Publisher, runnerConfig *config.RunnerConfig) *Runner {
	return &Runner{
		sandboxExecutor: executor,
		natsPublisher:   publisher,
		runnerConfig:    runnerConfig,
	}
}

// ProcessSubmission là hàm chính xử lý toàn bộ submission.
// Nó được gọi bởi worker.JobHandler.
func (r *Runner) ProcessSubmission(ctx context.Context, submission models.Submission) {
	log.Printf("Processing SubmissionID: %s, Language: %s", submission.ID, submission.Language.RunCommand) // Giả sử client gửi LanguageID

	// 1. Lấy cấu hình chi tiết cho ngôn ngữ từ `languages.json`
	langDetails := submission.Language

	// 2. Tạo thư mục tạm duy nhất cho submission này
	var tempDir string = r.runnerConfig.SandboxBaseDir + "/" + submission.ID
	err := os.MkdirAll(tempDir, 0755)
	if err != nil {
		log.Printf("Error creating temp directory for SubmissionID %s: %v", submission.ID, err)
		r.publishOverallError(submission.ID, models.InternalError, "Failed to create temp environment.")
		return
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			log.Printf("Error removing temp directory %s: %v", tempDir, err)
		} else {
			log.Printf("Cleaned up temp directory: %s", tempDir)
		}
	}()
	log.Printf("Created temp directory for SubmissionID %s: %s", submission.ID, tempDir)

	// 3. Ghi source code vào file trong thư mục tạm
	sourceFilePath := filepath.Join(tempDir, langDetails.SourceFile)
	if err := os.WriteFile(sourceFilePath, []byte(submission.Code), 0644); err != nil {
		log.Printf("Error writing source code for SubmissionID %s: %v", submission.ID, err)
		r.publishOverallError(submission.ID, models.InternalError, "Failed to write source code.")
		return
	}
	log.Printf("Source code written to: %s", sourceFilePath)

	// 4. Bước Biên Dịch (nếu ngôn ngữ yêu cầu)
	executablePath := sourceFilePath      // Mặc định cho ngôn ngữ thông dịch
	if langDetails.CompileCommand != "" { // nếu có compile command
		compileCmdTemplate := strings.Fields(langDetails.CompileCommand) // ["go", "build", "-o", "{output_file}", "{source_file}"]

		outputFileName := langDetails.BinaryFile // Ví dụ: "a.out" hoặc "main"
		if outputFileName == "" {
			outputFileName = "executable" // Tên mặc định nếu không có trong config
		}
		//todo: check file extension
		compiledExecutablePath := filepath.Join(tempDir, outputFileName)

		actualCompileCmd := make([]string, len(compileCmdTemplate))
		for i, part := range compileCmdTemplate {
			part = strings.ReplaceAll(part, "{source_file}", sourceFilePath)
			part = strings.ReplaceAll(part, "{output_file}", compiledExecutablePath)
			// Bạn có thể thêm các placeholder khác như {temp_dir}
			part = strings.ReplaceAll(part, "{temp_dir}", tempDir)
			actualCompileCmd[i] = part
		}

		log.Printf("Compiling SubmissionID %s with command: %v", submission.ID, actualCompileCmd)

		// Set timeout cho quá trình biên dịch (ví dụ: 30 giây)
		compileCtx, compileCancel := context.WithTimeout(ctx, 30*time.Second)
		defer compileCancel()

		cmd := exec.CommandContext(compileCtx, actualCompileCmd[0], actualCompileCmd[1:]...)
		cmd.Dir = tempDir                                 // Chạy lệnh biên dịch từ thư mục tạm
		compileOutput, compileErr := cmd.CombinedOutput() // Lấy cả stdout và stderr của trình biên dịch

		if compileErr != nil {
			log.Printf("Compilation failed for SubmissionID %s: %v. Output: %s", submission.ID, compileErr, string(compileOutput))
			// Gửi kết quả Compile Error cho tất cả test cases hoặc một kết quả tổng
			for _, tc := range submission.TestCases {
				result := models.SubmissionResult{
					SubmissionID: submission.ID,
					TestCaseID:   tc.ID,
					Status:       models.CompileError,
					Error:        string(compileOutput), // Gửi output lỗi biên dịch
				}
				r.natsPublisher.PublishSubmissionResult(result)
			}
			return // Dừng xử lý nếu biên dịch lỗi
		}
		executablePath = compiledExecutablePath // Cập nhật đường dẫn file thực thi
		log.Printf("Compilation successful for SubmissionID %s. Executable at: %s", submission.ID, executablePath)
	}

	// 5. Chuẩn bị Lệnh Chạy cho Sandbox
	// langDetails.RunCmd là template từ languages.json, ví dụ: "./{executable}" hoặc "python3 {source_file}"
	runCmdTemplate := strings.Fields(langDetails.RunCommand)
	actualRunCmd := make([]string, len(runCmdTemplate))
	for i, part := range runCmdTemplate {
		part = strings.ReplaceAll(part, "{executable}", executablePath)  // Nếu đã biên dịch
		part = strings.ReplaceAll(part, "{source_file}", sourceFilePath) // Nếu là script
		// Bạn có thể thêm các placeholder khác như {temp_dir}
		part = strings.ReplaceAll(part, "{temp_dir}", tempDir)
		actualRunCmd[i] = part
	}
	log.Printf("Prepared run command for SubmissionID %s: %v", submission.ID, actualRunCmd)

	// 6. Chạy từng Test Case
	for _, tc := range submission.TestCases {
		log.Printf("Running TestCaseID: %s for SubmissionID: %s", tc.ID, submission.ID)

		// Tạo context với timeout cho test case này
		runCtx, runCancel := context.WithTimeout(ctx, time.Duration(submission.TimeLimitInMs)*time.Millisecond)
		defer runCancel()

		sandboxReq := sandbox.RunRequest{
			SubmissionID:     submission.ID,
			TestCaseID:       tc.ID,
			RunCommand:       actualRunCmd,
			WorkingDirectory: tempDir, // Sandbox sẽ chạy lệnh từ thư mục này
			Input:            tc.Input,
			TimeLimitMs:      submission.TimeLimitInMs,
			MemoryLimitKb:    submission.MemoryLimitInKb,
		}

		// Gọi Executor để chạy code trong sandbox
		execResult, err := r.sandboxExecutor.Execute(runCtx, sandboxReq)

		finalStatus := models.TestcaseStatus("")
		var output, execErrorMsg string
		timeUsed := 0
		memoryUsed := 0

		if err != nil { // Lỗi từ chính sandbox executor (không phải lỗi của code user)
			log.Printf("Sandbox execution error for TestCaseID %s, SubmissionID %s: %v", tc.ID, submission.ID, err)
			finalStatus = models.InternalError
			execErrorMsg = fmt.Sprintf("Sandbox execution failed: %v", err)
		} else {
			finalStatus = execResult.Status
			output = execResult.Stdout
			execErrorMsg = execResult.Stderr // Stderr từ code người dùng
			timeUsed = execResult.TimeUsedMs
			memoryUsed = execResult.MemoryUsedKb

			// Nếu sandbox chạy thành công (code người dùng có thể vẫn lỗi runtime, TLE, MLE)
			// và status trả về là Success (nghĩa là code chạy xong trong giới hạn)
			// thì mới cần so sánh output.
			if finalStatus == models.Success {
				if r.compareOutput(output, tc.ExpectOutput, submission.Settings) {
					finalStatus = models.Success
				} else {
					finalStatus = models.WrongAnswer
				}
			}
		}

		// Chuẩn bị và gửi kết quả của test case này
		result := models.SubmissionResult{
			SubmissionID:   submission.ID,
			TestCaseID:     tc.ID,
			Status:         finalStatus,
			TimeUsedInMs:   timeUsed,
			MemoryUsedInKb: memoryUsed,
			Output:         output,       // stdout của user code
			Error:          execErrorMsg, // stderr của user code hoặc lỗi sandbox
		}
		r.natsPublisher.PublishSubmissionResult(result)
		log.Printf("Result for TestCaseID %s, SubmissionID %s: Status=%s, Time=%dms, Mem=%dkB",
			tc.ID, submission.ID, result.Status, result.TimeUsedInMs, result.MemoryUsedInKb)

		// (Tùy chọn) Nếu gặp lỗi nghiêm trọng (không phải WA) thì có thể dừng chạy các test case còn lại
		if finalStatus != models.Success && finalStatus != models.WrongAnswer {
			log.Printf("Stopping further test cases for SubmissionID %s due to status: %s on TestCaseID: %s",
				submission.ID, finalStatus, tc.ID)
			// break // Bỏ comment nếu muốn dừng sớm
		}
	}

	log.Printf("Finished processing SubmissionID: %s", submission.ID)
}

// compareOutput so sánh output thực tế với output mong đợi
func (r *Runner) compareOutput(actual, expected string, settings models.SubmissionSettings) bool {
	if settings.WithTrim {
		actual = strings.TrimSpace(actual)
		expected = strings.TrimSpace(expected)
	}
	if !settings.WithCaseSensitive {
		actual = strings.ToLower(actual)
		expected = strings.ToLower(expected)
	}
	// TODO: Xử lý `settings.WithWhitespace` (ví dụ: chuẩn hóa nhiều khoảng trắng thành 1, bỏ qua dòng trống cuối...)
	// Hiện tại, so sánh chính xác sau khi trim và xử lý case.
	return actual == expected
}

// publishOverallError gửi một lỗi chung cho tất cả test cases của một submission
// (Dùng khi có lỗi ở giai đoạn chuẩn bị, trước khi chạy từng test case)
func (r *Runner) publishOverallError(submissionID string, status models.TestcaseStatus, errMsg string) {
	// Cần danh sách TestCase IDs để gửi lỗi. Nếu không có, gửi một bản tin chung.
	// Hoặc, API của bạn cần đảm bảo submission.TestCases không rỗng.
	// For now, assuming we don't have test case IDs if this function is called very early.
	// A better approach might be to have a dedicated NATS subject for submission-level errors.
	// Here, we'll just log it. The client consuming results should handle missing test case results.
	log.Printf("Publishing overall error for SubmissionID %s: Status=%s, Error=%s", submissionID, status, errMsg)
	// Nếu bạn vẫn muốn publish cho từng test case (nếu có thông tin):
	// for _, tc := range submission.TestCases { // Cần `submission` ở đây
	// 	result := models.SubmissionResult{
	// 		SubmissionID: submissionID,
	// 		TestCaseID:   tc.ID,
	// 		Status:       status,
	// 		Error:        errMsg,
	// 	}
	// 	r.natsPublisher.PublishSubmissionResult(result)
	// }
}
