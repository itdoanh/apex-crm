package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Port                      string
	DatabaseURL               string
	DBHost                    string
	DBPort                    string
	DBUser                    string
	DBPassword                string
	DBName                    string
	DBSSLMode                 string
	AllowOrigins              string
	AdminKey                  string
	GoogleServiceAccountJSON  string
	GoogleServiceAccountEmail string
	GoogleServiceAccountFile  string
}

// loadDotEnv tự động đọc file .env nếu có để nạp biến môi trường cho máy chủ
func loadDotEnv() {
	candidates := []string{
		".env",
		filepath.Join("..", ".env"),
		filepath.Join("backend", ".env"),
	}
	// Kiểm tra cả thư mục chứa file thực thi
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), ".env"))
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "..", ".env"))
	}

	for _, path := range candidates {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])
				// Loại bỏ dấu nháy kép hoặc đơn nếu có bao quanh
				val = strings.Trim(val, `"'`)
				if key != "" && os.Getenv(key) == "" {
					_ = os.Setenv(key, val)
				}
			}
		}
		break // Đã nạp thành công từ 1 file .env đầu tiên tìm thấy
	}
}

func Load() *Config {
	loadDotEnv()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	allowOrigins := os.Getenv("ALLOW_ORIGINS")
	if allowOrigins == "" {
		allowOrigins = "*"
	}

	adminKey := os.Getenv("ADMIN_KEY")
	if adminKey == "" {
		adminKey = "apex080"
	}

	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}

	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		dbPort = "5432"
	}

	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
		dbUser = "postgres"
	}

	dbPass := os.Getenv("DB_PASSWORD")
	if dbPass == "" {
		dbPass = "postgres"
	}

	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "apex_leads"
	}

	dbSSLMode := os.Getenv("DB_SSLMODE")
	if dbSSLMode == "" {
		dbSSLMode = "disable"
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
			url.QueryEscape(dbUser),
			url.QueryEscape(dbPass),
			dbHost,
			dbPort,
			dbName,
			dbSSLMode,
		)
	}

	// Nạp Service Account Key cho Google Sheets Email Bot (mặc định cho toàn hệ thống)
	saFile := os.Getenv("GOOGLE_SERVICE_ACCOUNT_FILE")
	saJSON := os.Getenv("GOOGLE_SERVICE_ACCOUNT_JSON")

	if saJSON == "" {
		candidates := []string{
			saFile,
			"service-account.json",
			filepath.Join("backend", "service-account.json"),
			filepath.Join("..", "service-account.json"),
			filepath.Join("config", "service-account.json"),
			filepath.Join("backend", "config", "service-account.json"),
		}
		if exe, err := os.Executable(); err == nil {
			candidates = append(candidates, filepath.Join(filepath.Dir(exe), "service-account.json"))
			candidates = append(candidates, filepath.Join(filepath.Dir(exe), "..", "service-account.json"))
			candidates = append(candidates, filepath.Join(filepath.Dir(exe), "backend", "service-account.json"))
		}
		for _, path := range candidates {
			if path == "" {
				continue
			}
			if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
				saJSON = string(data)
				saFile = path
				break
			}
		}
	}

	var saEmail string
	if saJSON != "" {
		var parsed struct {
			ClientEmail string `json:"client_email"`
		}
		if err := json.Unmarshal([]byte(saJSON), &parsed); err == nil {
			saEmail = parsed.ClientEmail
		}
	}
	if saEmail == "" {
		saEmail = os.Getenv("GOOGLE_BOT_EMAIL")
	}
	if saEmail == "" {
		saEmail = "sheet-bot@rinva-drive-tool.iam.gserviceaccount.com"
	}

	return &Config{
		Port:                      port,
		DatabaseURL:               databaseURL,
		DBHost:                    dbHost,
		DBPort:                    dbPort,
		DBUser:                    dbUser,
		DBPassword:                dbPass,
		DBName:                    dbName,
		DBSSLMode:                 dbSSLMode,
		AllowOrigins:              allowOrigins,
		AdminKey:                  adminKey,
		GoogleServiceAccountJSON:  saJSON,
		GoogleServiceAccountEmail: saEmail,
		GoogleServiceAccountFile:  saFile,
	}
}
