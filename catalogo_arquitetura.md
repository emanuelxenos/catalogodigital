# Arquitetura e Guia de Implementação: Catálogo Digital via WhatsApp

Este documento serve como especificação técnica detalhada para a criação de um sistema de catálogo digital multi-tenant (SaaS) utilizando **Go (Golang), HTMX, Alpine.js e Tailwind CSS**. O objetivo é permitir que lojistas gerenciem seus produtos e compartilhem links, e que clientes montem pedidos em um carrinho reativo local e enviem diretamente para o WhatsApp do lojista.

---

## 1. Stack Tecnológica

*   **Backend:** Go (Golang) utilizando o roteador `go-chi/chi` (ótimo suporte para subdomínios/parâmetros de rota).
*   **Banco de Dados:** PostgreSQL (gerenciado no Go com `sqlc` ou `pgx` para queries de alta performance).
*   **Frontend (SSR):** Go `html/template` nativo.
*   **Reatividade Servidor-Cliente:** HTMX (carregamento dinâmico de partes da página, filtros e paginação sem reload).
*   **Reatividade Local (Carrinho):** Alpine.js (armazenamento e manipulação do carrinho de compras no `localStorage`).
*   **Estilização:** Tailwind CSS (foco em design moderno, mobile-first e responsivo).

---

## 2. Estrutura de Diretórios Recomendada

```text
├── cmd/
│   └── web/
│       └── main.go           # Inicialização do servidor, DB e rotas
├── internal/
│   ├── config/               # Configurações de ambiente (.env)
│   ├── database/             # Conexão e queries auto-geradas (sqlc)
│   │   ├── db.go
│   │   ├── models.go
│   │   └── query.sql.go
│   ├── handlers/             # Controladores (Handlers HTTP de Go)
│   │   ├── admin.go          # Painel do lojista
│   │   ├── catalog.go        # Catálogo público da loja
│   │   └── home.go           # Página inicial do SaaS
│   └── middleware/           # Autenticação e logs
├── templates/                # Arquivos HTML estruturados
│   ├── layouts/
│   │   ├── base.html         # Template base com HTMX, Alpine e Tailwind carregados
│   │   └── admin.html        # Template base do painel administrativo
│   ├── admin/                # Views do painel (produtos, configurações)
│   ├── catalog/              # Views do catálogo público
│   └── partials/             # Blocos reutilizáveis (ex: cards de produtos, itens do carrinho)
├── public/                   # Arquivos estáticos
│   ├── css/                  # Tailwind CSS compilado
│   └── js/                   # Scripts auxiliares
├── sql/                      # Migrações do banco de dados (schema.sql e queries.sql)
├── go.mod
└── go.sum
```

---

## 3. Modelagem do Banco de Dados (PostgreSQL)

```sql
-- Tabela de Lojistas/Administradores
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Tabela de Lojas (Tenants)
CREATE TABLE shops (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) UNIQUE NOT NULL, -- Usado na URL: catalogo.com/slug-da-loja
    whatsapp_number VARCHAR(20) NOT NULL, -- Formato internacional (Ex: 5511999999999)
    logo_url TEXT,
    primary_color VARCHAR(7) DEFAULT '#3B82F6', -- Cor customizável da marca (Hex)
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Tabela de Categorias dos Produtos
CREATE TABLE categories (
    id SERIAL PRIMARY KEY,
    shop_id INTEGER REFERENCES shops(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    position INTEGER DEFAULT 0, -- Ordenação das categorias na tela
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Tabela de Produtos
CREATE TABLE products (
    id SERIAL PRIMARY KEY,
    shop_id INTEGER REFERENCES shops(id) ON DELETE CASCADE,
    category_id INTEGER REFERENCES categories(id) ON DELETE SET NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    price DECIMAL(10, 2) NOT NULL,
    image_url TEXT,
    is_available BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
```

---

## 4. Fluxos de Funcionamento

### A. Catálogo Público (`/{shop_slug}`)
1. O cliente acessa o link da loja. O Go renderiza a página base buscando a loja e os produtos ativos no banco através do `shop_slug`.
2. As categorias são exibidas em um carrossel horizontal no topo.
3. Clicar em uma categoria dispara um comando **HTMX** (`hx-get="/{shop_slug}/produtos?categoria=ID"`) que retorna apenas o HTML correspondente à lista de produtos filtrados, substituindo a div de destino sem recarregar o cabeçalho ou o estado do carrinho.

### B. Carrinho de Compras Reativo (Alpine.js)
1. O estado do carrinho (`itens`, `total`, `quantidade`) reside puramente no cliente usando Alpine.js e persistido no `localStorage`.
2. Cada card de produto possui um botão `@click="adicionarAoCarrinho(produto)"`.
3. Um botão flutuante no canto inferior direito mostra a quantidade de itens no carrinho. Ao clicar, abre um painel lateral (*Drawer*) com o resumo do pedido.
4. Dentro do Drawer, o cliente pode aumentar ou diminuir as quantidades, disparando atualizações reativas automáticas no valor total.

### C. Finalização do Pedido (Envio para o WhatsApp)
1. No Drawer do carrinho, o cliente preenche um formulário básico:
   * Nome
   * Método de recebimento (Entrega / Retirada)
   * Endereço completo (se Entrega)
   * Forma de pagamento pretendida (Pix, Cartão, Dinheiro)
2. Ao clicar no botão **"Enviar Pedido via WhatsApp"**, o Alpine.js executa uma função que:
   * Formata uma mensagem em Markdown do WhatsApp com a lista de itens, quantidades, valores e dados de entrega.
   * Cria o link URL de redirecionamento:
     `https://wa.me/TELEFONE_DA_LOJA?text=MENSAGEM_FORMATADA_E_CODIFICADA`
   * Abre o link em uma nova guia, iniciando a conversa com o pedido pronto.

### D. Painel de Controle do Lojista (`/admin`)
1. Autenticação básica via sessão (cookie seguro).
2. O lojista pode editar o perfil da loja (cor da marca, número de WhatsApp e logotipo).
3. Tela de Gerenciamento de Produtos:
   * Cadastro, edição e exclusão de itens e categorias.
   * Upload de fotos de produtos para o servidor Go ou integrador de CDN (S3, Cloudinary).
   * Uso de **HTMX** para deletar ou atualizar a disponibilidade do produto instantaneamente na lista da tabela sem dar F5.

---

## 5. Diretrizes de Design UI/UX (Foco em Visual Premium)

Para garantir uma interface que cause um impacto visual fortíssimo no primeiro acesso:

*   **Paleta de Cores Dinâmica:** A cor principal da interface (botões de ação, contornos ativos, botão do WhatsApp) deve ser baseada no campo `primary_color` da tabela `shops` configurado pelo lojista.
*   **Estilo Visual Moderno:**
    *   Uso de **Glassmorphism** (fundos levemente translúcidos com `backdrop-blur-md` e bordas finas semi-transparentes) para modais e o menu lateral do carrinho.
    *   Sombras suaves (`shadow-xl`) e bordas bem arredondadas (`rounded-2xl`) nos cards de produtos.
    *   Layout escuro (Dark Mode) opcional ou paletas suaves e elegantes baseadas em HSL (evitar cores puras e saturadas como vermelho puro ou azul puro).
*   **Tipografia:** Utilizar a fonte **Inter** ou **Outfit** (via Google Fonts) para um aspecto moderno e limpo.
*   **Micro-interações:**
    *   Efeito de escala suave nos cards de produtos ao passar o mouse (`transition-all duration-300 hover:-translate-y-1 hover:shadow-2xl`).
    *   Animação de pulso sutil no botão flutuante do carrinho quando um item é adicionado.
*   **Design Mobile-First:** O catálogo deve ser projetado pensando prioritariamente na tela de um celular, simulando um Web App nativo com navegação rápida pelo polegar.

---

## 6. Exemplo de Código do Template Base (`templates/layouts/base.html`)

```html
<!DOCTYPE html>
<html lang="pt-BR" class="h-full bg-slate-50">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{ .Shop.Name }} | Catálogo</title>
    <!-- Meta tags dinâmicas para compartilhamento -->
    <meta property="og:title" content="Confira o catálogo de {{ .Shop.Name }}">
    <meta property="og:description" content="Faça seu pedido diretamente pelo nosso catálogo digital e envie no WhatsApp!">
    <meta property="og:image" content="{{ .Shop.LogoURL }}">
    <!-- Google Fonts -->
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=Outfit:wght@300;400;600;700&display=swap" rel="stylesheet">
    <style>
        body { font-family: 'Outfit', sans-serif; }
    </style>
    <!-- Tailwind CSS -->
    <script src="https://cdn.tailwindcss.com"></script>
    <!-- HTMX -->
    <script src="https://unpkg.com/htmx.org@1.9.10"></script>
    <!-- Alpine.js -->
    <script defer src="https://unpkg.com/alpinejs@3.13.5/dist/cdn.min.js"></script>
</head>
<body class="h-full text-slate-800" x-data="carrinhoGlobal()">

    <!-- O conteúdo da página entra aqui -->
    <main class="pb-24">
        {{ embed }}
    </main>

</body>
</html>
```
