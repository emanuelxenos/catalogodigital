package database

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// ==================== USERS ====================

// CreateUser cria um novo usuário no banco
func (db *DB) CreateUser(ctx context.Context, name, email, passwordHash string) (*User, error) {
	user := &User{}
	err := db.Pool.QueryRow(ctx,
		`INSERT INTO users (name, email, password_hash) VALUES ($1, $2, $3)
		 RETURNING id, name, email, password_hash, is_super_admin, created_at`,
		name, email, passwordHash,
	).Scan(&user.ID, &user.Name, &user.Email, &user.PasswordHash, &user.IsSuperAdmin, &user.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar usuário: %w", err)
	}
	return user, nil
}

// GetUserByEmail busca um usuário pelo email
func (db *DB) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	user := &User{}
	err := db.Pool.QueryRow(ctx,
		`SELECT id, name, email, password_hash, is_super_admin, created_at FROM users WHERE email = $1`,
		email,
	).Scan(&user.ID, &user.Name, &user.Email, &user.PasswordHash, &user.IsSuperAdmin, &user.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("usuário não encontrado: %w", err)
	}
	return user, nil
}

// GetUserByID busca um usuário pelo ID
func (db *DB) GetUserByID(ctx context.Context, id int) (*User, error) {
	user := &User{}
	err := db.Pool.QueryRow(ctx,
		`SELECT id, name, email, password_hash, is_super_admin, created_at FROM users WHERE id = $1`,
		id,
	).Scan(&user.ID, &user.Name, &user.Email, &user.PasswordHash, &user.IsSuperAdmin, &user.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("usuário não encontrado: %w", err)
	}
	return user, nil
}

// ==================== SHOPS ====================

// GetShopBySlug busca uma loja pelo slug (URL)
func (db *DB) GetShopBySlug(ctx context.Context, slug string) (*Shop, error) {
	shop := &Shop{}
	err := db.Pool.QueryRow(ctx,
		`SELECT id, user_id, name, slug, whatsapp_number, logo_url, primary_color, is_active, created_at,
		        banner_url, delivery_fee, business_hours, plan_id, plan_expires_at, COALESCE(asaas_customer_id, ''), COALESCE(asaas_subscription_id, '')
		 FROM shops WHERE slug = $1`,
		slug,
	).Scan(&shop.ID, &shop.UserID, &shop.Name, &shop.Slug, &shop.WhatsappNumber,
		&shop.LogoURL, &shop.PrimaryColor, &shop.IsActive, &shop.CreatedAt,
		&shop.BannerURL, &shop.DeliveryFee, &shop.BusinessHours, &shop.PlanID, &shop.PlanExpiresAt,
		&shop.AsaasCustomerID, &shop.AsaasSubscriptionID)
	if err != nil {
		return nil, fmt.Errorf("loja não encontrada: %w", err)
	}
	return shop, nil
}

// GetShopByUserID busca a loja de um usuário
func (db *DB) GetShopByUserID(ctx context.Context, userID int) (*Shop, error) {
	shop := &Shop{}
	err := db.Pool.QueryRow(ctx,
		`SELECT id, user_id, name, slug, whatsapp_number, logo_url, primary_color, is_active, created_at,
		        banner_url, delivery_fee, business_hours, plan_id, plan_expires_at, COALESCE(asaas_customer_id, ''), COALESCE(asaas_subscription_id, '')
		 FROM shops WHERE user_id = $1`,
		userID,
	).Scan(&shop.ID, &shop.UserID, &shop.Name, &shop.Slug, &shop.WhatsappNumber,
		&shop.LogoURL, &shop.PrimaryColor, &shop.IsActive, &shop.CreatedAt,
		&shop.BannerURL, &shop.DeliveryFee, &shop.BusinessHours, &shop.PlanID, &shop.PlanExpiresAt,
		&shop.AsaasCustomerID, &shop.AsaasSubscriptionID)
	if err != nil {
		return nil, fmt.Errorf("loja não encontrada: %w", err)
	}
	return shop, nil
}


// UpdateShop atualiza os dados de uma loja
func (db *DB) UpdateShop(ctx context.Context, shop *Shop) error {
	_, err := db.Pool.Exec(ctx,
		`UPDATE shops SET name = $1, slug = $2, whatsapp_number = $3, logo_url = $4, primary_color = $5,
		                 banner_url = $6, delivery_fee = $7, business_hours = $8
		 WHERE id = $9`,
		shop.Name, shop.Slug, shop.WhatsappNumber, shop.LogoURL, shop.PrimaryColor,
		shop.BannerURL, shop.DeliveryFee, shop.BusinessHours, shop.ID,
	)
	if err != nil {
		return fmt.Errorf("erro ao atualizar loja: %w", err)
	}
	return nil
}

// CreateShop cria uma nova loja
func (db *DB) CreateShop(ctx context.Context, shop *Shop) error {
	err := db.Pool.QueryRow(ctx,
		`INSERT INTO shops (user_id, name, slug, whatsapp_number, logo_url, primary_color, banner_url, delivery_fee, business_hours, plan_expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, CURRENT_TIMESTAMP + INTERVAL '7 days') RETURNING id, created_at, plan_expires_at`,
		shop.UserID, shop.Name, shop.Slug, shop.WhatsappNumber, shop.LogoURL, shop.PrimaryColor,
		shop.BannerURL, shop.DeliveryFee, shop.BusinessHours,
	).Scan(&shop.ID, &shop.CreatedAt, &shop.PlanExpiresAt)
	if err != nil {
		return fmt.Errorf("erro ao criar loja: %w", err)
	}
	return nil
}

// ==================== CATEGORIES ====================

// ListCategoriesByShop lista todas as categorias de uma loja ordenadas por posição
func (db *DB) ListCategoriesByShop(ctx context.Context, shopID int) ([]Category, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT id, shop_id, name, position, created_at
		 FROM categories WHERE shop_id = $1 ORDER BY position ASC, name ASC`,
		shopID,
	)
	if err != nil {
		return nil, fmt.Errorf("erro ao listar categorias: %w", err)
	}
	defer rows.Close()

	var categories []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.ShopID, &c.Name, &c.Position, &c.CreatedAt); err != nil {
			return nil, err
		}
		categories = append(categories, c)
	}
	return categories, nil
}

// CreateCategory cria uma nova categoria
func (db *DB) CreateCategory(ctx context.Context, shopID int, name string, position int) (*Category, error) {
	cat := &Category{}
	err := db.Pool.QueryRow(ctx,
		`INSERT INTO categories (shop_id, name, position) VALUES ($1, $2, $3)
		 RETURNING id, shop_id, name, position, created_at`,
		shopID, name, position,
	).Scan(&cat.ID, &cat.ShopID, &cat.Name, &cat.Position, &cat.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar categoria: %w", err)
	}
	return cat, nil
}

// DeleteCategory deleta uma categoria
func (db *DB) DeleteCategory(ctx context.Context, id, shopID int) error {
	result, err := db.Pool.Exec(ctx,
		`DELETE FROM categories WHERE id = $1 AND shop_id = $2`,
		id, shopID,
	)
	if err != nil {
		return fmt.Errorf("erro ao deletar categoria: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("categoria não encontrada")
	}
	return nil
}

// ==================== PRODUCTS ====================

// ==================== PRODUCTS ====================

// ListProductsByShop lista todos os produtos de uma loja
func (db *DB) ListProductsByShop(ctx context.Context, shopID int) ([]Product, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT p.id, p.shop_id, p.category_id, p.name, p.description, p.price,
		        p.image_url, p.is_available, p.created_at, COALESCE(c.name, 'Sem categoria') as category_name, p.options, p.images
		 FROM products p
		 LEFT JOIN categories c ON p.category_id = c.id
		 WHERE p.shop_id = $1
		 ORDER BY p.created_at DESC`,
		shopID,
	)
	if err != nil {
		return nil, fmt.Errorf("erro ao listar produtos: %w", err)
	}
	defer rows.Close()

	var products []Product
	for rows.Next() {
		var p Product
		if err := rows.Scan(&p.ID, &p.ShopID, &p.CategoryID, &p.Name, &p.Description,
			&p.Price, &p.ImageURL, &p.IsAvailable, &p.CreatedAt, &p.CategoryName, &p.Options, &p.Images); err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	return products, nil
}

// ListProductsByCategory lista produtos filtrados por categoria (para HTMX)
func (db *DB) ListProductsByCategory(ctx context.Context, shopID, categoryID int) ([]Product, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT p.id, p.shop_id, p.category_id, p.name, p.description, p.price,
		        p.image_url, p.is_available, p.created_at, COALESCE(c.name, 'Sem categoria') as category_name, p.options, p.images
		 FROM products p
		 LEFT JOIN categories c ON p.category_id = c.id
		 WHERE p.shop_id = $1 AND p.category_id = $2 AND p.is_available = TRUE
		 ORDER BY p.name ASC`,
		shopID, categoryID,
	)
	if err != nil {
		return nil, fmt.Errorf("erro ao listar produtos por categoria: %w", err)
	}
	defer rows.Close()

	var products []Product
	for rows.Next() {
		var p Product
		if err := rows.Scan(&p.ID, &p.ShopID, &p.CategoryID, &p.Name, &p.Description,
			&p.Price, &p.ImageURL, &p.IsAvailable, &p.CreatedAt, &p.CategoryName, &p.Options, &p.Images); err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	return products, nil
}

// ListAvailableProductsByShop lista produtos disponíveis de uma loja (catálogo público)
func (db *DB) ListAvailableProductsByShop(ctx context.Context, shopID int) ([]Product, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT p.id, p.shop_id, p.category_id, p.name, p.description, p.price,
		        p.image_url, p.is_available, p.created_at, COALESCE(c.name, 'Sem categoria') as category_name, p.options, p.images
		 FROM products p
		 LEFT JOIN categories c ON p.category_id = c.id
		 WHERE p.shop_id = $1 AND p.is_available = TRUE
		 ORDER BY c.position ASC, p.name ASC`,
		shopID,
	)
	if err != nil {
		return nil, fmt.Errorf("erro ao listar produtos disponíveis: %w", err)
	}
	defer rows.Close()

	var products []Product
	for rows.Next() {
		var p Product
		if err := rows.Scan(&p.ID, &p.ShopID, &p.CategoryID, &p.Name, &p.Description,
			&p.Price, &p.ImageURL, &p.IsAvailable, &p.CreatedAt, &p.CategoryName, &p.Options, &p.Images); err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	return products, nil
}

// CreateProduct cria um novo produto
func (db *DB) CreateProduct(ctx context.Context, p *Product) error {
	err := db.Pool.QueryRow(ctx,
		`INSERT INTO products (shop_id, category_id, name, description, price, image_url, is_available, options, images)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id, created_at`,
		p.ShopID, p.CategoryID, p.Name, p.Description, p.Price, p.ImageURL, p.IsAvailable, p.Options, p.Images,
	).Scan(&p.ID, &p.CreatedAt)
	if err != nil {
		return fmt.Errorf("erro ao criar produto: %w", err)
	}
	return nil
}

// UpdateProduct atualiza um produto existente
func (db *DB) UpdateProduct(ctx context.Context, p *Product) error {
	_, err := db.Pool.Exec(ctx,
		`UPDATE products SET category_id = $1, name = $2, description = $3, price = $4,
		 image_url = $5, is_available = $6, options = $7, images = $8 WHERE id = $9 AND shop_id = $10`,
		p.CategoryID, p.Name, p.Description, p.Price, p.ImageURL, p.IsAvailable, p.Options, p.Images, p.ID, p.ShopID,
	)
	if err != nil {
		return fmt.Errorf("erro ao atualizar produto: %w", err)
	}
	return nil
}

// DeleteProduct deleta um produto
func (db *DB) DeleteProduct(ctx context.Context, id, shopID int) error {
	result, err := db.Pool.Exec(ctx,
		`DELETE FROM products WHERE id = $1 AND shop_id = $2`,
		id, shopID,
	)
	if err != nil {
		return fmt.Errorf("erro ao deletar produto: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("produto não encontrado")
	}
	return nil
}

// ToggleProductAvailability alterna a disponibilidade de um produto
func (db *DB) ToggleProductAvailability(ctx context.Context, id, shopID int) (*Product, error) {
	p := &Product{}
	err := db.Pool.QueryRow(ctx,
		`UPDATE products SET is_available = NOT is_available
		 WHERE id = $1 AND shop_id = $2
		 RETURNING id, shop_id, category_id, name, description, price, image_url, is_available, created_at, options, images`,
		id, shopID,
	).Scan(&p.ID, &p.ShopID, &p.CategoryID, &p.Name, &p.Description,
		&p.Price, &p.ImageURL, &p.IsAvailable, &p.CreatedAt, &p.Options, &p.Images)
	if err != nil {
		return nil, fmt.Errorf("erro ao alternar disponibilidade: %w", err)
	}
	return p, nil
}

// GetProduct busca um produto pelo ID
func (db *DB) GetProduct(ctx context.Context, id, shopID int) (*Product, error) {
	p := &Product{}
	err := db.Pool.QueryRow(ctx,
		`SELECT p.id, p.shop_id, p.category_id, p.name, p.description, p.price,
		        p.image_url, p.is_available, p.created_at, COALESCE(c.name, 'Sem categoria') as category_name, p.options, p.images
		 FROM products p
		 LEFT JOIN categories c ON p.category_id = c.id
		 WHERE p.id = $1 AND p.shop_id = $2`,
		id, shopID,
	).Scan(&p.ID, &p.ShopID, &p.CategoryID, &p.Name, &p.Description,
		&p.Price, &p.ImageURL, &p.IsAvailable, &p.CreatedAt, &p.CategoryName, &p.Options, &p.Images)
	if err != nil {
		return nil, fmt.Errorf("produto não encontrado: %w", err)
	}
	return p, nil
}

// CountProductsByShop conta o total de produtos de uma loja
func (db *DB) CountProductsByShop(ctx context.Context, shopID int) (int, error) {
	var count int
	err := db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM products WHERE shop_id = $1`, shopID,
	).Scan(&count)
	return count, err
}

// CountCategoriesByShop conta o total de categorias de uma loja
func (db *DB) CountCategoriesByShop(ctx context.Context, shopID int) (int, error) {
	var count int
	err := db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM categories WHERE shop_id = $1`, shopID,
	).Scan(&count)
	return count, err
}

// ==================== SESSIONS ====================

// CreateSession cria uma nova sessão de autenticação
func (db *DB) CreateSession(ctx context.Context, userID int) (*Session, error) {
	// Gera um ID aleatório de 32 bytes (64 chars hex)
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return nil, fmt.Errorf("erro ao gerar ID de sessão: %w", err)
	}
	sessionID := hex.EncodeToString(bytes)

	session := &Session{}
	err := db.Pool.QueryRow(ctx,
		`INSERT INTO sessions (id, user_id, expires_at) VALUES ($1, $2, $3)
		 RETURNING id, user_id, created_at, expires_at`,
		sessionID, userID, time.Now().Add(24*time.Hour*7), // Expira em 7 dias
	).Scan(&session.ID, &session.UserID, &session.CreatedAt, &session.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar sessão: %w", err)
	}
	return session, nil
}

// GetSession busca uma sessão válida pelo ID
func (db *DB) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	session := &Session{}
	err := db.Pool.QueryRow(ctx,
		`SELECT id, user_id, created_at, expires_at FROM sessions
		 WHERE id = $1 AND expires_at > NOW()`,
		sessionID,
	).Scan(&session.ID, &session.UserID, &session.CreatedAt, &session.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("sessão não encontrada ou expirada: %w", err)
	}
	return session, nil
}

// DeleteSession deleta uma sessão (logout)
func (db *DB) DeleteSession(ctx context.Context, sessionID string) error {
	_, err := db.Pool.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, sessionID)
	return err
}

// CleanExpiredSessions remove sessões expiradas
func (db *DB) CleanExpiredSessions(ctx context.Context) error {
	_, err := db.Pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at < NOW()`)
	return err
}

// ==================== COUPONS ====================

// CreateCoupon cria um novo cupom
func (db *DB) CreateCoupon(ctx context.Context, c *Coupon) error {
	err := db.Pool.QueryRow(ctx,
		`INSERT INTO coupons (shop_id, code, type, value, is_active)
		 VALUES ($1, UPPER($2), $3, $4, $5)
		 RETURNING id, created_at`,
		c.ShopID, c.Code, c.Type, c.Value, c.IsActive,
	).Scan(&c.ID, &c.CreatedAt)
	if err != nil {
		return fmt.Errorf("erro ao criar cupom: %w", err)
	}
	return nil
}

// ListCouponsByShop lista cupons de uma loja
func (db *DB) ListCouponsByShop(ctx context.Context, shopID int) ([]Coupon, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT id, shop_id, code, type, value, is_active, created_at
		 FROM coupons WHERE shop_id = $1 ORDER BY created_at DESC`,
		shopID,
	)
	if err != nil {
		return nil, fmt.Errorf("erro ao listar cupons: %w", err)
	}
	defer rows.Close()

	var coupons []Coupon
	for rows.Next() {
		var c Coupon
		if err := rows.Scan(&c.ID, &c.ShopID, &c.Code, &c.Type, &c.Value, &c.IsActive, &c.CreatedAt); err != nil {
			return nil, err
		}
		coupons = append(coupons, c)
	}
	return coupons, nil
}

// GetCouponByCode busca cupom ativo pelo código
func (db *DB) GetCouponByCode(ctx context.Context, shopID int, code string) (*Coupon, error) {
	c := &Coupon{}
	err := db.Pool.QueryRow(ctx,
		`SELECT id, shop_id, code, type, value, is_active, created_at
		 FROM coupons WHERE shop_id = $1 AND UPPER(code) = UPPER($2) AND is_active = TRUE`,
		shopID, code,
	).Scan(&c.ID, &c.ShopID, &c.Code, &c.Type, &c.Value, &c.IsActive, &c.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("cupom não encontrado ou inativo: %w", err)
	}
	return c, nil
}

// DeleteCoupon remove um cupom
func (db *DB) DeleteCoupon(ctx context.Context, id, shopID int) error {
	result, err := db.Pool.Exec(ctx,
		`DELETE FROM coupons WHERE id = $1 AND shop_id = $2`,
		id, shopID,
	)
	if err != nil {
		return fmt.Errorf("erro ao deletar cupom: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("cupom não encontrado")
	}
	return nil
}

// ==================== ORDERS ====================

// CreateOrder cria um pedido no banco de dados
func (db *DB) CreateOrder(ctx context.Context, o *Order) error {
	err := db.Pool.QueryRow(ctx,
		`INSERT INTO orders (shop_id, customer_name, customer_phone, customer_email, delivery_method, address, payment_method, coupon_code, discount, subtotal, total, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		 RETURNING id, created_at`,
		o.ShopID, o.CustomerName, o.CustomerPhone, o.CustomerEmail, o.DeliveryMethod, o.Address, o.PaymentMethod, o.CouponCode, o.Discount, o.Subtotal, o.Total, o.Status,
	).Scan(&o.ID, &o.CreatedAt)
	if err != nil {
		return fmt.Errorf("erro ao salvar pedido no banco: %w", err)
	}
	return nil
}

// CreateOrderItem cria um item associado a um pedido
func (db *DB) CreateOrderItem(ctx context.Context, item *OrderItem) error {
	err := db.Pool.QueryRow(ctx,
		`INSERT INTO order_items (order_id, product_id, name, price, qty, note, options)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id`,
		item.OrderID, item.ProductID, item.Name, item.Price, item.Qty, item.Note, item.Options,
	).Scan(&item.ID)
	if err != nil {
		return fmt.Errorf("erro ao salvar item de pedido: %w", err)
	}
	return nil
}

// ListOrdersByShop lista os pedidos de uma loja
func (db *DB) ListOrdersByShop(ctx context.Context, shopID int) ([]Order, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT id, shop_id, customer_name, customer_phone, customer_email, delivery_method, address, payment_method, coupon_code, discount, subtotal, total, status, created_at
		 FROM orders WHERE shop_id = $1 ORDER BY created_at DESC`,
		shopID,
	)
	if err != nil {
		return nil, fmt.Errorf("erro ao listar pedidos: %w", err)
	}
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.ID, &o.ShopID, &o.CustomerName, &o.CustomerPhone, &o.CustomerEmail, &o.DeliveryMethod, &o.Address, &o.PaymentMethod, &o.CouponCode, &o.Discount, &o.Subtotal, &o.Total, &o.Status, &o.CreatedAt); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}

	// Carrega itens para cada pedido
	for i := range orders {
		items, err := db.ListOrderItems(ctx, orders[i].ID)
		if err == nil {
			orders[i].Items = items
		}
	}

	return orders, nil
}

// GetOrderByID busca um pedido pelo ID e shopID
func (db *DB) GetOrderByID(ctx context.Context, id, shopID int) (*Order, error) {
	var o Order
	err := db.Pool.QueryRow(ctx,
		`SELECT id, shop_id, customer_name, customer_phone, customer_email, delivery_method, address, payment_method, coupon_code, discount, subtotal, total, status, created_at
		 FROM orders WHERE id = $1 AND shop_id = $2`,
		id, shopID,
	).Scan(&o.ID, &o.ShopID, &o.CustomerName, &o.CustomerPhone, &o.CustomerEmail, &o.DeliveryMethod, &o.Address, &o.PaymentMethod, &o.CouponCode, &o.Discount, &o.Subtotal, &o.Total, &o.Status, &o.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar pedido: %w", err)
	}

	items, err := db.ListOrderItems(ctx, o.ID)
	if err == nil {
		o.Items = items
	}

	return &o, nil
}

// ListOrderItems lista os itens de um pedido
func (db *DB) ListOrderItems(ctx context.Context, orderID int) ([]OrderItem, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT id, order_id, product_id, name, price, qty, note, options
		 FROM order_items WHERE order_id = $1 ORDER BY id ASC`,
		orderID,
	)
	if err != nil {
		return nil, fmt.Errorf("erro ao listar itens do pedido: %w", err)
	}
	defer rows.Close()

	var items []OrderItem
	for rows.Next() {
		var item OrderItem
		if err := rows.Scan(&item.ID, &item.OrderID, &item.ProductID, &item.Name, &item.Price, &item.Qty, &item.Note, &item.Options); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

// UpdateOrderStatus atualiza o status de um pedido
func (db *DB) UpdateOrderStatus(ctx context.Context, id, shopID int, status string) error {
	result, err := db.Pool.Exec(ctx,
		`UPDATE orders SET status = $1 WHERE id = $2 AND shop_id = $3`,
		status, id, shopID,
	)
	if err != nil {
		return fmt.Errorf("erro ao atualizar status do pedido: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("pedido não encontrado")
	}
	return nil
}

// GetOrderMetrics retorna métricas consolidadas de vendas para o painel admin
func (db *DB) GetOrderMetrics(ctx context.Context, shopID int) (map[string]float64, error) {
	metrics := make(map[string]float64)

	// Faturamento total deste mês
	var faturamento float64
	err := db.Pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(total), 0) FROM orders
		 WHERE shop_id = $1 AND status != 'Cancelado'
		 AND created_at >= date_trunc('month', CURRENT_TIMESTAMP)`,
		shopID,
	).Scan(&faturamento)
	if err != nil {
		return nil, err
	}
	metrics["revenue_month"] = faturamento

	// Total de pedidos geral
	var totalOrders float64
	err = db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM orders WHERE shop_id = $1`,
		shopID,
	).Scan(&totalOrders)
	if err != nil {
		return nil, err
	}
	metrics["total_orders"] = totalOrders

	// Pedidos pendentes (Pendente, Preparando, Enviado)
	var pendingOrders float64
	err = db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM orders WHERE shop_id = $1 AND status IN ('Pendente', 'Preparando', 'Enviado')`,
		shopID,
	).Scan(&pendingOrders)
	if err != nil {
		return nil, err
	}
	metrics["pending_orders"] = pendingOrders

	// Ticket médio geral (desconsiderando cancelados)
	var ticketMedio float64
	err = db.Pool.QueryRow(ctx,
		`SELECT COALESCE(AVG(total), 0) FROM orders
		 WHERE shop_id = $1 AND status != 'Cancelado'`,
		shopID,
	).Scan(&ticketMedio)
	if err != nil {
		return nil, err
	}
	metrics["average_ticket"] = ticketMedio

	return metrics, nil
}

// GetDailySalesLast7Days busca o faturamento e número de pedidos diários dos últimos 7 dias
func (db *DB) GetDailySalesLast7Days(ctx context.Context, shopID int) ([]DailySales, error) {
	query := `
		SELECT 
			d.date::date as sales_date,
			to_char(d.date, 'DD/MM') as day_name,
			COALESCE(SUM(o.total), 0) as total_sales,
			COUNT(o.id)::int as order_count
		FROM (
			SELECT generate_series(
				CURRENT_DATE - INTERVAL '6 days',
				CURRENT_DATE,
				'1 day'::interval
			)::date as date
		) d
		LEFT JOIN orders o ON o.shop_id = $1 AND o.created_at::date = d.date AND o.status != 'Cancelado'
		GROUP BY d.date
		ORDER BY d.date ASC
	`
	rows, err := db.Pool.Query(ctx, query, shopID)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar vendas diarias: %w", err)
	}
	defer rows.Close()

	var sales []DailySales
	for rows.Next() {
		var s DailySales
		if err := rows.Scan(&s.Date, &s.DayName, &s.TotalSales, &s.OrderCount); err != nil {
			return nil, err
		}
		sales = append(sales, s)
	}
	return sales, nil
}

// GetPlatformMetrics calcula estatísticas consolidadas para o super admin
func (db *DB) GetPlatformMetrics(ctx context.Context) (map[string]interface{}, error) {
	metrics := make(map[string]interface{})

	// Total de Lojistas
	var totalUsers int
	err := db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&totalUsers)
	if err != nil {
		return nil, fmt.Errorf("erro ao contar usuarios: %w", err)
	}
	metrics["total_users"] = totalUsers

	// Total de Lojas
	var totalShops int
	err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM shops").Scan(&totalShops)
	if err != nil {
		return nil, fmt.Errorf("erro ao contar lojas: %w", err)
	}
	metrics["total_shops"] = totalShops

	// Total de Pedidos
	var totalOrders int
	err = db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM orders").Scan(&totalOrders)
	if err != nil {
		return nil, fmt.Errorf("erro ao contar pedidos: %w", err)
	}
	metrics["total_orders"] = totalOrders

	// Faturamento Consolidado Geral (desconsiderando pedidos cancelados)
	var globalRevenue float64
	err = db.Pool.QueryRow(ctx, "SELECT COALESCE(SUM(total), 0) FROM orders WHERE status != 'Cancelado'").Scan(&globalRevenue)
	if err != nil {
		return nil, fmt.Errorf("erro ao calcular faturamento global: %w", err)
	}
	metrics["global_revenue"] = globalRevenue

	return metrics, nil
}

// ListShopsWithOwners retorna todas as lojas cadastradas com os dados do lojista proprietário
func (db *DB) ListShopsWithOwners(ctx context.Context) ([]ShopWithOwner, error) {
	query := `
		SELECT s.id, s.user_id, s.name, s.slug, s.whatsapp_number, s.logo_url, s.is_active, s.created_at,
		       u.name as owner_name, u.email as owner_email, s.plan_id, s.plan_expires_at, p.name as plan_name
		FROM shops s
		JOIN users u ON s.user_id = u.id
		JOIN plans p ON s.plan_id = p.id
		ORDER BY s.created_at DESC
	`
	rows, err := db.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("erro ao consultar lojas: %w", err)
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
			return nil, err
		}
		list = append(list, sw)
	}
	return list, nil
}

// ToggleShopActive altera o status ativo de uma loja no SaaS
func (db *DB) ToggleShopActive(ctx context.Context, shopID int) (bool, error) {
	var active bool
	err := db.Pool.QueryRow(ctx,
		`UPDATE shops SET is_active = NOT is_active WHERE id = $1 RETURNING is_active`,
		shopID,
	).Scan(&active)
	if err != nil {
		return false, fmt.Errorf("erro ao alternar status da loja: %w", err)
	}
	return active, nil
}

// UpgradeShopPlan altera o plano de uma loja e define a data de expiração (dias a partir de agora)
func (db *DB) UpgradeShopPlan(ctx context.Context, shopID, planID int, days int) error {
	var err error
	if planID == 1 {
		// Plano Bronze: expira exatamente 7 dias após a criação da loja
		_, err = db.Pool.Exec(ctx,
			`UPDATE shops SET plan_id = $1, plan_expires_at = created_at + INTERVAL '7 days' WHERE id = $2`,
			planID, shopID)
	} else if days <= 0 {
		// Sem expiração
		_, err = db.Pool.Exec(ctx,
			`UPDATE shops SET plan_id = $1, plan_expires_at = NULL WHERE id = $2`,
			planID, shopID)
	} else {
		_, err = db.Pool.Exec(ctx,
			`UPDATE shops SET plan_id = $1, plan_expires_at = CURRENT_TIMESTAMP + ($2 * INTERVAL '1 day') WHERE id = $3`,
			planID, days, shopID)
	}
	if err != nil {
		return fmt.Errorf("erro ao fazer upgrade do plano: %w", err)
	}
	return nil
}

// GetPlanByID retorna os detalhes e limites de um plano
func (db *DB) GetPlanByID(ctx context.Context, planID int) (*Plan, error) {
	p := &Plan{}
	err := db.Pool.QueryRow(ctx,
		`SELECT id, name, price, max_products, max_categories, features::text FROM plans WHERE id = $1`,
		planID,
	).Scan(&p.ID, &p.Name, &p.Price, &p.MaxProducts, &p.MaxCategories, &p.Features)
	if err != nil {
		return nil, fmt.Errorf("plano não encontrado: %w", err)
	}
	return p, nil
}

// GetShopUsage retorna a quantidade atual de produtos e categorias cadastradas na loja
func (db *DB) GetShopUsage(ctx context.Context, shopID int) (currentProducts int, currentCategories int, err error) {
	err = db.Pool.QueryRow(ctx,
		`SELECT 
			(SELECT COUNT(*) FROM products WHERE shop_id = $1) as products_count,
			(SELECT COUNT(*) FROM categories WHERE shop_id = $1) as categories_count`,
		shopID,
	).Scan(&currentProducts, &currentCategories)
	if err != nil {
		return 0, 0, fmt.Errorf("erro ao calcular uso da loja: %w", err)
	}
	return currentProducts, currentCategories, nil
}

// CreateShopBanner cria um novo banner promocional associado a loja
func (db *DB) CreateShopBanner(ctx context.Context, b *ShopBanner) error {
	err := db.Pool.QueryRow(ctx,
		`INSERT INTO shop_banners (shop_id, image_url, link_url, position)
		 VALUES ($1, $2, $3, $4) RETURNING id, created_at`,
		b.ShopID, b.ImageURL, b.LinkURL, b.Position,
	).Scan(&b.ID, &b.CreatedAt)
	if err != nil {
		return fmt.Errorf("erro ao criar banner: %w", err)
	}
	return nil
}

// ListShopBanners lista todos os banners promocionais da loja
func (db *DB) ListShopBanners(ctx context.Context, shopID int) ([]ShopBanner, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT id, shop_id, image_url, link_url, position, created_at
		 FROM shop_banners WHERE shop_id = $1 ORDER BY position ASC, created_at DESC`,
		shopID,
	)
	if err != nil {
		return nil, fmt.Errorf("erro ao listar banners: %w", err)
	}
	defer rows.Close()

	var banners []ShopBanner
	for rows.Next() {
		var b ShopBanner
		if err := rows.Scan(&b.ID, &b.ShopID, &b.ImageURL, &b.LinkURL, &b.Position, &b.CreatedAt); err != nil {
			return nil, err
		}
		banners = append(banners, b)
	}
	return banners, nil
}

// DeleteShopBanner deleta um banner promocional
func (db *DB) DeleteShopBanner(ctx context.Context, id, shopID int) error {
	result, err := db.Pool.Exec(ctx,
		`DELETE FROM shop_banners WHERE id = $1 AND shop_id = $2`,
		id, shopID,
	)
	if err != nil {
		return fmt.Errorf("erro ao deletar banner: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("banner não encontrado")
	}
	return nil
}

// GetBestSellingProducts busca os produtos mais vendidos de uma loja
func (db *DB) GetBestSellingProducts(ctx context.Context, shopID, limit int) ([]BestSeller, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT oi.name, SUM(oi.qty)::int as total_qty, SUM(oi.qty * oi.price)::float8 as total_revenue
		 FROM order_items oi
		 JOIN orders o ON oi.order_id = o.id
		 WHERE o.shop_id = $1 AND o.status != 'Cancelado'
		 GROUP BY oi.name
		 ORDER BY total_qty DESC
		 LIMIT $2`,
		shopID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar mais vendidos: %w", err)
	}
	defer rows.Close()

	var list []BestSeller
	for rows.Next() {
		var item BestSeller
		if err := rows.Scan(&item.Name, &item.TotalQty, &item.TotalRevenue); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, nil
}

// GetSalesByPaymentMethod busca estatísticas de vendas por método de pagamento
func (db *DB) GetSalesByPaymentMethod(ctx context.Context, shopID int) ([]PaymentMethodStat, error) {
	rows, err := db.Pool.Query(ctx,
		`SELECT o.payment_method, COUNT(*)::int as order_count, SUM(o.total)::float8 as total_revenue
		 FROM orders o
		 WHERE o.shop_id = $1 AND o.status != 'Cancelado'
		 GROUP BY o.payment_method`,
		shopID,
	)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar vendas por pagamento: %w", err)
	}
	defer rows.Close()

	var list []PaymentMethodStat
	for rows.Next() {
		var item PaymentMethodStat
		if err := rows.Scan(&item.Method, &item.OrderCount, &item.TotalRevenue); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, nil
}

// GetDailySalesLast30Days busca faturamento diário dos últimos 30 dias
func (db *DB) GetDailySalesLast30Days(ctx context.Context, shopID int) ([]DailySales, error) {
	query := `
		SELECT 
			d.date::date as sales_date,
			to_char(d.date, 'DD/MM') as day_name,
			COALESCE(SUM(o.total), 0) as total_sales,
			COUNT(o.id)::int as order_count
		FROM (
			SELECT generate_series(
				CURRENT_DATE - INTERVAL '29 days',
				CURRENT_DATE,
				'1 day'::interval
			)::date as date
		) d
		LEFT JOIN orders o ON o.shop_id = $1 AND o.created_at::date = d.date AND o.status != 'Cancelado'
		GROUP BY d.date
		ORDER BY d.date ASC
	`
	rows, err := db.Pool.Query(ctx, query, shopID)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar vendas de 30 dias: %w", err)
	}
	defer rows.Close()

	var sales []DailySales
	for rows.Next() {
		var s DailySales
		if err := rows.Scan(&s.Date, &s.DayName, &s.TotalSales, &s.OrderCount); err != nil {
			return nil, err
		}
		sales = append(sales, s)
	}
	return sales, nil
}

// ==================== ASAAS PAYMENT CHARGES ====================

// SaveAsaasCustomerID salva o ID do cliente no Asaas para reutilização
func (db *DB) SaveAsaasCustomerID(ctx context.Context, shopID int, customerID string) error {
	_, err := db.Pool.Exec(ctx,
		`UPDATE shops SET asaas_customer_id = $1 WHERE id = $2`,
		customerID, shopID,
	)
	if err != nil {
		return fmt.Errorf("erro ao salvar asaas_customer_id: %w", err)
	}
	return nil
}

// CreatePaymentCharge registra uma nova cobrança pendente no banco
func (db *DB) CreatePaymentCharge(ctx context.Context, c *PaymentCharge) error {
	err := db.Pool.QueryRow(ctx,
		`INSERT INTO payment_charges (shop_id, plan_id, asaas_payment_id, billing_type, amount, status, pix_qr_code, pix_copy_paste, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id, created_at`,
		c.ShopID, c.PlanID, c.AsaasPaymentID, c.BillingType, c.Amount, c.Status,
		c.PixQRCode, c.PixCopyPaste, c.ExpiresAt,
	).Scan(&c.ID, &c.CreatedAt)
	if err != nil {
		return fmt.Errorf("erro ao criar payment_charge: %w", err)
	}
	return nil
}

// GetChargeByAsaasID busca uma cobrança pelo ID do Asaas
func (db *DB) GetChargeByAsaasID(ctx context.Context, asaasID string) (*PaymentCharge, error) {
	c := &PaymentCharge{}
	err := db.Pool.QueryRow(ctx,
		`SELECT id, shop_id, plan_id, asaas_payment_id, billing_type, amount, status, pix_qr_code, pix_copy_paste, expires_at, created_at
		 FROM payment_charges WHERE asaas_payment_id = $1`,
		asaasID,
	).Scan(&c.ID, &c.ShopID, &c.PlanID, &c.AsaasPaymentID, &c.BillingType, &c.Amount, &c.Status,
		&c.PixQRCode, &c.PixCopyPaste, &c.ExpiresAt, &c.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("cobrança não encontrada: %w", err)
	}
	return c, nil
}

// GetChargeByID busca uma cobrança pelo ID interno (para polling de status)
func (db *DB) GetChargeByID(ctx context.Context, id int) (*PaymentCharge, error) {
	c := &PaymentCharge{}
	err := db.Pool.QueryRow(ctx,
		`SELECT id, shop_id, plan_id, asaas_payment_id, billing_type, amount, status, pix_qr_code, pix_copy_paste, expires_at, created_at
		 FROM payment_charges WHERE id = $1`,
		id,
	).Scan(&c.ID, &c.ShopID, &c.PlanID, &c.AsaasPaymentID, &c.BillingType, &c.Amount, &c.Status,
		&c.PixQRCode, &c.PixCopyPaste, &c.ExpiresAt, &c.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("cobrança não encontrada: %w", err)
	}
	return c, nil
}

// UpdateChargeStatus atualiza o status de uma cobrança
func (db *DB) UpdateChargeStatus(ctx context.Context, asaasID, status string) error {
	_, err := db.Pool.Exec(ctx,
		`UPDATE payment_charges SET status = $1 WHERE asaas_payment_id = $2`,
		status, asaasID,
	)
	if err != nil {
		return fmt.Errorf("erro ao atualizar status da cobrança: %w", err)
	}
	return nil
}

// SaveAsaasSubscriptionID associa o ID da assinatura ativa à loja
func (db *DB) SaveAsaasSubscriptionID(ctx context.Context, shopID int, subscriptionID string) error {
	_, err := db.Pool.Exec(ctx,
		`UPDATE shops SET asaas_subscription_id = $1 WHERE id = $2`,
		subscriptionID, shopID,
	)
	if err != nil {
		return fmt.Errorf("erro ao salvar asaas_subscription_id: %w", err)
	}
	return nil
}

// GetShopByAsaasCustomerID localiza uma loja pelo ID do cliente no Asaas
func (db *DB) GetShopByAsaasCustomerID(ctx context.Context, customerID string) (*Shop, error) {
	shop := &Shop{}
	err := db.Pool.QueryRow(ctx,
		`SELECT id, user_id, name, slug, whatsapp_number, logo_url, primary_color, is_active, created_at,
		        banner_url, delivery_fee, business_hours, plan_id, plan_expires_at, COALESCE(asaas_customer_id, ''), COALESCE(asaas_subscription_id, '')
		 FROM shops WHERE asaas_customer_id = $1`,
		customerID,
	).Scan(&shop.ID, &shop.UserID, &shop.Name, &shop.Slug, &shop.WhatsappNumber,
		&shop.LogoURL, &shop.PrimaryColor, &shop.IsActive, &shop.CreatedAt,
		&shop.BannerURL, &shop.DeliveryFee, &shop.BusinessHours, &shop.PlanID, &shop.PlanExpiresAt,
		&shop.AsaasCustomerID, &shop.AsaasSubscriptionID)
	if err != nil {
		return nil, fmt.Errorf("loja não encontrada para o Asaas Customer ID %s: %w", customerID, err)
	}
	return shop, nil
}

// ListChargesByShop recupera o histórico de cobranças de uma loja de forma paginada e opcionalmente filtrada por ano
func (db *DB) ListChargesByShop(ctx context.Context, shopID int, limit, offset int, year int) ([]PaymentCharge, int, error) {
	var total int
	var err error
	if year > 0 {
		err = db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM payment_charges WHERE shop_id = $1 AND EXTRACT(YEAR FROM created_at) = $2`, shopID, year).Scan(&total)
	} else {
		err = db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM payment_charges WHERE shop_id = $1`, shopID).Scan(&total)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("erro ao contar cobranças da loja: %w", err)
	}

	var query string
	var args []interface{}
	if year > 0 {
		query = `SELECT id, shop_id, plan_id, asaas_payment_id, billing_type, amount, status, pix_qr_code, pix_copy_paste, expires_at, created_at
		         FROM payment_charges WHERE shop_id = $1 AND EXTRACT(YEAR FROM created_at) = $2 ORDER BY created_at DESC LIMIT $3 OFFSET $4`
		args = []interface{}{shopID, year, limit, offset}
	} else {
		query = `SELECT id, shop_id, plan_id, asaas_payment_id, billing_type, amount, status, pix_qr_code, pix_copy_paste, expires_at, created_at
		         FROM payment_charges WHERE shop_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`
		args = []interface{}{shopID, limit, offset}
	}

	rows, err := db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("erro ao listar cobranças da loja: %w", err)
	}
	defer rows.Close()

	var charges []PaymentCharge
	for rows.Next() {
		var c PaymentCharge
		err := rows.Scan(&c.ID, &c.ShopID, &c.PlanID, &c.AsaasPaymentID, &c.BillingType, &c.Amount, &c.Status,
			&c.PixQRCode, &c.PixCopyPaste, &c.ExpiresAt, &c.CreatedAt)
		if err != nil {
			return nil, 0, err
		}
		charges = append(charges, c)
	}
	return charges, total, nil
}

// GetBillingYears retorna todos os anos distintos com faturamento para a loja
func (db *DB) GetBillingYears(ctx context.Context, shopID int) ([]int, error) {
	rows, err := db.Pool.Query(ctx, `SELECT DISTINCT EXTRACT(YEAR FROM created_at)::int as y FROM payment_charges WHERE shop_id = $1 ORDER BY y DESC`, shopID)
	if err != nil {
		return nil, fmt.Errorf("erro ao listar anos de faturamento: %w", err)
	}
	defer rows.Close()

	var years []int
	for rows.Next() {
		var y int
		if err := rows.Scan(&y); err == nil {
			years = append(years, y)
		}
	}
	if len(years) == 0 {
		years = append(years, time.Now().Year())
	}
	return years, nil
}





