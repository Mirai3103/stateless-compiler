package config

import (
	"errors"
	"log"
	"strings"

	"github.com/spf13/viper"
)

// Config chứa tất cả cấu hình cho runner-service
type Config struct {
	NATS              NATSConfig   `mapstructure:"nats"`
	Runner            RunnerConfig `mapstructure:"runner"`
	MaxConcurrentJobs int          `mapstructure:"maxConcurrentJobs"` // Số job xử lý đồng thời tối đa (sẽ cần semaphore)
	// Thêm các mục config khác ở đây, ví dụ: LogConfig
}

// NATSConfig chứa cấu hình kết nối NATS
type NATSConfig struct {
	URL                   string `mapstructure:"url"`
	SubmissionCreatedSubj string `mapstructure:"submissionCreatedSubject"`
	SubmissionResultSubj  string `mapstructure:"submissionResultSubject"`
	QueueGroup            string `mapstructure:"queueGroup"`
	// Các cài đặt nâng cao khác cho NATS client nếu cần
	// MaxReconnects int `mapstructure:"maxReconnects"`
	// ReconnectWaitSec int `mapstructure:"reconnectWaitSec"`
}

// RunnerConfig chứa cấu hình cho hoạt động của runner
type RunnerConfig struct {
	SandboxBaseDir        string `mapstructure:"sandboxBaseDir"`        // Thư mục gốc cho các sandbox tạm thời
	CompilationTimeoutSec int    `mapstructure:"compilationTimeoutSec"` // Thời gian timeout cho bước biên dịch (giây)
	MaxConcurrentJobs     int    `mapstructure:"maxConcurrentJobs"`     // Số job xử lý đồng thời tối đa (sẽ cần semaphore)
	// DefaultTimeLimitMs int `mapstructure:"defaultTimeLimitMs"` // Nếu muốn có giá trị mặc định
	// DefaultMemoryLimitKb int `mapstructure:"defaultMemoryLimitKb"`// Nếu muốn có giá trị mặc định
	SandboxType string `mapstructure:"sandboxType"` // Loại sandbox (Docker, Firejail, ...); có thể dùng để chọn runner
}

// AppConfig là biến toàn cục (hoặc được truyền đi) để giữ config đã load.
// Tốt hơn là truyền instance Config đi thay vì dùng biến toàn cục.
// var AppConfig Config

// LoadConfig đọc cấu hình từ file và environment variables.
func LoadConfig(configPaths ...string) (*Config, error) {
	v := viper.New()

	// 1. Đặt tên file config (không có đuôi file)
	v.SetConfigName("config") // Ví dụ: config.yaml, config.json, config.toml

	// 2. Đặt loại file config
	v.SetConfigType("yaml") // Hoặc json, toml, ...

	// 3. Thêm các đường dẫn để Viper tìm file config
	// Viper sẽ tìm theo thứ tự bạn thêm vào.
	if len(configPaths) > 0 {
		for _, path := range configPaths {
			v.AddConfigPath(path)
		}
	}
	v.AddConfigPath("./configs")            // Thư mục chứa file config trong project
	v.AddConfigPath(".")                    // Tìm ở thư mục hiện tại
	v.AddConfigPath("/etc/runner-service/") // Đường dẫn trên server production (nếu có)
	// Thêm các đường dẫn khác nếu cần

	// 4. Đọc các biến môi trường (quan trọng để override)
	v.AutomaticEnv() // Tự động đọc các biến môi trường khớp với key config

	// 5. Đặt tiền tố cho biến môi trường (tùy chọn nhưng nên có)
	// Ví dụ: RUNNER_NATS_URL sẽ map tới config.NATS.URL
	v.SetEnvPrefix("RUNNER")

	// 6. Xử lý các key có dấu chấm khi map từ biến môi trường
	// Ví dụ: nats.url -> NATS_URL (nếu không có prefix)
	// Hoặc runner.nats.url -> RUNNER_NATS_URL (nếu có prefix RUNNER)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// 7. Đặt giá trị mặc định (tùy chọn)
	// Ví dụ:
	v.SetDefault("nats.url", "nats://localhost:4222")
	v.SetDefault("nats.submissionCreatedSubject", "submission.created")
	v.SetDefault("nats.submissionResultSubject", "submission.result")
	// v.SetDefault("nats.maxReconnects", 5)
	v.SetDefault("nats.queueGroup", "runner-service-group")
	v.SetDefault("runner.sandboxBaseDir", "/tmp/runner_sandbox")
	v.SetDefault("runner.compilationTimeoutSec", 30)
	v.SetDefault("runner.sandboxType", "direct") // Hoặc "firejail", "docker", ...
	v.SetDefault("runner.maxConcurrentJobs", 100)

	// 8. Đọc file config
	if err := v.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if errors.As(err, &configFileNotFoundError) {
			// File config không tìm thấy; không sao nếu có giá trị mặc định hoặc biến môi trường
			log.Println("Config file not found; using defaults and environment variables.")
		}
	} else {
		log.Printf("Using config file: %s", v.ConfigFileUsed())
	}

	// 9. Unmarshal config vào struct
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		log.Printf("Error unmarshalling config: %s", err)
		return nil, err
	}

	// Gán vào biến toàn cục nếu bạn muốn (không khuyến khích bằng dependency injection)
	// AppConfig = cfg

	log.Printf("Configuration loaded successfully: %+v", cfg)
	return &cfg, nil
}
