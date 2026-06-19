package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// Config armazena todas as configurações da aplicação
type Config struct {
	DatabaseURL         string
	Port                string
	SessionSecret       string
	DevMode             bool
	SMTPHost            string
	SMTPPort            string
	SMTPUser            string
	SMTPPass            string
	SMTPFrom            string
	AsaasAPIKey         string
	AsaasAPIURL         string
	AsaasWebhookToken   string
	AppBaseURL          string
}

// Load carrega as variáveis de ambiente do .env e retorna a configuração
func Load() (*Config, error) {
	// Tenta carregar .env (não falha se não existir)
	_ = godotenv.Load()

	// Decide qual chave e URL do Asaas usar (sandbox ou produção)
	asaasEnv := getEnv("ASSAAS_ENV", "sandbox")
	var asaasKey, asaasURL string
	if asaasEnv == "production" {
		asaasKey = getEnv("ASSAAS_PROD_KEY", "")
		asaasURL = "https://www.asaas.com/api/v3"
	} else {
		asaasKey = getEnv("ASSAAS_SANDBOX_KEY", "")
		asaasURL = "https://sandbox.asaas.com/api/v3"
	}

	cfg := &Config{
		DatabaseURL:       getEnv("DATABASE_URL", "postgres://postgres:admin123@localhost:5432/catalogo?sslmode=disable"),
		Port:              getEnv("PORT", "8080"),
		SessionSecret:     getEnv("SESSION_SECRET", "catalogo-secret-key-default"),
		DevMode:           getEnv("DEV_MODE", "false") == "true",
		SMTPHost:          getEnv("SMTP_HOST", ""),
		SMTPPort:          getEnv("SMTP_PORT", "587"),
		SMTPUser:          getEnv("SMTP_USER", ""),
		SMTPPass:          getEnv("SMTP_PASS", ""),
		SMTPFrom:          getEnv("SMTP_FROM", ""),
		AsaasAPIKey:       asaasKey,
		AsaasAPIURL:       asaasURL,
		AsaasWebhookToken: getEnv("ASSAAS_WEBHOOK_TOKEN", "catalogo-webhook-secret-2024"),
		AppBaseURL:        getEnv("APP_BASE_URL", "http://localhost:8080"),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL é obrigatório")
	}

	return cfg, nil
}


func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
