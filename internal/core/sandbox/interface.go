package sandbox

import (
	"context"
	"github.com/Mirai3103/remote-compiler/internal/config"

	"github.com/Mirai3103/remote-compiler/internal/models"
)

type Type string

const (
	DirectSandbox   Type = "direct"   // Không sử dụng sandbox, chạy trực tiếp
	FirejailSandbox Type = "firejail" // Sử dụng firejail để cách ly
	IsolateSandbox  Type = "isolate"  // Sử dụng isolate để cách ly
)

// RunRequest chứa thông tin cần thiết để *chạy* một chương trình đã được chuẩn bị
// (ví dụ: đã biên dịch, hoặc là một script).
type RunRequest struct {
	SubmissionID     string   // ID của submission chung
	TestCaseID       string   // ID của test case cụ thể đang chạy
	RunCommand       []string // Lệnh và các tham số để thực thi (ví dụ: ["./a.out"] hoặc ["python", "main.py"])
	WorkingDirectory string   // Thư mục làm việc nơi lệnh sẽ được thực thi (thường là thư mục tạm chứa file thực thi/script)
	Input            string   // Dữ liệu đầu vào cho test case
	TimeLimitMs      int      // Giới hạn thời gian chạy (milliseconds)
	MemoryLimitKb    int      // Giới hạn bộ nhớ (kilobytes)
	// Có thể thêm các thông tin khác như Environment Variables nếu cần
	// EnvVars          map[string]string
}

// ExecuteResult chứa kết quả sau khi thực thi code.
// Lưu ý: Status ở đây sẽ không bao gồm CompileError, vì việc biên dịch
// được xử lý ở bên ngoài Executor.
type ExecuteResult struct {
	Status       models.TestcaseStatus // Trạng thái (Success, RuntimeError, TimeLimitExceeded, MemoryLimitExceeded, WrongAnswer, etc.)
	Stdout       string                // Output chuẩn
	Stderr       string                // Output lỗi chuẩn (từ quá trình chạy, không phải lỗi biên dịch)
	ExitCode     int                   // Mã thoát của tiến trình
	TimeUsedMs   int                   // Thời gian thực thi (milliseconds)
	MemoryUsedKb int                   // Bộ nhớ sử dụng (kilobytes)
	// SandboxError   string             // (Tùy chọn) Lỗi từ chính sandbox nếu có, phân biệt với lỗi của code người dùng
}

// Executor là interface chung cho các môi trường thực thi code đã được chuẩn bị.
type Executor interface {
	// Execute chạy lệnh được cung cấp trong RunRequest bên trong môi trường sandbox.
	// Nó không chịu trách nhiệm biên dịch.
	Execute(ctx context.Context, req RunRequest) (*ExecuteResult, error)

	// ID trả về một định danh cho loại executor này (ví dụ: "direct", "firejail_v1").
	ID() string
}

func NewExecutor(rc config.RunnerConfig) Executor {
	switch rc.SandboxType {
	case string(DirectSandbox):
		return &directExecutor{
			cfg: rc,
		}
	case string(FirejailSandbox):
		return nil
	case string(IsolateSandbox):
		return nil
	default:
		return nil
	}
}
