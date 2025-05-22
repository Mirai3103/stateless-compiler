package models

type TestcaseStatus string

const (
	Success             TestcaseStatus = "success"
	CompileError        TestcaseStatus = "compile_error"
	RuntimeError        TestcaseStatus = "runtime_error"
	WrongAnswer         TestcaseStatus = "wrong_answer"
	TimeLimitExceeded   TestcaseStatus = "time_limit_exceeded"
	MemoryLimitExceeded TestcaseStatus = "memory_limit_exceeded"
	Running             TestcaseStatus = "running"
	None                TestcaseStatus = "none"
)
const InternalError = ""

type Submission struct {
	ID              string             `json:"id"`
	Language        Language           `json:"language"`
	Code            string             `json:"code"`
	TimeLimitInMs   int                `json:"timeLimitInMs"`
	MemoryLimitInKb int                `json:"memoryLimitInKb"`
	TestCases       []TestCase         `json:"testCases"`
	Settings        SubmissionSettings `json:"settings"`
}

type SubmissionSettings struct {
	WithTrim          bool `json:"with_trim"`
	WithCaseSensitive bool `json:"with_case_sensitive"`
	WithWhitespace    bool `json:"with_whitespace"`
}

type Language struct {
	ID             string `json:"id"`
	SourceFile     string ` json:"sourceFile"`
	BinaryFile     string `json:"binaryFile"`
	CompileCommand string `json:"compileCommand"`
	RunCommand     string `json:"runCommand"`
}

type TestCase struct {
	ID           string `json:"id"`
	Input        string `json:"input"`
	ExpectOutput string `json:"expectOutput"`
}

type SubmissionResult struct {
	SubmissionID   string         `json:"submission_id"`
	TestCaseID     string         `json:"test_case_id"`
	Status         TestcaseStatus `json:"status"`
	TimeUsedInMs   int            `json:"time_used_in_ms"`
	MemoryUsedInKb int            `json:"memory_used_in_kb"`
	Output         string         `json:"output"`
	Error          string         `json:"error"`
}
