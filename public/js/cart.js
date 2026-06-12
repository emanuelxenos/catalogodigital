// carrinhoGlobal - Alpine.js store para o carrinho de compras
// Persiste no localStorage para manter o estado entre navegações
function carrinhoGlobal() {
    return {
        items: JSON.parse(localStorage.getItem('cart_items') || '[]'),
        isOpen: false,
        customerName: localStorage.getItem('cart_customerName') || '',
        deliveryMethod: localStorage.getItem('cart_deliveryMethod') || 'entrega',
        address: localStorage.getItem('cart_address') || '',
        paymentMethod: localStorage.getItem('cart_paymentMethod') || '',
        justAdded: false,
        toastMessage: '',
        searchQuery: '',
        productModalOpen: false,
        selectedProduct: null,
        modalQty: 1,

        // Salva o estado no localStorage
        save() {
            localStorage.setItem('cart_items', JSON.stringify(this.items));
            localStorage.setItem('cart_customerName', this.customerName);
            localStorage.setItem('cart_deliveryMethod', this.deliveryMethod);
            localStorage.setItem('cart_address', this.address);
            localStorage.setItem('cart_paymentMethod', this.paymentMethod);
        },

        // Adiciona um item ao carrinho
        addItem(product) {
            const existing = this.items.find(i => i.id === product.id);
            if (existing) {
                existing.qty++;
            } else {
                this.items.push({
                    id: product.id,
                    name: product.name,
                    price: product.price,
                    image: product.image || '',
                    qty: 1
                });
            }
            this.save();
            
            // Animação de pulso no FAB
            this.justAdded = true;
            setTimeout(() => { this.justAdded = false; }, 500);
            
            // Toast
            this.showToast('✅ ' + product.name + ' adicionado!');
        },

        // Remove um item do carrinho
        removeItem(id) {
            this.items = this.items.filter(i => i.id !== id);
            this.save();
        },

        // Atualiza a quantidade de um item
        updateQty(id, delta) {
            const item = this.items.find(i => i.id === id);
            if (!item) return;
            
            item.qty += delta;
            if (item.qty <= 0) {
                this.removeItem(id);
            } else {
                this.save();
            }
        },

        // Calcula o total do carrinho
        getTotal() {
            return this.items.reduce((sum, item) => sum + (item.price * item.qty), 0);
        },

        // Conta total de itens
        getTotalItems() {
            return this.items.reduce((sum, item) => sum + item.qty, 0);
        },

        // Formata valor em moeda brasileira
        formatCurrency(value) {
            return new Intl.NumberFormat('pt-BR', {
                style: 'currency',
                currency: 'BRL'
            }).format(value);
        },

        // Formata a mensagem para WhatsApp
        formatWhatsAppMessage() {
            let msg = '🛒 *NOVO PEDIDO*\n';
            msg += '━━━━━━━━━━━━━━━\n\n';
            
            // Itens
            this.items.forEach((item, idx) => {
                msg += `*${idx + 1}.* ${item.name}\n`;
                msg += `   Qtd: ${item.qty} x ${this.formatCurrency(item.price)}\n`;
                msg += `   Subtotal: ${this.formatCurrency(item.price * item.qty)}\n\n`;
            });
            
            msg += '━━━━━━━━━━━━━━━\n';
            msg += `💰 *TOTAL: ${this.formatCurrency(this.getTotal())}*\n`;
            msg += '━━━━━━━━━━━━━━━\n\n';
            
            // Dados do cliente
            msg += `👤 *Nome:* ${this.customerName}\n`;
            
            if (this.deliveryMethod === 'entrega') {
                msg += `🛵 *Método:* Entrega\n`;
                if (this.address) {
                    msg += `📍 *Endereço:* ${this.address}\n`;
                }
            } else {
                msg += `🏪 *Método:* Retirada no local\n`;
            }
            
            const paymentLabels = {
                'pix': '💳 Pix',
                'cartao': '💳 Cartão',
                'dinheiro': '💵 Dinheiro'
            };
            msg += `💳 *Pagamento:* ${paymentLabels[this.paymentMethod] || this.paymentMethod}\n`;
            
            return msg;
        },

        // Envia o pedido para o WhatsApp
        sendToWhatsApp(shopPhone) {
            if (!this.customerName) {
                this.showToast('⚠️ Por favor, informe seu nome');
                return;
            }
            if (!this.paymentMethod) {
                this.showToast('⚠️ Selecione a forma de pagamento');
                return;
            }
            if (this.deliveryMethod === 'entrega' && !this.address) {
                this.showToast('⚠️ Informe o endereço de entrega');
                return;
            }

            const message = this.formatWhatsAppMessage();
            const encodedMessage = encodeURIComponent(message);
            const url = `https://wa.me/${shopPhone}?text=${encodedMessage}`;
            
            // Abre o WhatsApp
            window.open(url, '_blank');
            
            // Limpa o carrinho após enviar
            this.items = [];
            this.customerName = '';
            this.address = '';
            this.paymentMethod = '';
            this.save();
            this.isOpen = false;
            
            this.showToast('✅ Pedido enviado! Verifique o WhatsApp');
        },

        // Mostra uma notificação toast
        showToast(message) {
            this.toastMessage = message;
            setTimeout(() => {
                this.toastMessage = '';
            }, 3000);
        },

        // Limpa todo o carrinho
        clearCart() {
            this.items = [];
            this.save();
            this.showToast('Carrinho limpo');
        },

        // Modal de produto
        openProductModal(product) {
            this.selectedProduct = product;
            this.modalQty = 1;
            this.productModalOpen = true;
        },

        closeProductModal() {
            this.productModalOpen = false;
            setTimeout(() => {
                this.selectedProduct = null;
            }, 300);
        },

        increaseModalQty() {
            this.modalQty++;
        },

        decreaseModalQty() {
            if (this.modalQty > 1) {
                this.modalQty--;
            }
        },

        addModalProductToCart() {
            if (!this.selectedProduct) return;
            
            const product = this.selectedProduct;
            const existing = this.items.find(i => i.id === product.id);
            if (existing) {
                existing.qty += this.modalQty;
            } else {
                this.items.push({
                    id: product.id,
                    name: product.name,
                    price: product.price,
                    image: product.image || '',
                    qty: this.modalQty
                });
            }
            this.save();
            
            // Animação de pulso no FAB
            this.justAdded = true;
            setTimeout(() => { this.justAdded = false; }, 500);
            
            this.showToast('✅ ' + this.modalQty + 'x ' + product.name + ' adicionado!');
            this.closeProductModal();
        },

        // Filtro local de busca
        productMatches(name, categoryName) {
            if (!this.searchQuery) return true;
            const query = this.searchQuery.toLowerCase().trim();
            const matchesName = name.toLowerCase().includes(query);
            const matchesCategory = categoryName ? categoryName.toLowerCase().includes(query) : false;
            return matchesName || matchesCategory;
        }
    };
}
