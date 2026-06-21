package database

import (
	"context"
	"fmt"
	"time"
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

// ListShopsWithOwnersPaginated lista todas as lojas de forma paginada com filtro de busca opcional
func (db *DB) ListShopsWithOwnersPaginated(ctx context.Context, limit, offset int, search string) ([]ShopWithOwner, int, error) {
	var total int
	var err error
	
	queryCount := `SELECT COUNT(*) FROM shops s JOIN users u ON s.user_id = u.id`
	querySelect := `
		SELECT s.id, s.user_id, s.name, s.slug, s.whatsapp_number, s.logo_url, s.is_active, s.created_at,
		       u.name as owner_name, u.email as owner_email, s.plan_id, s.plan_expires_at, p.name as plan_name
		FROM shops s
		JOIN users u ON s.user_id = u.id
		JOIN plans p ON s.plan_id = p.id
	`
	
	var countArgs []interface{}
	var selectArgs []interface{}
	filterSQL := ""
	
	if search != "" {
		filterSQL = " WHERE s.name ILIKE $1 OR s.slug ILIKE $1 OR u.name ILIKE $1 OR u.email ILIKE $1"
		pattern := "%" + search + "%"
		countArgs = append(countArgs, pattern)
		selectArgs = append(selectArgs, pattern)
	}
	
	err = db.Pool.QueryRow(ctx, queryCount+filterSQL, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("erro ao contar lojas: %w", err)
	}
	
	querySelect += filterSQL
	if search != "" {
		querySelect += " ORDER BY s.created_at DESC LIMIT $2 OFFSET $3"
		selectArgs = append(selectArgs, limit, offset)
	} else {
		querySelect += " ORDER BY s.created_at DESC LIMIT $1 OFFSET $2"
		selectArgs = append(selectArgs, limit, offset)
	}
	
	rows, err := db.Pool.Query(ctx, querySelect, selectArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("erro ao consultar lojas paginadas: %w", err)
	}
	defer rows.Close()

	var list []ShopWithOwner
	for rows.Next() {
		var sw ShopWithOwner
		err := rows.Scan(
			&sw.ID, &sw.UserID, &sw.Name, &sw.Slug, &sw.WhatsappNumber, &sw.LogoURL, &sw.IsActive, &sw.CreatedAt,
			&sw.OwnerName, &sw.OwnerEmail, &sw.PlanID, &sw.PlanExpiresAt, &sw.PlanName,
		)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, sw)
	}
	return list, total, nil
}
// UpdatePlan atualiza o preço e os limites de um plano específico no banco de dados
func (db *DB) UpdatePlan(ctx context.Context, id int, price float64, maxProducts, maxCategories int) error {
	_, err := db.Pool.Exec(ctx,
		`UPDATE plans SET price = $1, max_products = $2, max_categories = $3 WHERE id = $4`,
		price, maxProducts, maxCategories, id)
	if err != nil {
		return fmt.Errorf("erro ao atualizar plano: %w", err)
	}
	return nil
}

// ListGlobalChargesFiltered retorna faturas mestre filtradas por período/ano com paginação
func (db *DB) ListGlobalChargesFiltered(ctx context.Context, limit, offset int, startDate, endDate string, year int) ([]PaymentChargeWithDetails, int, error) {
	var total int
	queryCount := `
		SELECT COUNT(*) 
		FROM payment_charges c
		JOIN shops s ON c.shop_id = s.id
		WHERE 1=1
	`
	querySelect := `
		SELECT c.id, c.shop_id, c.plan_id, c.asaas_payment_id, c.billing_type, c.amount, c.status, c.pix_qr_code, c.pix_copy_paste, c.expires_at, c.created_at, s.name as shop_name, p.name as plan_name
		FROM payment_charges c
		JOIN shops s ON c.shop_id = s.id
		JOIN plans p ON c.plan_id = p.id
		WHERE 1=1
	`

	var args []interface{}
	var countArgs []interface{}
	argID := 1

	filterSQL := ""
	if startDate != "" {
		filterSQL += fmt.Sprintf(" AND c.created_at >= $%d", argID)
		args = append(args, startDate+" 00:00:00")
		countArgs = append(countArgs, startDate+" 00:00:00")
		argID++
	}
	if endDate != "" {
		filterSQL += fmt.Sprintf(" AND c.created_at <= $%d", argID)
		args = append(args, endDate+" 23:59:59")
		countArgs = append(countArgs, endDate+" 23:59:59")
		argID++
	}
	if year > 0 {
		filterSQL += fmt.Sprintf(" AND EXTRACT(YEAR FROM c.created_at) = $%d", argID)
		args = append(args, year)
		countArgs = append(countArgs, year)
		argID++
	}

	// Executa contagem
	err := db.Pool.QueryRow(ctx, queryCount+filterSQL, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("erro ao contar faturas filtradas: %w", err)
	}

	// Executa busca
	querySelect += filterSQL
	querySelect += fmt.Sprintf(" ORDER BY c.created_at DESC LIMIT $%d OFFSET $%d", argID, argID+1)
	args = append(args, limit, offset)

	rows, err := db.Pool.Query(ctx, querySelect, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("erro ao consultar faturas filtradas: %w", err)
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

// ListGlobalChargesFilteredAll retorna todas as faturas filtradas sem limite para exportação
func (db *DB) ListGlobalChargesFilteredAll(ctx context.Context, startDate, endDate string, year int) ([]PaymentChargeWithDetails, error) {
	querySelect := `
		SELECT c.id, c.shop_id, c.plan_id, c.asaas_payment_id, c.billing_type, c.amount, c.status, c.pix_qr_code, c.pix_copy_paste, c.expires_at, c.created_at, s.name as shop_name, p.name as plan_name
		FROM payment_charges c
		JOIN shops s ON c.shop_id = s.id
		JOIN plans p ON c.plan_id = p.id
		WHERE 1=1
	`

	var args []interface{}
	argID := 1

	filterSQL := ""
	if startDate != "" {
		filterSQL += fmt.Sprintf(" AND c.created_at >= $%d", argID)
		args = append(args, startDate+" 00:00:00")
		argID++
	}
	if endDate != "" {
		filterSQL += fmt.Sprintf(" AND c.created_at <= $%d", argID)
		args = append(args, endDate+" 23:59:59")
		argID++
	}
	if year > 0 {
		filterSQL += fmt.Sprintf(" AND EXTRACT(YEAR FROM c.created_at) = $%d", argID)
		args = append(args, year)
		argID++
	}

	querySelect += filterSQL + " ORDER BY c.created_at DESC"

	rows, err := db.Pool.Query(ctx, querySelect, args...)
	if err != nil {
		return nil, fmt.Errorf("erro ao consultar todas as faturas filtradas: %w", err)
	}
	defer rows.Close()

	var list []PaymentChargeWithDetails
	for rows.Next() {
		var c PaymentChargeWithDetails
		err := rows.Scan(
			&c.ID, &c.ShopID, &c.PlanID, &c.AsaasPaymentID, &c.BillingType, &c.Amount, &c.Status, &c.PixQRCode, &c.PixCopyPaste, &c.ExpiresAt, &c.CreatedAt, &c.ShopName, &c.PlanName,
		)
		if err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, nil
}

// GetSaaSRevenueChartData retorna faturamento agrupado por dia ou mês para o gráfico
func (db *DB) GetSaaSRevenueChartData(ctx context.Context, startDate, endDate string, year int) ([]SaaSRevenuePoint, error) {
	// 1. Busca todas as cobranças pagas/confirmadas que atendem ao filtro
	querySelect := `
		SELECT c.created_at, c.amount
		FROM payment_charges c
		WHERE c.status IN ('RECEIVED', 'CONFIRMED')
	`
	var args []interface{}
	argID := 1

	filterSQL := ""
	if startDate != "" {
		filterSQL += fmt.Sprintf(" AND c.created_at >= $%d", argID)
		args = append(args, startDate+" 00:00:00")
		argID++
	}
	if endDate != "" {
		filterSQL += fmt.Sprintf(" AND c.created_at <= $%d", argID)
		args = append(args, endDate+" 23:59:59")
		argID++
	}
	if year > 0 {
		filterSQL += fmt.Sprintf(" AND EXTRACT(YEAR FROM c.created_at) = $%d", argID)
		args = append(args, year)
		argID++
	}

	querySelect += filterSQL + " ORDER BY c.created_at ASC"

	rows, err := db.Pool.Query(ctx, querySelect, args...)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar dados do gráfico SaaS: %w", err)
	}
	defer rows.Close()

	type rawCharge struct {
		createdAt time.Time
		amount    float64
	}
	var charges []rawCharge
	for rows.Next() {
		var rc rawCharge
		if err := rows.Scan(&rc.createdAt, &rc.amount); err == nil {
			charges = append(charges, rc)
		}
	}

	// 2. Determina o período de agrupamento (Dia se <= 45 dias, Mês se > 45 dias)
	var tStart, tEnd time.Time
	var parseErr error
	if startDate != "" && endDate != "" {
		tStart, parseErr = time.Parse("2006-01-02", startDate)
		if parseErr == nil {
			tEnd, _ = time.Parse("2006-01-02", endDate)
		}
	}

	if parseErr != nil || startDate == "" || endDate == "" {
		if len(charges) > 0 {
			tStart = charges[0].createdAt
			tEnd = charges[len(charges)-1].createdAt
		} else {
			tEnd = time.Now()
			tStart = tEnd.AddDate(0, 0, -30)
		}
	}

	useMonthly := false
	if year > 0 {
		useMonthly = true
		tStart = time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
		tEnd = time.Date(year, time.December, 31, 23, 59, 59, 0, time.UTC)
	} else if tEnd.Sub(tStart).Hours() > (45 * 24) {
		useMonthly = true
	}

	var points []SaaSRevenuePoint

	if !useMonthly {
		// Agrupa por DIA (DD/MM)
		daysMap := make(map[string]*SaaSRevenuePoint)
		var daysOrder []string

		curr := tStart
		for !curr.After(tEnd) {
			dayStr := curr.Format("02/01")
			daysOrder = append(daysOrder, dayStr)
			daysMap[dayStr] = &SaaSRevenuePoint{
				DayName:    dayStr,
				TotalSales: 0.0,
				OrderCount: 0,
			}
			curr = curr.AddDate(0, 0, 1)
		}

		for _, c := range charges {
			dayStr := c.createdAt.Format("02/01")
			if pt, exists := daysMap[dayStr]; exists {
				pt.TotalSales += c.amount
				pt.OrderCount++
			}
		}

		for _, dayStr := range daysOrder {
			points = append(points, *daysMap[dayStr])
		}
	} else {
		// Agrupa por MÊS
		monthNames := []string{"Jan", "Fev", "Mar", "Abr", "Mai", "Jun", "Jul", "Ago", "Set", "Out", "Nov", "Dez"}
		
		type monthKey struct {
			year  int
			month int
		}
		var monthsOrder []monthKey
		mKeyMap := make(map[monthKey]*SaaSRevenuePoint)

		startMonth := int(tStart.Month())
		startYear := tStart.Year()
		endMonth := int(tEnd.Month())
		endYear := tEnd.Year()

		curr := time.Date(startYear, time.Month(startMonth), 1, 0, 0, 0, 0, time.UTC)
		limit := time.Date(endYear, time.Month(endMonth), 1, 0, 0, 0, 0, time.UTC)

		for !curr.After(limit) {
			k := monthKey{year: curr.Year(), month: int(curr.Month())}
			monthsOrder = append(monthsOrder, k)
			
			label := monthNames[k.month-1]
			if startYear != endYear {
				label = fmt.Sprintf("%s/%02d", label, k.year%100)
			}
			
			mKeyMap[k] = &SaaSRevenuePoint{
				DayName:    label,
				TotalSales: 0.0,
				OrderCount: 0,
			}
			curr = curr.AddDate(0, 1, 0)
		}

		for _, c := range charges {
			k := monthKey{year: c.createdAt.Year(), month: int(c.createdAt.Month())}
			if pt, exists := mKeyMap[k]; exists {
				pt.TotalSales += c.amount
				pt.OrderCount++
			}
		}

		for _, k := range monthsOrder {
			points = append(points, *mKeyMap[k])
		}
	}

	return points, nil
}

// GetDistinctBillingYears retorna todos os anos que possuem cobranças no sistema
func (db *DB) GetDistinctBillingYears(ctx context.Context) ([]int, error) {
	rows, err := db.Pool.Query(ctx, 
		"SELECT DISTINCT EXTRACT(YEAR FROM created_at)::int AS yr FROM payment_charges ORDER BY yr DESC")
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar anos de faturamento: %w", err)
	}
	defer rows.Close()

	var years []int
	for rows.Next() {
		var yr int
		if err := rows.Scan(&yr); err == nil {
			years = append(years, yr)
		}
	}
	
	currentYear := time.Now().Year()
	hasCurrent := false
	for _, y := range years {
		if y == currentYear {
			hasCurrent = true
			break
		}
	}
	if !hasCurrent {
		years = append([]int{currentYear}, years...)
	}

	return years, nil
}
