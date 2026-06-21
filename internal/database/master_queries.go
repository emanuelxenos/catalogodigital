package database

import (
	"context"
	"fmt"
)

// GetPlatformConfigs busca todas as configurações globais da plataforma e as retorna em um map
func (db *DB) GetPlatformConfigs(ctx context.Context) (map[string]string, error) {
	configs := make(map[string]string)
	rows, err := db.Pool.Query(ctx, "SELECT key, value FROM platform_configs")
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar platform_configs: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var key, val string
		if err := rows.Scan(&key, &val); err != nil {
			return nil, fmt.Errorf("erro ao scanear platform_config: %w", err)
		}
		configs[key] = val
	}

	return configs, nil
}

// UpdatePlatformConfig insere ou atualiza uma configuração global no banco
func (db *DB) UpdatePlatformConfig(ctx context.Context, key, value string) error {
	_, err := db.Pool.Exec(ctx, 
		`INSERT INTO platform_configs (key, value, updated_at) VALUES ($1, $2, CURRENT_TIMESTAMP)
		 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = CURRENT_TIMESTAMP`,
		key, value)
	if err != nil {
		return fmt.Errorf("erro ao salvar platform_config %s: %w", key, err)
	}
	return nil
}

// ListPlatformAuditLogs busca os logs de auditoria mais recentes (últimos 50)
func (db *DB) ListPlatformAuditLogs(ctx context.Context) ([]PlatformAuditLog, error) {
	var logs []PlatformAuditLog
	rows, err := db.Pool.Query(ctx, 
		"SELECT id, action, details, created_at FROM platform_audit_logs ORDER BY created_at DESC LIMIT 50")
	if err != nil {
		return nil, fmt.Errorf("erro ao listar platform_audit_logs: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var l PlatformAuditLog
		if err := rows.Scan(&l.ID, &l.Action, &l.Details, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("erro ao scanear platform_audit_log: %w", err)
		}
		logs = append(logs, l)
	}

	return logs, nil
}

// CreatePlatformAuditLog grava um novo registro de auditoria na plataforma
func (db *DB) CreatePlatformAuditLog(ctx context.Context, action, details string) error {
	_, err := db.Pool.Exec(ctx, 
		"INSERT INTO platform_audit_logs (action, details) VALUES ($1, $2)", 
		action, details)
	if err != nil {
		return fmt.Errorf("erro ao criar platform_audit_log: %w", err)
	}
	return nil
}

// ListGlobalChargesPaginated lista todas as cobranças de faturas na plataforma de forma paginada
func (db *DB) ListGlobalChargesPaginated(ctx context.Context, limit, offset int) ([]PaymentChargeWithDetails, int, error) {
	var total int
	err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM payment_charges`).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("erro ao contar faturas globais: %w", err)
	}

	query := `
		SELECT c.id, c.shop_id, c.plan_id, c.asaas_payment_id, c.billing_type, c.amount, c.status, c.pix_qr_code, c.pix_copy_paste, c.expires_at, c.created_at, s.name as shop_name, p.name as plan_name
		FROM payment_charges c
		JOIN shops s ON c.shop_id = s.id
		JOIN plans p ON c.plan_id = p.id
		ORDER BY c.created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := db.Pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("erro ao consultar faturas globais: %w", err)
	}
	defer rows.Close()

	var list []PaymentChargeWithDetails
	for rows.Next() {
		var c PaymentChargeWithDetails
		err := rows.Scan(
			&c.ID, &c.ShopID, &c.PlanID, &c.AsaasPaymentID, &c.BillingType, &c.Amount, &c.Status, &c.PixQRCode, &c.PixCopyPaste, &c.ExpiresAt, &c.CreatedAt, &c.ShopName, &c.PlanName,
		)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, c)
	}
	return list, total, nil
}
