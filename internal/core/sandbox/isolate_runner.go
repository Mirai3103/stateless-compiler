package sandbox

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"github.com/Mirai3103/remote-compiler/internal/config"
	"github.com/Mirai3103/remote-compiler/internal/models"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

const (
	// DefaultIsolatePath is the default path to the isolate executable.
	DefaultIsolatePath = "isolate"
	// DefaultEnvPath is the default PATH environment variable for sandboxed processes.
	DefaultEnvPath = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
	// DefaultFsizeKb is the default file size limit in KB.
	DefaultFsizeKb = 65536 // 64 MB
	// DefaultProcesses is the default maximum number of processes.
	DefaultProcesses = 64
	// DefaultExtraTimeSeconds is the default extra time before killing a TLE process.
	DefaultExtraTimeSeconds = 2.0
	// DefaultWallTimeFactor is the factor to multiply CPU time limit for wall time limit.
	DefaultWallTimeFactor = 2.0
	// DefaultBoxCleanupTimeout is the timeout for the isolate --cleanup command.
	DefaultBoxCleanupTimeout = 5 * time.Second
)

// IsolateExecutorConfig holds configuration specific to the IsolateExecutor.
type IsolateExecutorConfig struct {
	IsolatePath      string  `mapstructure:"isolatePath"`
	EnvPath          string  `mapstructure:"envPath"`
	DefaultFsizeKb   int     `mapstructure:"defaultFsizeKb"`
	DefaultProcesses int     `mapstructure:"defaultProcesses"`
	ExtraTimeSeconds float64 `mapstructure:"extraTimeSeconds"`
	WallTimeFactor   float64 `mapstructure:"wallTimeFactor"`
	// TempDir is a directory on the host for temporary files like stdin, stdout, meta.
	// If empty, os.TempDir() will be used.
	TempDir string `mapstructure:"tempDir"`
}

// IsolateExecutor implements the sandbox.Executor interface using the 'isolate' tool.
type IsolateExecutor struct {
	config       IsolateExecutorConfig
	runnerConfig config.RunnerConfig // General runner config for SandboxBaseDir etc.
	boxIDCounter atomic.Uint32
}

// NewIsolateExecutor creates a new IsolateExecutor.
// It's assumed that the relevant IsolateExecutorConfig is part of the global RunnerConfig
// or passed appropriately. For this example, we'll assume it's nested or accessible.
func NewIsolateExecutor(runnerCfg config.RunnerConfig, isolateCfg IsolateExecutorConfig) *IsolateExecutor {
	if isolateCfg.IsolatePath == "" {
		isolateCfg.IsolatePath = DefaultIsolatePath
	}
	if isolateCfg.EnvPath == "" {
		isolateCfg.EnvPath = DefaultEnvPath
	}
	if isolateCfg.DefaultFsizeKb == 0 {
		isolateCfg.DefaultFsizeKb = DefaultFsizeKb
	}
	if isolateCfg.DefaultProcesses == 0 {
		isolateCfg.DefaultProcesses = DefaultProcesses
	}
	if isolateCfg.ExtraTimeSeconds == 0 {
		isolateCfg.ExtraTimeSeconds = DefaultExtraTimeSeconds
	}
	if isolateCfg.WallTimeFactor == 0 {
		isolateCfg.WallTimeFactor = DefaultWallTimeFactor
	}

	return &IsolateExecutor{
		config:       isolateCfg,
		runnerConfig: runnerCfg,
	}
}

// ID returns the identifier for this executor.
func (e *IsolateExecutor) ID() string {
	return "isolate_executor_v1"
}

// Execute runs the command specified in RunRequest within an isolate sandbox.
func (e *IsolateExecutor) Execute(ctx context.Context, req RunRequest) (*ExecuteResult, error) {
	boxID := e.boxIDCounter.Add(1)
	boxIDStr := fmt.Sprintf("%d", boxID)

	log.Printf("[%s] BoxID %s: Starting execution for SubmissionID: %s, TestCaseID: %s",
		e.ID(), boxIDStr, req.SubmissionID, req.TestCaseID)

	// 1. Prepare temporary host files for stdin, stdout, stderr, meta
	tempFileHostDir := e.config.TempDir
	if tempFileHostDir == "" {
		tempFileHostDir = os.TempDir()
	}

	stdinFile, err := os.CreateTemp(tempFileHostDir, fmt.Sprintf("isolate_%s_stdin_*.txt", boxIDStr))
	if err != nil {
		return nil, &Error{Type: ErrInternal, Message: "failed to create stdin temp file", Cause: err}
	}
	defer os.Remove(stdinFile.Name())
	if _, err := stdinFile.WriteString(req.Input); err != nil {
		stdinFile.Close()
		return nil, &Error{Type: ErrInternal, Message: "failed to write to stdin temp file", Cause: err}
	}
	stdinFile.Close()

	stdoutFilePath := filepath.Join(tempFileHostDir, fmt.Sprintf("isolate_%s_stdout.txt", boxIDStr))
	stderrFilePath := filepath.Join(tempFileHostDir, fmt.Sprintf("isolate_%s_stderr.txt", boxIDStr))
	metaFilePath := filepath.Join(tempFileHostDir, fmt.Sprintf("isolate_%s_meta.txt", boxIDStr))

	defer os.Remove(stdoutFilePath)
	defer os.Remove(stderrFilePath)
	defer os.Remove(metaFilePath)

	// 2. Isolate init
	initArgs := []string{"--box-id=" + boxIDStr, "--cg", "--init"}
	initCmd := exec.Command(e.config.IsolatePath, initArgs...)
	log.Printf("[%s] BoxID %s: Initializing sandbox: %s %v", e.ID(), boxIDStr, e.config.IsolatePath, initArgs)
	if output, err := initCmd.CombinedOutput(); err != nil {
		log.Printf("[%s] BoxID %s: Isolate init failed. Output: %s", e.ID(), boxIDStr, string(output))
		return nil, &Error{Type: ErrInternal, Message: "isolate init failed", Cause: err, Details: string(output)}
	}

	// 3. Defer Isolate cleanup
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), DefaultBoxCleanupTimeout)
		defer cancel()
		cleanupArgs := []string{"--box-id=" + boxIDStr, "--cleanup"}
		cleanupCmd := exec.CommandContext(cleanupCtx, e.config.IsolatePath, cleanupArgs...)
		log.Printf("[%s] BoxID %s: Cleaning up sandbox: %s %v", e.ID(), boxIDStr, e.config.IsolatePath, cleanupArgs)
		if output, err := cleanupCmd.CombinedOutput(); err != nil {
			// Log cleanup error, but don't override original execution error
			log.Printf("[%s] BoxID %s: Isolate cleanup failed. Output: %s, Error: %v", e.ID(), boxIDStr, string(output), err)
		} else {
			log.Printf("[%s] BoxID %s: Sandbox cleanup successful.", e.ID(), boxIDStr)
		}
	}()

	// 4. Construct isolate run command arguments
	runArgs := []string{"--box-id=" + boxIDStr, "--cg"}
	runArgs = append(runArgs, "--cg-mem="+fmt.Sprintf("%d", req.MemoryLimitKb)) // Memory limit in KB
	timeLimitSec := float64(req.TimeLimitMs) / 1000.0
	runArgs = append(runArgs, "--time="+fmt.Sprintf("%.3f", timeLimitSec)) // CPU time limit in seconds

	wallTimeLimitSec := timeLimitSec * e.config.WallTimeFactor
	if wallTimeLimitSec < timeLimitSec+e.config.ExtraTimeSeconds { // ensure wall-time is reasonably larger
		wallTimeLimitSec = timeLimitSec + e.config.ExtraTimeSeconds + 1.0
	}
	runArgs = append(runArgs, "--wall-time="+fmt.Sprintf("%.3f", wallTimeLimitSec))
	runArgs = append(runArgs, "--extra-time="+fmt.Sprintf("%.3f", e.config.ExtraTimeSeconds))

	fsizeKb := e.config.DefaultFsizeKb
	if req.MemoryLimitKb > 0 && req.MemoryLimitKb*2 > fsizeKb { // Heuristic for fsize based on mem limit
		// fsizeKb = req.MemoryLimitKb * 2 // Allow files up to 2x memory
	}
	runArgs = append(runArgs, "--fsize="+fmt.Sprintf("%d", fsizeKb))

	runArgs = append(runArgs, "--stdin="+stdinFile.Name())
	runArgs = append(runArgs, "--stdout="+stdoutFilePath)
	runArgs = append(runArgs, "--stderr="+stderrFilePath)
	runArgs = append(runArgs, "--meta="+metaFilePath)

	// Mount the host's working directory (containing the code) to /box inside the sandbox.
	// Isolate executes commands with /box as the default current directory.
	runArgs = append(runArgs, "--dir="+req.WorkingDirectory+":/box:rw") // :rw for compilation/output if needed
	// For compiled languages where req.WorkingDirectory might contain only the binary and it's read-only:
	// runArgs = append(runArgs, "--dir="+req.WorkingDirectory+":/box:ro")

	runArgs = append(runArgs, "--env=PATH="+e.config.EnvPath)
	// Add any other environment variables from req.EnvVars if that feature is added

	processes := e.config.DefaultProcesses
	runArgs = append(runArgs, fmt.Sprintf("--processes=%d", processes))

	runArgs = append(runArgs, "--run", "--")
	runArgs = append(runArgs, req.RunCommand...)

	// 5. Execute isolate run command
	log.Printf("[%s] BoxID %s: Running command in sandbox: %s %v", e.ID(), boxIDStr, e.config.IsolatePath, runArgs)
	cmdRun := exec.CommandContext(ctx, e.config.IsolatePath, runArgs...)
	runErr := cmdRun.Run() // This error is often non-nil for non-zero exit, TLE, etc.
	// We primarily rely on the meta file for status.

	// If context was cancelled, cmdRun.Run() might return an error related to that.
	// Isolate should ideally detect the timeout itself and write to meta.
	if errors.Is(ctx.Err(), context.DeadlineExceeded) && runErr != nil {
		log.Printf("[%s] BoxID %s: Context deadline exceeded during run. Isolate error: %v", e.ID(), boxIDStr, runErr)
		// Meta file should ideally reflect "TO" status from isolate due to wall-time or extra-time.
	} else if runErr != nil {
		log.Printf("[%s] BoxID %s: Isolate run command finished with error (may be expected for non-zero exit/signal): %v", e.ID(), boxIDStr, runErr)
	}

	// 6. Read stdout, stderr
	stdoutBytes, err := os.ReadFile(stdoutFilePath)
	if err != nil && !os.IsNotExist(err) {
		log.Printf("[%s] BoxID %s: Error reading stdout file %s: %v", e.ID(), boxIDStr, stdoutFilePath, err)
	}
	stderrBytes, err := os.ReadFile(stderrFilePath)
	if err != nil && !os.IsNotExist(err) {
		log.Printf("[%s] BoxID %s: Error reading stderr file %s: %v", e.ID(), boxIDStr, stderrFilePath, err)
	}

	// 7. Parse meta file
	meta, parseMetaErr := parseIsolateMetaFile(metaFilePath)
	if parseMetaErr != nil {
		log.Printf("[%s] BoxID %s: Error parsing meta file %s: %v", e.ID(), boxIDStr, metaFilePath, parseMetaErr)
		// If meta file is crucial and unparseable, this could be an internal error.
		// However, if context timed out, meta might not be fully written.
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return &ExecuteResult{
				Status: models.TimeLimitExceeded, // Assume TLE if context timed out and meta is bad
				Stdout: string(stdoutBytes),
				Stderr: string(stderrBytes),
			}, nil
		}
		return nil, &Error{Type: ErrInternal, Message: "failed to parse isolate meta file", Cause: parseMetaErr}
	}
	log.Printf("[%s] BoxID %s: Parsed meta: %+v", e.ID(), boxIDStr, meta)

	// 8. Determine final status and result
	result := &ExecuteResult{
		Stdout:       string(stdoutBytes),
		Stderr:       string(stderrBytes),
		ExitCode:     meta.ExitCode,
		TimeUsedMs:   int(meta.TimeSeconds * 1000), // Isolate time is CPU time
		MemoryUsedKb: meta.CGMemKB,                 // CGMem is peak cgroup memory
	}

	// Check for explicit context timeout first, as isolate might not always write "TO"
	// if killed externally by the Go context before its own wall-time/extra-time handling.
	if ctx.Err() == context.DeadlineExceeded {
		result.Status = models.TimeLimitExceeded
		log.Printf("[%s] BoxID %s: Final status determined by context deadline: TLE", e.ID(), boxIDStr)
	} else {
		// Determine status based on meta file
		switch meta.Status {
		case "TO":
			result.Status = models.TimeLimitExceeded
		case "SG", "RE":
			// Check OOM killer first
			if meta.CGOOMKilled > 0 || (req.MemoryLimitKb > 0 && meta.CGMemKB > 0 && meta.CGMemKB > req.MemoryLimitKb) {
				result.Status = models.MemoryLimitExceeded
			} else if meta.ExitCode != 0 {
				result.Status = models.RuntimeError
			} else {
				// Exited 0 despite signal/RE status - unusual, treat as RE
				result.Status = models.RuntimeError
				if result.Stderr == "" {
					result.Stderr = fmt.Sprintf("Exited with status %s but exit code 0.", meta.Status)
				}
			}
		case "XX":
			log.Printf("[%s] BoxID %s: Isolate internal error: %s", e.ID(), boxIDStr, meta.Message)
			return nil, &Error{Type: ErrInternal, Message: "isolate internal error: " + meta.Message}
		default: // Includes empty status, which typically means success if exitcode is 0
			if meta.ExitCode == 0 {
				// Still check memory for success cases
				if meta.CGOOMKilled > 0 || (req.MemoryLimitKb > 0 && meta.CGMemKB > 0 && meta.CGMemKB > req.MemoryLimitKb) {
					result.Status = models.MemoryLimitExceeded
				} else {
					result.Status = models.Success
				}
			} else {
				// Fallback to RuntimeError if ExitCode is non-zero and no other specific status
				result.Status = models.RuntimeError
			}
		}
	}

	// Final check: if time from meta exceeds limit significantly (e.g. due to extra-time)
	// and status isn't already TLE, mark as TLE. Isolate's `time` is CPU time.
	if result.Status != models.TimeLimitExceeded && float64(result.TimeUsedMs) > float64(req.TimeLimitMs)*1.05 { // 5% buffer
		// This condition might be hit if isolate's TLE detection (based on `time` + `extra-time`)
		// didn't trigger but the reported CPU time is over the soft limit.
		// Or if wall-time was hit but isolate reported it differently.
		log.Printf("[%s] BoxID %s: CPU Time %dms exceeded soft limit %dms. Marking TLE.",
			e.ID(), boxIDStr, result.TimeUsedMs, req.TimeLimitMs)
		// result.Status = models.TimeLimitExceeded // Be careful with overriding like this.
	}

	log.Printf("[%s] BoxID %s: Finished execution. Status: %s, Time: %dms, Mem: %dkB",
		e.ID(), boxIDStr, result.Status, result.TimeUsedMs, result.MemoryUsedKb)
	return result, nil
}

// isolateMeta holds parsed data from the isolate --meta file.
type isolateMeta struct {
	TimeSeconds  float64 // CPU time
	TimeWall     float64 // Wall-clock time
	MaxRSS       int     // Max RSS (KB) - less useful with cgroups
	CGMemKB      int     // Peak CGroup memory usage (KB)
	CGOOMKilled  int     // Whether CGroup OOM killer was invoked (0 or 1)
	ExitCode     int
	Status       string // e.g., TO, RE, SG, XX
	Message      string
	CSWVoluntary int
	CSWForced    int
}

func parseIsolateMetaFile(filePath string) (*isolateMeta, error) {
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Meta file might not exist if isolate failed very early or was killed before writing it
			log.Printf("Meta file %s does not exist. Returning empty meta.", filePath)
			return &isolateMeta{Status: "XX", Message: "Meta file not found"}, nil // Treat as internal error
		}
		return nil, fmt.Errorf("failed to open meta file %s: %w", filePath, err)
	}
	defer file.Close()

	meta := &isolateMeta{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue // Skip malformed lines
		}
		key, value := parts[0], parts[1]
		switch key {
		case "time":
			meta.TimeSeconds, _ = strconv.ParseFloat(value, 64)
		case "time-wall":
			meta.TimeWall, _ = strconv.ParseFloat(value, 64)
		case "max-rss": // This is less reliable with cgroups
			meta.MaxRSS, _ = strconv.Atoi(value)
		case "cg-mem": // Peak memory usage in bytes by cgroup, convert to KB
			cgMemBytes, _ := strconv.Atoi(value)
			meta.CGMemKB = (cgMemBytes + 1023) / 1024 // Round up to nearest KB
		case "cg-oom-killed":
			meta.CGOOMKilled, _ = strconv.Atoi(value)
		case "exitcode":
			meta.ExitCode, _ = strconv.Atoi(value)
		case "status":
			meta.Status = value
		case "message":
			meta.Message = value
		case "csw-voluntary":
			meta.CSWVoluntary, _ = strconv.Atoi(value)
		case "csw-forced":
			meta.CSWForced, _ = strconv.Atoi(value)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading meta file %s: %w", filePath, err)
	}
	return meta, nil
}

// You would then update the sandbox.NewExecutor factory function:
// In internal/core/sandbox/interface.go
/*
func NewExecutor(runnerCfg config.RunnerConfig, allAppCfgFull *appconfig.Config) Executor { // Example: pass full config if isolate config is nested
	switch runnerCfg.SandboxType {
	case string(DirectSandbox):
		return &directExecutor{
			cfg: runnerCfg,
		}
	case string(IsolateSandbox):
		// Assuming IsolateExecutorConfig is part of your main appConfig structure
		// For example, if allAppCfgFull.Isolate is your IsolateExecutorConfig
		isolateSpecificCfg := IsolateExecutorConfig{ // Populate this from your actual app config
				IsolatePath:      allAppCfgFull.GetString("runner.isolate.path"), // Fictional config structure
				EnvPath:          allAppCfgFull.GetString("runner.isolate.envPath"),
				DefaultFsizeKb:   allAppCfgFull.GetInt("runner.isolate.fsizeKb"),
				// ... and so on
				TempDir:          allAppCfgFull.GetString("runner.isolate.tempDir"),
		}
		// Or, more directly if RunnerConfig contains IsolateExecutorConfig:
		// isolateSpecificCfg := runnerCfg.IsolateConfig // If you add such a field
		return NewIsolateExecutor(runnerCfg, isolateSpecificCfg)

	// case string(FirejailSandbox):
	//	 return nil // To be implemented
	default:
		log.Printf("Unsupported sandbox type: %s, falling back to direct executor or nil", runnerCfg.SandboxType)
		// Fallback or error:
		// return &directExecutor{cfg: runnerCfg} // Or handle error appropriately
		return nil
	}
}
*/
