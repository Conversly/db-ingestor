package config

import (
	"errors"
	"os"
	"strconv"
)

type Config struct {
	DatabaseURL            string
	LogLevel               string
	Debug                  bool
	ServiceName            string
	Environment            string
	Hostname               string
	ServerPort             string
	WorkerCount            int
	BatchSize              int
	JwtRefreshSecret       string
	JwtSecret              string
	Port                   string
	AllowedOrigins         []string
}

func LoadConfig() (*Config, error) {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	jwtRefreshSecret := os.Getenv("JWT_REFRESH_SECRET")
	if jwtRefreshSecret == "" {
		return nil, errors.New("JWT_REFRESH_SECRET is required")
	}
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return nil, errors.New("JWT_SECRET is required")
	}
	allowedOrigins := []string{"*"}
	if ao := os.Getenv("ALLOWED_ORIGINS"); ao != "" {
		allowedOrigins = []string{}
		for _, origin := range os.Getenv("ALLOWED_ORIGINS") {
			allowedOrigins = append(allowedOrigins, string(origin))
		}
	}
	databaseUrl := os.Getenv("DATABASE_URL")
	if databaseUrl == "" {
		return nil, errors.New("DATABASE_URL is required")
	}

	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}

	debug := os.Getenv("DEBUG")
	if debug == "" {
		debug = "false"
	}

	serviceName := os.Getenv("SERVICE_NAME")
	if serviceName == "" {
		serviceName = "lightning-orders"
	}

	hostname := os.Getenv("HOSTNAME")
	if hostname == "" {
		hostname = "lightning-orders"
	}

	environment := os.Getenv("ENVIRONMENT")
	if environment == "" {
		environment = "development"
	}

	serverPort := os.Getenv("SERVER_PORT")
	if serverPort == "" {
		serverPort = "8080"
	}

	workerCount := 10 // default value
	if wc := os.Getenv("WORKER_COUNT"); wc != "" {
		if parsed, err := strconv.Atoi(wc); err == nil {
			workerCount = parsed
		}
	}

	batchSize := 100 // default value
	if bs := os.Getenv("BATCH_SIZE"); bs != "" {
		if parsed, err := strconv.Atoi(bs); err == nil {
			batchSize = parsed
		}
	}

	return &Config{
		JwtRefreshSecret:       jwtRefreshSecret,
		JwtSecret:              jwtSecret,
		Port:                   port,
		AllowedOrigins:         allowedOrigins,
		DatabaseURL: databaseUrl,
		LogLevel:    logLevel,
		Debug:       debug == "true",
		ServiceName: serviceName,
		Hostname:    hostname,
		Environment: environment,
		ServerPort:  serverPort,
		WorkerCount: workerCount,
		BatchSize:   batchSize,
	}, nil
}
