-- =============================================================
-- Catálogo Digital SaaS - Schema do Banco de Dados
-- =============================================================

-- Extensão para UUIDs (opcional, útil para sessões)
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Tabela de Lojistas/Administradores
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Tabela de Lojas (Tenants)
CREATE TABLE IF NOT EXISTS shops (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) UNIQUE NOT NULL,
    whatsapp_number VARCHAR(20) NOT NULL,
    logo_url TEXT DEFAULT '',
    primary_color VARCHAR(7) DEFAULT '#3B82F6',
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Tabela de Categorias dos Produtos
CREATE TABLE IF NOT EXISTS categories (
    id SERIAL PRIMARY KEY,
    shop_id INTEGER REFERENCES shops(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    position INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Tabela de Produtos
CREATE TABLE IF NOT EXISTS products (
    id SERIAL PRIMARY KEY,
    shop_id INTEGER REFERENCES shops(id) ON DELETE CASCADE,
    category_id INTEGER REFERENCES categories(id) ON DELETE SET NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT DEFAULT '',
    price DECIMAL(10, 2) NOT NULL,
    image_url TEXT DEFAULT '',
    is_available BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Tabela de Sessões (autenticação)
CREATE TABLE IF NOT EXISTS sessions (
    id VARCHAR(64) PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL
);

-- Índices para performance
CREATE INDEX IF NOT EXISTS idx_shops_slug ON shops(slug);
CREATE INDEX IF NOT EXISTS idx_shops_user_id ON shops(user_id);
CREATE INDEX IF NOT EXISTS idx_products_shop_id ON products(shop_id);
CREATE INDEX IF NOT EXISTS idx_products_category_id ON products(category_id);
CREATE INDEX IF NOT EXISTS idx_categories_shop_id ON categories(shop_id);
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);

-- =============================================================
-- Alterações para Catálogo Comercial (SaaS Completo)
-- =============================================================

-- Novas colunas em shops
ALTER TABLE shops ADD COLUMN IF NOT EXISTS banner_url TEXT DEFAULT '';
ALTER TABLE shops ADD COLUMN IF NOT EXISTS delivery_fee DECIMAL(10,2) DEFAULT 0.00;
ALTER TABLE shops ADD COLUMN IF NOT EXISTS business_hours JSONB DEFAULT NULL;

-- Novas colunas em produtos
ALTER TABLE products ADD COLUMN IF NOT EXISTS options JSONB DEFAULT NULL;

-- Tabela de Cupons
CREATE TABLE IF NOT EXISTS coupons (
    id SERIAL PRIMARY KEY,
    shop_id INTEGER REFERENCES shops(id) ON DELETE CASCADE,
    code VARCHAR(50) NOT NULL,
    type VARCHAR(10) NOT NULL, -- 'percentage' ou 'fixed'
    value DECIMAL(10,2) NOT NULL,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT unique_shop_coupon UNIQUE (shop_id, code)
);

-- Tabela de Pedidos
CREATE TABLE IF NOT EXISTS orders (
    id SERIAL PRIMARY KEY,
    shop_id INTEGER REFERENCES shops(id) ON DELETE CASCADE,
    customer_name VARCHAR(255) NOT NULL,
    delivery_method VARCHAR(50) NOT NULL, -- 'delivery' ou 'pickup'
    address TEXT DEFAULT '',
    payment_method VARCHAR(50) NOT NULL,
    coupon_code VARCHAR(50) DEFAULT '',
    discount DECIMAL(10,2) DEFAULT 0.00,
    subtotal DECIMAL(10,2) NOT NULL,
    total DECIMAL(10,2) NOT NULL,
    status VARCHAR(50) DEFAULT 'Pendente', -- 'Pendente', 'Preparando', 'Enviado', 'Concluido', 'Cancelado'
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Tabela de Itens de Pedidos
CREATE TABLE IF NOT EXISTS order_items (
    id SERIAL PRIMARY KEY,
    order_id INTEGER REFERENCES orders(id) ON DELETE CASCADE,
    product_id INTEGER REFERENCES products(id) ON DELETE SET NULL,
    name VARCHAR(255) NOT NULL,
    price DECIMAL(10,2) NOT NULL,
    qty INTEGER NOT NULL,
    note TEXT DEFAULT '',
    options JSONB DEFAULT NULL -- Opções escolhidas pelo cliente (ex: tamanho, adicionais)
);

-- Índices adicionais para performance
CREATE INDEX IF NOT EXISTS idx_coupons_shop_id ON coupons(shop_id);
CREATE INDEX IF NOT EXISTS idx_orders_shop_id ON orders(shop_id);
CREATE INDEX IF NOT EXISTS idx_order_items_order_id ON order_items(order_id);

-- Suporte a múltiplas fotos de produtos
ALTER TABLE products ADD COLUMN IF NOT EXISTS images JSONB DEFAULT NULL;

-- Suporte a Super Admin (Dono da Plataforma)
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_super_admin BOOLEAN DEFAULT FALSE;

-- Promove o usuário administrador a Super Admin
UPDATE users SET is_super_admin = TRUE WHERE email = 'admin@admin.com';

-- =============================================================
-- Subsystem SaaS - Tabelas Exclusivas do Admin Mestre
-- =============================================================

CREATE TABLE IF NOT EXISTS platform_configs (
    key VARCHAR(255) PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS platform_audit_logs (
    id SERIAL PRIMARY KEY,
    action VARCHAR(255) NOT NULL,
    details TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Configurações padrão da plataforma
INSERT INTO platform_configs (key, value) VALUES 
('platform_name', 'Catálogo Digital SaaS') ON CONFLICT DO NOTHING;
INSERT INTO platform_configs (key, value) VALUES 
('maintenance_mode', 'false') ON CONFLICT DO NOTHING;
INSERT INTO platform_configs (key, value) VALUES 
('global_subscription_fee', '49.90') ON CONFLICT DO NOTHING;
INSERT INTO platform_configs (key, value) VALUES 
('support_whatsapp', '5511999999999') ON CONFLICT DO NOTHING;

-- =============================================================
-- SaaS Subscription Plans
-- =============================================================

CREATE TABLE IF NOT EXISTS plans (
    id SERIAL PRIMARY KEY,
    name VARCHAR(50) NOT NULL UNIQUE,
    price DECIMAL(10, 2) NOT NULL,
    max_products INTEGER NOT NULL,    -- -1 para ilimitado
    max_categories INTEGER NOT NULL,  -- -1 para ilimitado
    features JSONB DEFAULT NULL       -- Ex: {"coupons": true, "business_hours": true}
);

-- Popula os planos iniciais
INSERT INTO plans (id, name, price, max_products, max_categories, features) VALUES
(1, 'Bronze (Grátis)', 0.00, 10, 3, '{"coupons": false, "business_hours": false}'),
(2, 'Prata (Profissional)', 49.90, 100, 10, '{"coupons": true, "business_hours": true}'),
(3, 'Ouro (Ilimitado)', 89.90, -1, -1, '{"coupons": true, "business_hours": true}')
ON CONFLICT (id) DO UPDATE SET 
    price = EXCLUDED.price, 
    max_products = EXCLUDED.max_products, 
    max_categories = EXCLUDED.max_categories, 
    features = EXCLUDED.features;

-- Associa plano e data de expiração à tabela de shops
ALTER TABLE shops ADD COLUMN IF NOT EXISTS plan_id INTEGER REFERENCES plans(id) DEFAULT 1;
ALTER TABLE shops ADD COLUMN IF NOT EXISTS plan_expires_at TIMESTAMP WITH TIME ZONE DEFAULT NULL;





