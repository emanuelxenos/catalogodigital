package database

import (
	"time"
)

// User representa um lojista/administrador
type User struct {
	ID           int       `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	IsSuperAdmin bool      `json:"is_super_admin"`
	CreatedAt    time.Time `json:"created_at"`
}

// Shop representa uma loja (tenant)
type Shop struct {
	ID             int       `json:"id"`
	UserID         int       `json:"user_id"`
	Name           string    `json:"name"`
	Slug           string    `json:"slug"`
	WhatsappNumber string    `json:"whatsapp_number"`
	LogoURL        string    `json:"logo_url"`
	PrimaryColor   string    `json:"primary_color"`
	IsActive       bool      `json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
	BannerURL      string    `json:"banner_url"`
	DeliveryFee    float64   `json:"delivery_fee"`
	BusinessHours  *string   `json:"business_hours"` // JSON string
}

// Category representa uma categoria de produtos
type Category struct {
	ID        int       `json:"id"`
	ShopID    int       `json:"shop_id"`
	Name      string    `json:"name"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"created_at"`
}

// Product representa um produto do catálogo
type Product struct {
	ID           int       `json:"id"`
	ShopID       int       `json:"shop_id"`
	CategoryID   *int      `json:"category_id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Price        float64   `json:"price"`
	ImageURL     string    `json:"image_url"`
	IsAvailable  bool      `json:"is_available"`
	CreatedAt    time.Time `json:"created_at"`
	CategoryName string    `json:"category_name,omitempty"` // JOIN auxiliar
	Options      *string   `json:"options"`                 // JSON string
	Images       *string   `json:"images"`                  // JSON string contendo array de URLs
}

// Session representa uma sessão de autenticação
type Session struct {
	ID        string    `json:"id"`
	UserID    int       `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Coupon representa um cupom de desconto
type Coupon struct {
	ID        int       `json:"id"`
	ShopID    int       `json:"shop_id"`
	Code      string    `json:"code"`
	Type      string    `json:"type"` // "percentage" ou "fixed"
	Value     float64   `json:"value"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

// Order representa um pedido de cliente gravado no banco
type Order struct {
	ID             int         `json:"id"`
	ShopID         int         `json:"shop_id"`
	CustomerName   string      `json:"customer_name"`
	DeliveryMethod string      `json:"delivery_method"` // "delivery" ou "pickup"
	Address        string      `json:"address"`
	PaymentMethod  string      `json:"payment_method"`
	CouponCode     string      `json:"coupon_code"`
	Discount       float64     `json:"discount"`
	Subtotal       float64     `json:"subtotal"`
	Total          float64     `json:"total"`
	Status         string      `json:"status"` // "Pendente", "Preparando", "Enviado", "Concluido", "Cancelado"
	CreatedAt      time.Time   `json:"created_at"`
	Items          []OrderItem `json:"items,omitempty"`
}

// OrderItem representa um item associado a um pedido
type OrderItem struct {
	ID        int     `json:"id"`
	OrderID   int     `json:"order_id"`
	ProductID *int    `json:"product_id"`
	Name      string  `json:"name"`
	Price     float64 `json:"price"`
	Qty       int     `json:"qty"`
	Note      string  `json:"note"`
	Options   *string `json:"options"` // JSON string
}

// DailySales representa dados consolidados diários para gráficos
type DailySales struct {
	Date       time.Time `json:"date"`
	DayName    string    `json:"day_name"`
	TotalSales float64   `json:"total_sales"`
	OrderCount int       `json:"order_count"`
}

// ShopWithOwner representa os dados da loja mesclados com dados do usuário dono
type ShopWithOwner struct {
	ID             int       `json:"id"`
	UserID         int       `json:"user_id"`
	Name           string    `json:"name"`
	Slug           string    `json:"slug"`
	WhatsappNumber string    `json:"whatsapp_number"`
	LogoURL        string    `json:"logo_url"`
	IsActive       bool      `json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
	OwnerName      string    `json:"owner_name"`
	OwnerEmail     string    `json:"owner_email"`
}

// PlatformConfig representa uma configuração global do SaaS
type PlatformConfig struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PlatformAuditLog representa um log de auditoria global da plataforma
type PlatformAuditLog struct {
	ID        int       `json:"id"`
	Action    string    `json:"action"`
	Details   string    `json:"details"`
	CreatedAt time.Time `json:"created_at"`
}




