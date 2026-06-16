package mail

import (
	"bytes"
	"catalogo/internal/database"
	"encoding/json"
	"fmt"
	"log"
	"net/smtp"
	"strings"
)

type Mailer struct {
	Host string
	Port string
	User string
	Pass string
	From string
}

type ChosenChoice struct {
	Name        string  `json:"name"`
	PriceAdjust float64 `json:"price_adjust"`
}

func NewMailer(host, port, user, pass, from string) *Mailer {
	return &Mailer{
		Host: host,
		Port: port,
		User: user,
		Pass: pass,
		From: from,
	}
}

func formatBRL(val float64) string {
	return fmt.Sprintf("R$ %.2f", val)
}

// SendOrderNotification envia ou simula a notificação de novo pedido
func (m *Mailer) SendOrderNotification(shop *database.Shop, order *database.Order, items []database.OrderItem, recipientEmail string) error {
	subject := fmt.Sprintf("🛍️ Novo Pedido #%d Recebido - %s", order.ID, shop.Name)

	// Formatação do conteúdo em HTML
	var body bytes.Buffer

	// Estilos e Estrutura HTML Premium
	body.WriteString(`<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Novo Pedido Recebido</title>
    <style>
        body {
            font-family: 'Helvetica Neue', Helvetica, Arial, sans-serif;
            background-color: #f3f4f6;
            margin: 0;
            padding: 0;
            -webkit-font-smoothing: antialiased;
        }
        .container {
            max-width: 600px;
            margin: 20px auto;
            background-color: #ffffff;
            border-radius: 16px;
            overflow: hidden;
            box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06);
        }
        .header {
            background-color: #0f172a;
            padding: 32px 24px;
            text-align: center;
            border-bottom: 4px solid #8b5cf6;
        }
        .header h1 {
            color: #ffffff;
            margin: 0;
            font-size: 24px;
            font-weight: 800;
            letter-spacing: -0.5px;
        }
        .header p {
            color: #94a3b8;
            margin: 8px 0 0 0;
            font-size: 14px;
        }
        .content {
            padding: 24px;
        }
        .section-title {
            font-size: 12px;
            font-weight: 700;
            text-transform: uppercase;
            letter-spacing: 1.5px;
            color: #64748b;
            margin-bottom: 12px;
            border-bottom: 1px solid #e2e8f0;
            padding-bottom: 6px;
        }
        .info-grid {
            margin-bottom: 24px;
        }
        .info-row {
            display: flex;
            justify-content: space-between;
            padding: 8px 0;
            font-size: 14px;
            border-bottom: 1px dashed #f1f5f9;
        }
        .info-label {
            color: #64748b;
            font-weight: 600;
        }
        .info-value {
            color: #0f172a;
            font-weight: 750;
            text-align: right;
        }
        .items-table {
            width: 100%;
            border-collapse: collapse;
            margin-bottom: 24px;
        }
        .items-table th {
            text-align: left;
            font-size: 12px;
            font-weight: 700;
            color: #64748b;
            text-transform: uppercase;
            padding: 8px;
            background-color: #f8fafc;
        }
        .items-table td {
            padding: 12px 8px;
            border-bottom: 1px solid #f1f5f9;
            font-size: 14px;
        }
        .item-name {
            font-weight: 600;
            color: #0f172a;
        }
        .item-options {
            font-size: 12px;
            color: #7c3aed;
            margin-top: 4px;
            font-style: italic;
        }
        .item-note {
            font-size: 12px;
            color: #dc2626;
            margin-top: 4px;
        }
        .totals-box {
            background-color: #f8fafc;
            border-radius: 12px;
            padding: 16px;
            margin-bottom: 24px;
        }
        .total-row {
            display: flex;
            justify-content: space-between;
            padding: 6px 0;
            font-size: 14px;
        }
        .total-row.grand-total {
            border-top: 1px solid #e2e8f0;
            margin-top: 8px;
            padding-top: 12px;
            font-size: 18px;
            font-weight: 800;
            color: #7c3aed;
        }
        .footer-action {
            text-align: center;
            margin-top: 32px;
            margin-bottom: 16px;
        }
        .btn-primary {
            display: inline-block;
            background-color: #8b5cf6;
            color: #ffffff !important;
            text-decoration: none;
            padding: 14px 28px;
            border-radius: 12px;
            font-weight: 700;
            font-size: 14px;
            box-shadow: 0 4px 6px -1px rgba(139, 92, 246, 0.2), 0 2px 4px -1px rgba(139, 92, 246, 0.1);
            transition: background-color 0.2s;
        }
        .btn-primary:hover {
            background-color: #7c3aed;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Cataloger</h1>
            <p>Novo Pedido Recebido com Sucesso!</p>
        </div>
        <div class="content">
            <div class="section-title">Resumo do Pedido</div>
            <div class="info-grid">
                <div class="info-row">
                    <span class="info-label">Pedido Nº</span>
                    <span class="info-value">#` + fmt.Sprintf("%d", order.ID) + `</span>
                </div>
                <div class="info-row">
                    <span class="info-label">Cliente</span>
                    <span class="info-value">` + order.CustomerName + `</span>
                </div>
                <div class="info-row">
                    <span class="info-label">WhatsApp</span>
                    <span class="info-value">` + order.CustomerPhone + `</span>
                </div>
                <div class="info-row">
                    <span class="info-label">Pagamento</span>
                    <span class="info-value">` + formatPaymentMethod(order.PaymentMethod) + `</span>
                </div>
                <div class="info-row">
                    <span class="info-label">Método de Entrega</span>
                    <span class="info-value">` + formatDeliveryMethod(order.DeliveryMethod) + `</span>
                </div>`)

	if order.DeliveryMethod == "entrega" && order.Address != "" {
		body.WriteString(`
                <div class="info-row" style="flex-direction: column; align-items: flex-start; border-bottom: none;">
                    <span class="info-label" style="margin-bottom: 4px;">Endereço de Entrega</span>
                    <span class="info-value" style="text-align: left; font-weight: normal; font-size: 13px; color: #334155;">` + order.Address + `</span>
                </div>`)
	}

	body.WriteString(`
            </div>

            <div class="section-title">Produtos</div>
            <table class="items-table">
                <thead>
                    <tr>
                        <th>Produto</th>
                        <th style="text-align: center;">Qtd</th>
                        <th style="text-align: right;">Total</th>
                    </tr>
                </thead>
                <tbody>`)

	for _, item := range items {
		body.WriteString("<tr><td><div class=\"item-name\">" + item.Name + "</div>")

		// Opções adicionais
		if item.Options != nil && *item.Options != "" && *item.Options != "{}" {
			var chosen map[string]json.RawMessage
			var optTexts []string
			if err := json.Unmarshal([]byte(*item.Options), &chosen); err == nil {
				for optKey, raw := range chosen {
					var list []ChosenChoice
					if err := json.Unmarshal(raw, &list); err == nil {
						var subOpts []string
						for _, c := range list {
							subOpts = append(subOpts, c.Name)
						}
						optTexts = append(optTexts, fmt.Sprintf("%s: %s", optKey, strings.Join(subOpts, ", ")))
					} else {
						var c ChosenChoice
						if err := json.Unmarshal(raw, &c); err == nil {
							optTexts = append(optTexts, fmt.Sprintf("%s: %s", optKey, c.Name))
						}
					}
				}
			}
			if len(optTexts) > 0 {
				body.WriteString("<div class=\"item-options\">⚙️ " + strings.Join(optTexts, " | ") + "</div>")
			}
		}

		// Observação individual
		if item.Note != "" {
			body.WriteString("<div class=\"item-note\">📝 " + item.Note + "</div>")
		}

		body.WriteString(fmt.Sprintf(`</td>
                        <td style="text-align: center; font-weight: bold; color: #475569;">%d</td>
                        <td style="text-align: right; font-weight: 600; color: #0f172a;">%s</td>
                    </tr>`, item.Qty, formatBRL(item.Price*float64(item.Qty))))
	}

	body.WriteString(`
                </tbody>
            </table>

            <div class="totals-box">
                <div class="total-row">
                    <span style="color: #64748b;">Subtotal</span>
                    <span style="font-weight: 600; color: #334155;">` + formatBRL(order.Subtotal) + `</span>
                </div>`)

	if order.Discount > 0 {
		body.WriteString(`
                <div class="total-row">
                    <span style="color: #64748b;">Desconto</span>
                    <span style="font-weight: 600; color: #16a34a;">-` + formatBRL(order.Discount) + `</span>
                </div>`)
	}

	if order.DeliveryMethod == "entrega" {
		body.WriteString(`
                <div class="total-row">
                    <span style="color: #64748b;">Taxa de Entrega</span>
                    <span style="font-weight: 600; color: #334155;">` + formatBRL(shop.DeliveryFee) + `</span>
                </div>`)
	}

	body.WriteString(`
                <div class="total-row grand-total">
                    <span>Total</span>
                    <span>` + formatBRL(order.Total) + `</span>
                </div>
            </div>

            <div class="footer-action">
                <a href="http://localhost:8080/admin/pedidos" class="btn-primary">Ver no Painel do Lojista</a>
            </div>
        </div>
    </div>
</body>
</html>`)

	htmlBody := body.String()

	// Se o SMTPHost estiver vazio, executamos a simulação
	if m.Host == "" || m.User == "" {
		log.Println("================================================================================")
		log.Printf("[SIMULAÇÃO DE E-MAIL] Para: %s", recipientEmail)
		log.Printf("[SIMULAÇÃO DE E-MAIL] Assunto: %s", subject)
		log.Println("[SIMULAÇÃO DE E-MAIL] Conteúdo HTML Gerado:")
		log.Println(htmlBody)
		log.Println("================================================================================")
		return nil
	}

	// Lógica de envio real
	auth := smtp.PlainAuth("", m.User, m.Pass, m.Host)
	addr := fmt.Sprintf("%s:%s", m.Host, m.Port)

	// Construção do cabeçalho de e-mail SMTP HTML
	header := make(map[string]string)
	header["From"] = m.From
	header["To"] = recipientEmail
	header["Subject"] = subject
	header["MIME-Version"] = "1.0"
	header["Content-Type"] = "text/html; charset=\"UTF-8\""

	var message bytes.Buffer
	for k, v := range header {
		message.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	message.WriteString("\r\n")
	message.WriteString(htmlBody)

	err := smtp.SendMail(addr, auth, m.From, []string{recipientEmail}, message.Bytes())
	if err != nil {
		return fmt.Errorf("falha ao enviar e-mail via SMTP: %w", err)
	}

	return nil
}

func formatPaymentMethod(m string) string {
	paymentLabels := map[string]string{
		"pix":      "Pix",
		"cartao":   "Cartão de Crédito/Débito",
		"dinheiro": "Dinheiro",
	}
	if lbl, ok := paymentLabels[m]; ok {
		return lbl
	}
	return m
}

func formatDeliveryMethod(m string) string {
	deliveryLabels := map[string]string{
		"entrega":  "Entrega ao Cliente",
		"retirada": "Retirada no Local",
	}
	if lbl, ok := deliveryLabels[m]; ok {
		return lbl
	}
	return m
}

// SendOrderStatusUpdateEmail envia um e-mail de atualização de status do pedido para o cliente
func (m *Mailer) SendOrderStatusUpdateEmail(shop *database.Shop, order *database.Order, recipientEmail string) error {
	subject := fmt.Sprintf("🔔 Atualização do seu Pedido #%d - %s", order.ID, shop.Name)

	var body bytes.Buffer

	// HTML Layout
	body.WriteString(`<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Atualização de Pedido</title>
    <style>
        body {
            font-family: 'Helvetica Neue', Helvetica, Arial, sans-serif;
            background-color: #f3f4f6;
            margin: 0;
            padding: 0;
            -webkit-font-smoothing: antialiased;
        }
        .container {
            max-width: 600px;
            margin: 20px auto;
            background-color: #ffffff;
            border-radius: 16px;
            overflow: hidden;
            box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06);
        }
        .header {
            background-color: #0f172a;
            padding: 32px 24px;
            text-align: center;
            border-bottom: 4px solid #8b5cf6;
        }
        .header h1 {
            color: #ffffff;
            margin: 0;
            font-size: 24px;
            font-weight: 800;
            letter-spacing: -0.5px;
        }
        .content {
            padding: 32px 24px;
            text-align: center;
        }
        .status-badge {
            display: inline-block;
            padding: 8px 18px;
            border-radius: 9999px;
            font-weight: 800;
            font-size: 14px;
            margin: 20px 0;
            text-transform: uppercase;
            letter-spacing: 1px;
        }
        .status-Pendente {
            background-color: #fef3c7;
            color: #d97706;
            border: 1px solid #fde68a;
        }
        .status-Preparando {
            background-color: #dbeafe;
            color: #2563eb;
            border: 1px solid #bfdbfe;
        }
        .status-Enviado {
            background-color: #f3e8ff;
            color: #7c3aed;
            border: 1px solid #e9d5ff;
        }
        .status-Concluido {
            background-color: #d1fae5;
            color: #059669;
            border: 1px solid #a7f3d0;
        }
        .status-Cancelado {
            background-color: #fee2e2;
            color: #dc2626;
            border: 1px solid #fecaca;
        }
        .message-box {
            font-size: 16px;
            color: #334155;
            line-height: 1.6;
            margin-bottom: 24px;
        }
        .totals-box {
            background-color: #f8fafc;
            border-radius: 12px;
            padding: 16px;
            max-width: 300px;
            margin: 0 auto 24px auto;
            font-size: 14px;
            text-align: left;
        }
        .total-row {
            display: flex;
            justify-content: space-between;
            padding: 4px 0;
        }
        .footer-action {
            margin-top: 32px;
            margin-bottom: 16px;
        }
        .btn-primary {
            display: inline-block;
            background-color: #8b5cf6;
            color: #ffffff !important;
            text-decoration: none;
            padding: 14px 28px;
            border-radius: 12px;
            font-weight: 700;
            font-size: 14px;
            box-shadow: 0 4px 6px -1px rgba(139, 92, 246, 0.2), 0 2px 4px -1px rgba(139, 92, 246, 0.1);
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>` + shop.Name + `</h1>
        </div>
        <div class="content">
            <div class="message-box">
                Olá, <strong>` + order.CustomerName + `</strong>!<br>
                Temos novidades sobre o seu pedido realizado em nossa loja.
            </div>
            
            <div class="message-box" style="font-size: 14px; color: #64748b;">
                O status do seu pedido <strong>#` + fmt.Sprintf("%d", order.ID) + `</strong> foi atualizado para:
            </div>
            
            <div class="status-badge status-` + order.Status + `">
                ` + order.Status + `
            </div>
            
            <div class="message-box">
                ` + getFriendlyStatusMessage(order.Status) + `
            </div>

            <div class="totals-box">
                <div class="total-row">
                    <span style="color: #64748b;">Método:</span>
                    <span style="font-weight: 600; color: #334155;">` + formatDeliveryMethod(order.DeliveryMethod) + `</span>
                </div>
                <div class="total-row">
                    <span style="color: #64748b;">Total do Pedido:</span>
                    <span style="font-weight: 800; color: #7c3aed;">` + formatBRL(order.Total) + `</span>
                </div>
            </div>

            <div class="footer-action">
                <a href="http://localhost:8080/` + shop.Slug + `" class="btn-primary">Acessar Nossa Loja</a>
            </div>
        </div>
    </div>
</body>
</html>`)

	htmlBody := body.String()

	// Simulation mode fallback
	if m.Host == "" || m.User == "" {
		log.Println("================================================================================")
		log.Printf("[SIMULAÇÃO DE E-MAIL] Para Cliente: %s", recipientEmail)
		log.Printf("[SIMULAÇÃO DE E-MAIL] Assunto: %s", subject)
		log.Println("[SIMULAÇÃO DE E-MAIL] Conteúdo HTML Gerado:")
		log.Println(htmlBody)
		log.Println("================================================================================")
		return nil
	}

	auth := smtp.PlainAuth("", m.User, m.Pass, m.Host)
	addr := fmt.Sprintf("%s:%s", m.Host, m.Port)

	header := make(map[string]string)
	header["From"] = m.From
	header["To"] = recipientEmail
	header["Subject"] = subject
	header["MIME-Version"] = "1.0"
	header["Content-Type"] = "text/html; charset=\"UTF-8\""

	var message bytes.Buffer
	for k, v := range header {
		message.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	message.WriteString("\r\n")
	message.WriteString(htmlBody)

	err := smtp.SendMail(addr, auth, m.From, []string{recipientEmail}, message.Bytes())
	if err != nil {
		return fmt.Errorf("falha ao enviar e-mail de atualização via SMTP: %w", err)
	}

	return nil
}

func getFriendlyStatusMessage(status string) string {
	switch status {
	case "Pendente":
		return "Seu pedido foi recebido e está aguardando análise."
	case "Preparando":
		return "Seu pedido já está em preparação! Estamos preparando tudo com muito carinho. 🍳"
	case "Enviado":
		return "Seu pedido foi enviado! Ele já está a caminho do seu endereço. 🛵"
	case "Concluido":
		return "Seu pedido foi concluído! Esperamos que você goste dos produtos. Obrigado pela preferência! ✨"
	case "Cancelado":
		return "Seu pedido foi cancelado pelo lojista. Se tiver dúvidas, entre em contato pelo nosso WhatsApp."
	default:
		return "O status do seu pedido foi alterado."
	}
}
