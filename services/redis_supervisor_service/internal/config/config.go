package config

type Config struct {
	RedisAddrs    []string
	DBServiceURL  string
	PingInterval  int
	FailureThreshold int
}

func LoadConfig() (*Config, error) {
	// TODO: Load configuration from environment variables or a file
	return &Config{
		RedisAddrs:    []string{"redis-a:6379", "redis-b:6379"}, // Example values
		DBServiceURL:  "http://db_service:8005",                   // Example value
		PingInterval:  1,                                        // In seconds
		FailureThreshold: 3,                                        // Number of consecutive failures
	}, nil
}
