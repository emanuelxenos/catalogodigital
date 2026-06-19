package asaas

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client encapsula a comunicação com a API do Asaas
type Client struct {
	APIKey  string
	BaseURL string
	http    *http.Client
}

// NewClient cria um novo client do Asaas
func NewClient(apiKey, baseURL string) *Client {
	return &Client{
		APIKey:  apiKey,
		BaseURL: baseURL,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

// ==================== STRUCTS DE REQUEST ====================

// CreateCustomerRequest payload para criar cliente
type CreateCustomerRequest struct {
	Name    string `json:"name"`
	Email   string `json:"email,omitempty"`
	Phone   string `json:"mobilePhone,omitempty"`
	GroupID string `json:"groupName,omitempty"`
}

// CreatePixChargeRequest payload para criar cobrança PIX
type CreatePixChargeRequest struct {
	Customer    string  `json:"customer"`
	BillingType string  `json:"billingType"`
	Value       float64 `json:"value"`
	DueDate     string  `json:"dueDate"`
	Description string  `json:"description"`
}

// CreateCardChargeRequest payload para criar cobrança por cartão
type CreateCardChargeRequest struct {
	Customer         string           `json:"customer"`
	BillingType      string           `json:"billingType"`
	Value            float64          `json:"value"`
	DueDate          string           `json:"dueDate"`
	Description      string           `json:"description"`
	CreditCard       CreditCardData   `json:"creditCard"`
	CreditCardHolder CreditCardHolder `json:"creditCardHolderInfo"`
	RemoteIP         string           `json:"remoteIp"`
}

// CreditCardData dados do cartão
type CreditCardData struct {
	HolderName  string `json:"holderName"`
	Number      string `json:"number"`
	ExpiryMonth string `json:"expiryMonth"`
	ExpiryYear  string `json:"expiryYear"`
	Ccv         string `json:"ccv"`
}

// CreditCardHolder informações do titular
type CreditCardHolder struct {
	Name          string `json:"name"`
	Email         string `json:"email"`
	CpfCnpj       string `json:"cpfCnpj"`
	Phone         string `json:"phone,omitempty"`
	PostalCode    string `json:"postalCode"`
	AddressNumber string `json:"addressNumber"`
}

// ==================== STRUCTS DE RESPOSTA ====================

// CustomerResponse resposta da criação de cliente
type CustomerResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// ChargeResponse resposta da criação de cobrança
type ChargeResponse struct {
	ID          string  `json:"id"`
	Status      string  `json:"status"`
	Value       float64 `json:"value"`
	BillingType string  `json:"billingType"`
	DueDate     string  `json:"dueDate"`
	InvoiceURL  string  `json:"invoiceUrl"`
}

// PixQRCodeResponse resposta do QR Code PIX
type PixQRCodeResponse struct {
	EncodedImage string `json:"encodedImage"` // base64 da imagem PNG
	Payload      string `json:"payload"`      // código copia-cola
	ExpirationDate string `json:"expirationDate"`
}

// WebhookPayment estrutura do evento de webhook do Asaas
type WebhookPayment struct {
	Event   string `json:"event"` // ex: PAYMENT_RECEIVED
	Payment struct {
		ID          string  `json:"id"`
		Status      string  `json:"status"`
		Value       float64 `json:"value"`
		Customer    string  `json:"customer"`
		Description string  `json:"description"`
		BillingType string  `json:"billingType"`
		Subscription string `json:"subscription,omitempty"`
	} `json:"payment"`
}

// asaasError representa erro retornado pela API
type asaasError struct {
	Errors []struct {
		Code        string `json:"code"`
		Description string `json:"description"`
	} `json:"errors"`
}

// CreateSubscriptionRequest payload para criar assinatura recorrente
type CreateSubscriptionRequest struct {
	Customer         string            `json:"customer"`
	BillingType      string            `json:"billingType"`
	Value            float64           `json:"value"`
	NextDueDate      string            `json:"nextDueDate"`
	Cycle            string            `json:"cycle"`
	Description      string            `json:"description"`
	CreditCard       *CreditCardData   `json:"creditCard,omitempty"`
	CreditCardHolder *CreditCardHolder `json:"creditCardHolderInfo,omitempty"`
	RemoteIP         string            `json:"remoteIp,omitempty"`
}

// SubscriptionResponse resposta da criação de assinatura
type SubscriptionResponse struct {
	ID          string  `json:"id"`
	Customer    string  `json:"customer"`
	BillingType string  `json:"billingType"`
	Value       float64 `json:"value"`
	Cycle       string  `json:"cycle"`
	Status      string  `json:"status"`
}

// SubscriptionPaymentsResponse resposta da listagem de cobranças de uma assinatura
type SubscriptionPaymentsResponse struct {
	Data []ChargeResponse `json:"data"`
}

// ==================== MÉTODOS ====================

// do executa uma requisição autenticada para a API do Asaas
func (c *Client) do(method, path string, body interface{}) ([]byte, int, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("erro ao serializar body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.BaseURL+path, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("erro ao criar requisição: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("access_token", c.APIKey)
	req.Header.Set("User-Agent", "CatalogoDigitalSaaS/1.0")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("erro na requisição para %s: %w", path, err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("erro ao ler resposta: %w", err)
	}

	// Se não for 2xx, extrai mensagem de erro
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var asErr asaasError
		if jsonErr := json.Unmarshal(respBytes, &asErr); jsonErr == nil && len(asErr.Errors) > 0 {
			return nil, resp.StatusCode, fmt.Errorf("Asaas API [%d]: %s", resp.StatusCode, asErr.Errors[0].Description)
		}
		return nil, resp.StatusCode, fmt.Errorf("Asaas API retornou status %d: %s", resp.StatusCode, string(respBytes))
	}

	return respBytes, resp.StatusCode, nil
}

// CreateCustomer cria ou busca um cliente no Asaas e retorna seu ID
func (c *Client) CreateCustomer(name, email, phone string) (string, error) {
	payload := CreateCustomerRequest{
		Name:  name,
		Email: email,
		Phone: phone,
	}

	data, _, err := c.do("POST", "/customers", payload)
	if err != nil {
		return "", fmt.Errorf("erro ao criar cliente no Asaas: %w", err)
	}

	var resp CustomerResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("erro ao parsear resposta de cliente: %w", err)
	}

	return resp.ID, nil
}

// CreatePixCharge cria uma cobrança PIX e retorna o ID do pagamento
func (c *Client) CreatePixCharge(customerID string, value float64, description string) (*ChargeResponse, error) {
	dueDate := time.Now().AddDate(0, 0, 3).Format("2006-01-02") // Vence em 3 dias

	payload := CreatePixChargeRequest{
		Customer:    customerID,
		BillingType: "PIX",
		Value:       value,
		DueDate:     dueDate,
		Description: description,
	}

	data, _, err := c.do("POST", "/payments", payload)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar cobrança PIX: %w", err)
	}

	var resp ChargeResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("erro ao parsear cobrança PIX: %w", err)
	}

	return &resp, nil
}

// GetPixQRCode busca o QR Code e copia-cola de uma cobrança PIX
func (c *Client) GetPixQRCode(paymentID string) (*PixQRCodeResponse, error) {
	data, _, err := c.do("GET", "/payments/"+paymentID+"/pixQrCode", nil)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar QR Code PIX: %w", err)
	}

	var resp PixQRCodeResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("erro ao parsear QR Code PIX: %w", err)
	}

	return &resp, nil
}

// CreateCardCharge cria uma cobrança via cartão de crédito
func (c *Client) CreateCardCharge(customerID string, value float64, description string, card CreditCardData, holder CreditCardHolder, remoteIP string) (*ChargeResponse, error) {
	dueDate := time.Now().Format("2006-01-02")

	payload := CreateCardChargeRequest{
		Customer:         customerID,
		BillingType:      "CREDIT_CARD",
		Value:            value,
		DueDate:          dueDate,
		Description:      description,
		CreditCard:       card,
		CreditCardHolder: holder,
		RemoteIP:         remoteIP,
	}

	data, _, err := c.do("POST", "/payments", payload)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar cobrança por cartão: %w", err)
	}

	var resp ChargeResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("erro ao parsear cobrança por cartão: %w", err)
	}

	return &resp, nil
}

// GetPaymentStatus consulta o status atual de uma cobrança
func (c *Client) GetPaymentStatus(paymentID string) (string, error) {
	data, _, err := c.do("GET", "/payments/"+paymentID, nil)
	if err != nil {
		return "", fmt.Errorf("erro ao consultar pagamento: %w", err)
	}

	var resp ChargeResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("erro ao parsear status do pagamento: %w", err)
	}

	return resp.Status, nil
}

// CreateSubscription cria uma assinatura recorrente no Asaas
func (c *Client) CreateSubscription(req CreateSubscriptionRequest) (*SubscriptionResponse, error) {
	data, _, err := c.do("POST", "/subscriptions", req)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar assinatura no Asaas: %w", err)
	}

	var resp SubscriptionResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("erro ao parsear resposta de assinatura: %w", err)
	}

	return &resp, nil
}

// GetSubscriptionPayments lista as cobranças vinculadas a uma assinatura
func (c *Client) GetSubscriptionPayments(subscriptionID string) ([]ChargeResponse, error) {
	data, _, err := c.do("GET", "/subscriptions/"+subscriptionID+"/payments", nil)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar cobranças da assinatura no Asaas: %w", err)
	}

	var resp SubscriptionPaymentsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("erro ao parsear cobranças da assinatura: %w", err)
	}

	return resp.Data, nil
}

// CancelSubscription cancela uma assinatura ativa no Asaas
func (c *Client) CancelSubscription(subscriptionID string) error {
	_, _, err := c.do("DELETE", "/subscriptions/"+subscriptionID, nil)
	if err != nil {
		return fmt.Errorf("erro ao cancelar assinatura no Asaas: %w", err)
	}
	return nil
}

