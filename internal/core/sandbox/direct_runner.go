package sandbox

import (
	"bytes"
	"context"
	"fmt" // Thêm vào để format lỗi memory
	"log"
	"os/exec"
	"strings"
	"sync/atomic" // Sử dụng cho maxMemUsage
	"syscall"
	"time"

	"github.com/Mirai3103/remote-compiler/internal/config" // Giữ nguyên config của bạn
	"github.com/Mirai3103/remote-compiler/internal/models" // Điều chỉnh import path nếu cần
	"github.com/shirou/gopsutil/v3/process"                // Thêm thư viện gopsutil
)

const (
	memoryPollInterval = 20 * time.Millisecond // Tần suất kiểm tra bộ nhớ
)

// DirectExecutor thực thi code trực tiếp trên host.
// CẢNH BÁO: Không an toàn cho code không đáng tin cậy.
type directExecutor struct {
	cfg config.RunnerConfig
}

// ID trả về định danh cho executor này.
func (e *directExecutor) ID() string {
	return "direct_executor_v1_mem_monitored" // Cập nhật ID nếu muốn
}

// Execute chạy lệnh được cung cấp trực tiếp trên host, có theo dõi bộ nhớ.
func (e *directExecutor) Execute(ctx context.Context, req RunRequest) (*ExecuteResult, error) {
	log.Printf("[%s] DirectExecute: Starting execution for SubmissionID: %s, TestCaseID: %s, Command: %v, TimeLimit: %dms, MemoryLimit: %dkB",
		e.ID(), req.SubmissionID, req.TestCaseID, req.RunCommand, req.TimeLimitMs, req.MemoryLimitKb)

	cmd := exec.CommandContext(ctx, req.RunCommand[0], req.RunCommand[1:]...)
	cmd.Dir = req.WorkingDirectory
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Stdin = strings.NewReader(req.Input)

	startTime := time.Now()
	var execErr error
	var exitCode int
	status := models.Running // Trạng thái ban đầu

	if err := cmd.Start(); err != nil {
		log.Printf("[%s] DirectExecute: Failed to start command for TestCaseID %s: %v", e.ID(), req.TestCaseID, err)
		return nil, &Error{
			Type:    ErrCmdStart,
			Message: "failed to start command",
			Cause:   err,
		}
	}

	pid := int32(cmd.Process.Pid)
	log.Printf("[%s] DirectExecute: Process started for TestCaseID %s with PID %d", e.ID(), req.TestCaseID, pid)

	errChan := make(chan error, 1)
	go func() {
		errChan <- cmd.Wait()
	}()

	var maxMemUsageAtomic uint64 // Dùng atomic để an toàn với goroutine
	memoryLimitExceeded := false
	memoryMonitorCtx, memoryMonitorCancel := context.WithCancel(context.Background())
	defer memoryMonitorCancel() // Đảm bảo goroutine theo dõi bộ nhớ được dừng

	// Goroutine theo dõi bộ nhớ
	go func() {
		ticker := time.NewTicker(memoryPollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-memoryMonitorCtx.Done():
				log.Printf("[%s] DirectExecute: Memory monitor stopped for TestCaseID %s, PID %d.", e.ID(), req.TestCaseID, pid)
				return
			case <-ticker.C:
				proc, err := process.NewProcess(pid)
				if err != nil {
					// Process có thể đã kết thúc, hoặc có lỗi tạm thời khi lấy process
					// log.Printf("[%s] DirectExecute: Failed to get process %d for memory check (may have exited): %v", e.ID(), pid, err)
					continue
				}
				memInfo, err := proc.MemoryInfo()
				if err != nil {
					// log.Printf("[%s] DirectExecute: Failed to get memory info for process %d: %v", e.ID(), pid, err)
					continue
				}

				currentMem := memInfo.RSS
				if currentMem > atomic.LoadUint64(&maxMemUsageAtomic) {
					atomic.StoreUint64(&maxMemUsageAtomic, currentMem)
				}

				// Kiểm tra giới hạn bộ nhớ (nếu có)
				if req.MemoryLimitKb > 0 && (currentMem/1024) > uint64(req.MemoryLimitKb) {
					memoryLimitExceeded = true
					log.Printf("[%s] DirectExecute: Memory limit exceeded for TestCaseID %s, PID %d. Usage: %d KB, Limit: %d KB",
						e.ID(), req.TestCaseID, pid, currentMem/1024, req.MemoryLimitKb)
					memoryMonitorCancel() // Dừng các lần kiểm tra tiếp theo
					if cmd.Process != nil {
						if killErr := cmd.Process.Kill(); killErr != nil {
							log.Printf("[%s] DirectExecute: Failed to kill process %d for TestCaseID %s due to MLE: %v", e.ID(), pid, req.TestCaseID, killErr)
						} else {
							log.Printf("[%s] DirectExecute: Process %d for TestCaseID %s killed due to MLE.", e.ID(), pid, req.TestCaseID)
						}
					}
					return // Thoát khỏi goroutine theo dõi bộ nhớ
				}
			}
		}
	}()

	select {
	case err := <-errChan: // cmd.Wait() hoàn thành
		execErr = err
		// Đảm bảo memory monitor đã dừng nếu process kết thúc trước khi monitor bị cancel bởi timeout/MLE
		memoryMonitorCancel()

		if execErr != nil {
			if exitErr, ok := execErr.(*exec.ExitError); ok {
				if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok {
					exitCode = ws.ExitStatus()
				} else {
					exitCode = -1 // Không lấy được exit status cụ thể trên một số OS/trường hợp
					log.Printf("[%s] DirectExecute: Could not get WaitStatus for TestCaseID %s", e.ID(), req.TestCaseID)
				}
				// Kiểm tra xem có phải bị kill do MLE không (memoryLimitExceeded sẽ true)
				if memoryLimitExceeded {
					status = models.MemoryLimitExceeded
					log.Printf("[%s] DirectExecute: Command for TestCaseID %s confirmed MLE. Exit code: %d. Stderr: %s", e.ID(), req.TestCaseID, exitCode, stderr.String())
				} else {
					status = models.RuntimeError
					log.Printf("[%s] DirectExecute: Command for TestCaseID %s exited with code %d. Stderr: %s", e.ID(), req.TestCaseID, exitCode, stderr.String())
				}
			} else {
				// Lỗi khác không phải ExitError (ví dụ: không tìm thấy command, hoặc bị kill bởi MLE nhưng Wait trả về lỗi khác)
				if memoryLimitExceeded { // Ưu tiên MLE nếu flag này được set
					status = models.MemoryLimitExceeded
					exitCode = -1 // Hoặc mã đặc trưng cho MLE kill
					log.Printf("[%s] DirectExecute: Command for TestCaseID %s failed (likely MLE, non-ExitError wait): %v. Stderr: %s", e.ID(), req.TestCaseID, execErr, stderr.String())
				} else {
					log.Printf("[%s] DirectExecute: cmd.Wait() error for TestCaseID %s (not ExitError): %v", e.ID(), req.TestCaseID, execErr)
					return nil, &Error{
						Type:    ErrCmdWait,
						Message: "command wait failed with unexpected error",
						Cause:   execErr,
					}
				}
			}
		} else {
			// Lệnh chạy thành công (exit code 0)
			memoryMonitorCancel() // Đảm bảo dừng nếu chưa dừng
			exitCode = 0
			if memoryLimitExceeded { // Vẫn có thể bị set nếu MLE xảy ra rất sát lúc kết thúc
				status = models.MemoryLimitExceeded
			} else {
				status = models.Success
			}
			log.Printf("[%s] DirectExecute: Command for TestCaseID %s completed. ExitCode: 0. Final status: %s", e.ID(), req.TestCaseID, status)
		}

	case <-ctx.Done(): // Context bị hủy (thường là do timeout từ runner)
		memoryMonitorCancel() // Dừng goroutine theo dõi bộ nhớ

		// Cố gắng kill tiến trình nếu nó vẫn đang chạy
		if cmd.Process != nil {
			if err := cmd.Process.Kill(); err != nil {
				log.Printf("[%s] DirectExecute: Failed to kill process %d for TestCaseID %s on timeout: %v", e.ID(), pid, req.TestCaseID, err)
			} else {
				log.Printf("[%s] DirectExecute: Process %d for TestCaseID %s killed due to context cancellation.", e.ID(), pid, req.TestCaseID)
			}
		}
		// Chờ Wait() trả về sau khi kill
		execErr = <-errChan // Đọc lỗi từ cmd.Wait (thường là "signal: killed")

		log.Printf("[%s] DirectExecute: Context cancelled for TestCaseID %s (likely timeout). Error from Wait: %v", e.ID(), req.TestCaseID, execErr)
		status = models.TimeLimitExceeded
		exitCode = -1 // Hoặc một mã đặc biệt cho TLE
	}

	timeUsedMs := int(time.Since(startTime).Milliseconds())
	finalMaxMemUsageKb := int(atomic.LoadUint64(&maxMemUsageAtomic) / 1024)

	// Xử lý trạng thái cuối cùng, ưu tiên MLE, sau đó TLE
	if memoryLimitExceeded { // Đã được set bởi memory monitor hoặc kiểm tra lại
		status = models.MemoryLimitExceeded
		log.Printf("[%s] DirectExecute: Final status for TestCaseID %s is MLE. Mem: %dkB, Time: %dms",
			e.ID(), req.TestCaseID, finalMaxMemUsageKb, timeUsedMs)
	} else if status == models.TimeLimitExceeded { // Đã được set bởi context timeout
		// Giữ nguyên TLE, không cần làm gì thêm
		log.Printf("[%s] DirectExecute: Final status for TestCaseID %s is TLE. Mem: %dkB, Time: %dms",
			e.ID(), req.TestCaseID, finalMaxMemUsageKb, timeUsedMs)
	} else if req.TimeLimitMs > 0 && timeUsedMs > req.TimeLimitMs {
		// Kiểm tra TLE dựa trên thời gian đo được, nếu context timeout có thể chưa đủ chính xác
		log.Printf("[%s] DirectExecute: Execution time %dms exceeded limit %dms for TestCaseID %s, overriding status to TLE.",
			e.ID(), timeUsedMs, req.TimeLimitMs, req.TestCaseID)
		status = models.TimeLimitExceeded
	}
	// Nếu không phải MLE, TLE, thì status đã là Success hoặc RuntimeError từ trước.

	result := &ExecuteResult{
		Status:       status,
		Stdout:       stdout.String(),
		Stderr:       stderr.String(),
		ExitCode:     exitCode,
		TimeUsedMs:   timeUsedMs,
		MemoryUsedKb: finalMaxMemUsageKb, // Sử dụng giá trị đã đo được
		// ErrorMsg sẽ chứa thông tin lỗi chi tiết hơn nếu cần (ví dụ, stderr)

	}
	if status != models.Success {
		// Có thể gán ErrorMsg từ stderr hoặc một thông báo lỗi cụ thể hơn
		// Ví dụ: result.ErrorMsg = stderr.String() khi là RE, MLE, TLE
	}

	log.Printf("[%s] DirectExecute: Finished execution for TestCaseID %s. Status: %s, Time: %dms, Mem: %dkB",
		e.ID(), req.TestCaseID, result.Status, result.TimeUsedMs, result.MemoryUsedKb)

	return result, nil
}

// ErrorType, Error struct giữ nguyên như bạn đã định nghĩa
type ErrorType string

const (
	ErrCmdStart ErrorType = "COMMAND_START_ERROR"
	ErrCmdWait  ErrorType = "COMMAND_WAIT_ERROR"
	ErrInternal ErrorType = "INTERNAL_SANDBOX_ERROR"
)

type Error struct {
	Type    ErrorType
	Message string
	Cause   error
	Details string
}

func (se *Error) Error() string {
	if se.Cause != nil {
		return fmt.Sprintf("%s: %s (type: %s)", se.Message, se.Cause.Error(), se.Type)
	}
	return fmt.Sprintf("%s (type: %s)", se.Message, se.Type)
}

func (se *Error) Unwrap() error {
	return se.Cause
}
