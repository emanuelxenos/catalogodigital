// carrinhoGlobal - Alpine.js store para o carrinho de compras
// Persiste no localStorage para manter o estado entre navegações
function carrinhoGlobal(deliveryFee = 0, shopIsOpen = true, deliveryZones = []) {
    return {
        items: JSON.parse(localStorage.getItem('cart_items') || '[]'),
        isOpen: false,
        customerName: localStorage.getItem('cart_customerName') || '',
        customerPhone: localStorage.getItem('cart_customerPhone') || '',
        customerEmail: localStorage.getItem('cart_customerEmail') || '',
        deliveryMethod: localStorage.getItem('cart_deliveryMethod') || 'entrega',
        address: localStorage.getItem('cart_address') || '',
        paymentMethod: localStorage.getItem('cart_paymentMethod') || '',
        justAdded: false,
        toastMessage: '',
        deliveryFee: parseFloat(deliveryFee),
        shopIsOpen: shopIsOpen, // Recebido do servidor via template Go
        deliveryZones: deliveryZones,
        selectedZoneId: localStorage.getItem('cart_selectedZoneId') ? parseInt(localStorage.getItem('cart_selectedZoneId')) : null,
        phoneError: '',
        emailError: '',
        
        // Modal de produto e observações
        searchQuery: '',
        productModalOpen: false,
        selectedProduct: null,
        modalQty: 1,
        modalNote: '',
        activeImageIdx: 0,

        // Sistema de opcionais selecionados
        selectedChoices: {}, // mapeia { "NomeOpcao": {name: "M", price_adjust: 5.0} } ou array se multi-escolha

        // Sistema de cupons e descontos reais
        couponCode: localStorage.getItem('cart_couponCode') || '',
        couponType: localStorage.getItem('cart_couponType') || '', // 'percentage' ou 'fixed'
        couponValue: parseFloat(localStorage.getItem('cart_couponValue') || '0'),
        discountApplied: parseFloat(localStorage.getItem('cart_couponValue') || '0') > 0 ? 1 : 0, // mantido para compatibilidade visual
        couponError: '',
        couponSuccess: '',
        showSuccessModal: false,
        checkoutSuccessOrderId: null,

        // Inicializa o carrinho
        init() {
            if (this.couponCode && this.couponValue > 0) {
                let descText = this.couponType === 'percentage' ? `${this.couponValue}% OFF` : `R$ ${this.couponValue.toFixed(2)} OFF`;
                this.couponSuccess = `Cupom ativo: ${descText}`;
            }
            if (this.deliveryZones && this.deliveryZones.length > 0) {
                this.updateDeliveryZone();
            }
        },

        // Salva o estado no localStorage
        save() {
            localStorage.setItem('cart_items', JSON.stringify(this.items));
            localStorage.setItem('cart_customerName', this.customerName);
            localStorage.setItem('cart_customerPhone', this.customerPhone);
            localStorage.setItem('cart_customerEmail', this.customerEmail);
            localStorage.setItem('cart_deliveryMethod', this.deliveryMethod);
            localStorage.setItem('cart_address', this.address);
            localStorage.setItem('cart_paymentMethod', this.paymentMethod);
            localStorage.setItem('cart_couponCode', this.couponCode);
            localStorage.setItem('cart_couponType', this.couponType || '');
            localStorage.setItem('cart_couponValue', String(this.couponValue || 0));
            localStorage.setItem('cart_selectedZoneId', this.selectedZoneId || '');
        },

        updateDeliveryZone() {
            if (this.deliveryZones && this.deliveryZones.length > 0) {
                const zone = this.deliveryZones.find(z => String(z.id) === String(this.selectedZoneId));
                if (zone) {
                    this.deliveryFee = parseFloat(zone.fee);
                } else {
                    this.deliveryFee = 0;
                }
            }
            this.save();
        },

        // Adiciona um item simples ao carrinho (card de compra rápida)
        addItem(product) {
            const existing = this.items.find(i => i.id === product.id && i.note === '' && !i.options_json);
            if (existing) {
                if (product.stockQty !== null && product.stockQty !== undefined) {
                    if (existing.qty >= product.stockQty) {
                        this.showToast(`⚠️ Apenas ${product.stockQty} unidades em estoque`);
                        return;
                    }
                }
                existing.qty++;
            } else {
                this.items.push({
                    id: product.id,
                    name: product.name,
                    price: product.price,
                    image: product.image || '',
                    qty: 1,
                    note: '',
                    options: {},
                    options_json: '',
                    options_text: '',
                    stockQty: product.stockQty
                });
            }
            this.save();
            
            this.justAdded = true;
            setTimeout(() => { this.justAdded = false; }, 500);
            
            this.showToast('✅ ' + product.name + ' adicionado!');
        },

        // Remove um item do carrinho
        removeItem(id) {
            this.items = this.items.filter(i => i.id !== id);
            this.save();
        },

        // Atualiza a quantidade de um item no drawer
        updateQty(id, delta) {
            const item = this.items.find(i => i.id === id);
            if (!item) return;
            
            if (delta > 0 && item.stockQty !== null && item.stockQty !== undefined) {
                if (item.qty >= item.stockQty) {
                    this.showToast(`⚠️ Apenas ${item.stockQty} unidades em estoque`);
                    return;
                }
            }
            
            item.qty += delta;
            if (item.qty <= 0) {
                this.removeItem(id);
            } else {
                this.save();
            }
        },

        // Métodos de controle de quantidade reativa direta no card (sem abrir modal)
        getItemQty(productId) {
            return this.items
                .filter(i => i.id === productId)
                .reduce((sum, item) => sum + item.qty, 0);
        },

        updateCardQty(product, delta) {
            const existing = this.items.find(i => i.id === product.id && i.note === '' && !i.options_json);
            if (existing) {
                if (delta > 0 && product.stockQty !== null && product.stockQty !== undefined) {
                    if (existing.qty >= product.stockQty) {
                        this.showToast(`⚠️ Apenas ${product.stockQty} unidades em estoque`);
                        return;
                    }
                }
                existing.qty += delta;
                if (existing.qty <= 0) {
                    this.items = this.items.filter(i => !(i.id === product.id && i.note === '' && !i.options_json));
                }
            } else if (delta > 0) {
                this.items.push({
                    id: product.id,
                    name: product.name,
                    price: product.price,
                    image: product.image || '',
                    qty: 1,
                    note: '',
                    options: {},
                    options_json: '',
                    options_text: '',
                    stockQty: product.stockQty
                });
            }
            this.save();
            
            this.justAdded = true;
            setTimeout(() => { this.justAdded = false; }, 500);
        },

        // Calcula o total bruto
        getTotal() {
            return this.items.reduce((sum, item) => sum + (item.price * item.qty), 0);
        },

        // Sistema de Cupons Reais com Validação no Banco
        applyCoupon() {
            const code = this.couponCode.toUpperCase().trim();
            this.couponError = '';
            this.couponSuccess = '';

            if (code === '') {
                this.couponError = 'Digite um cupom';
                return;
            }

            const slug = window.location.pathname.split('/').filter(Boolean).pop() || '';
            if (!slug) {
                this.couponError = 'Erro ao identificar a loja';
                return;
            }

            fetch(`/${slug}/coupon/${code}`)
                .then(res => res.json())
                .then(data => {
                    if (data.valid) {
                        this.couponType = data.type;
                        this.couponValue = parseFloat(data.value);
                        this.discountApplied = 1;

                        let descText = data.type === 'percentage' ? `${data.value}% OFF` : `R$ ${data.value.toFixed(2)} OFF`;
                        this.couponSuccess = `Cupom de desconto aplicado: ${descText}`;
                        this.showToast(`🎟️ Cupom ${data.code} aplicado!`);
                        this.save();
                    } else {
                        this.couponError = data.message || 'Cupom inválido!';
                        this.couponType = '';
                        this.couponValue = 0;
                        this.discountApplied = 0;
                        this.save();
                    }
                })
                .catch(err => {
                    this.couponError = 'Erro ao validar cupom';
                    console.error(err);
                });
        },

        removeCoupon() {
            this.couponCode = '';
            this.couponType = '';
            this.couponValue = 0;
            this.discountApplied = 0;
            this.couponError = '';
            this.couponSuccess = '';
            this.save();
            this.showToast('Cupom removido');
        },

        getDiscount() {
            if (this.couponType === 'percentage') {
                return this.getTotal() * (this.couponValue / 100);
            } else if (this.couponType === 'fixed') {
                return this.couponValue;
            }
            return 0;
        },

        getFinalTotal() {
            let total = this.getTotal() - this.getDiscount();
            if (this.deliveryMethod === 'entrega') {
                total += this.deliveryFee;
            }
            return Math.max(0, total);
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

        // -------------------------------------------------------
        // Helpers de Validação
        // -------------------------------------------------------

        // Extrai somente dígitos de uma string
        onlyDigits(str) {
            return str.replace(/\D/g, '');
        },

        // Aplica máscara ao telefone enquanto o usuário digita
        // Formato: (XX) XXXXX-XXXX ou (XX) XXXX-XXXX
        applyPhoneMask(value) {
            let digits = this.onlyDigits(value).substring(0, 11);
            if (digits.length === 0) return '';
            let result = '(' + digits.substring(0, 2);
            if (digits.length > 2) {
                result += ') ' + digits.substring(2, digits.length <= 10 ? 6 : 7);
            }
            if (digits.length > (digits.length <= 10 ? 6 : 7)) {
                result += '-' + digits.substring(digits.length <= 10 ? 6 : 7);
            }
            return result;
        },

        // Valida se o telefone tem o mínimo de dígitos para ser um número brasileiro válido
        // Exige: DDD (2) + número (8 ou 9 dígitos) = 10 ou 11 dígitos
        isValidPhone(value) {
            const digits = this.onlyDigits(value);
            return digits.length >= 10 && digits.length <= 11;
        },

        // Valida formato de e-mail simples
        isValidEmail(value) {
            if (!value || value.trim() === '') return true; // opcional
            return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value.trim());
        },

        // Handler para o campo de telefone: aplica máscara e limpa erro
        onPhoneInput(event) {
            this.customerPhone = this.applyPhoneMask(event.target.value);
            if (this.isValidPhone(this.customerPhone)) {
                this.phoneError = '';
            }
        },

        // Handler para o campo de email: limpa erro ao corrigir
        onEmailInput() {
            if (this.isValidEmail(this.customerEmail)) {
                this.emailError = '';
            }
        },

        // -------------------------------------------------------
        // Envia o pedido para a Rota de Checkout Segura do Backend
        // -------------------------------------------------------
        finalizeOrder() {
            // Reseta erros anteriores
            this.phoneError = '';
            this.emailError = '';

            if (!this.customerName || !this.customerName.trim()) {
                this.showToast('⚠️ Por favor, informe seu nome');
                return;
            }
            if (!this.customerPhone || !this.isValidPhone(this.customerPhone)) {
                this.phoneError = 'Número inválido. Informe DDD + número (ex: 87 99999-9999)';
                this.showToast('⚠️ WhatsApp inválido');
                return;
            }
            if (this.customerEmail && !this.isValidEmail(this.customerEmail)) {
                this.emailError = 'E-mail inválido. Verifique o formato (ex: nome@email.com)';
                this.showToast('⚠️ E-mail com formato inválido');
                return;
            }
            if (!this.paymentMethod) {
                this.showToast('⚠️ Selecione a forma de pagamento');
                return;
            }
            if (this.deliveryMethod === 'entrega') {
                if (this.deliveryZones && this.deliveryZones.length > 0 && !this.selectedZoneId) {
                    this.showToast('⚠️ Por favor, selecione seu bairro para entrega');
                    return;
                }
                if (!this.address) {
                    this.showToast('⚠️ Informe o endereço de entrega');
                    return;
                }
            }

            const slug = window.location.pathname.split('/').filter(Boolean).pop() || '';
            if (!slug) {
                this.showToast('⚠️ Erro ao identificar a loja');
                return;
            }

            // Prepara payload formatado para a rota Go
            const payload = {
                customerName: this.customerName,
                customerPhone: this.customerPhone,
                customerEmail: this.customerEmail,
                deliveryMethod: this.deliveryMethod,
                address: this.address,
                paymentMethod: this.paymentMethod,
                couponCode: this.couponCode,
                deliveryZoneId: this.deliveryZones && this.deliveryZones.length > 0 ? parseInt(this.selectedZoneId) : null,
                items: this.items.map(item => ({
                    id: item.id,
                    name: item.name,
                    price: item.price,
                    qty: item.qty,
                    note: item.note,
                    options: item.options_json || ''
                }))
            };

            fetch(`/${slug}/checkout`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify(payload)
            })
            .then(res => {
                if (!res.ok) {
                    return res.json().then(err => { throw new Error(err.message || 'Erro ao processar checkout'); });
                }
                return res.json();
            })
            .then(data => {
                if (data.success) {
                    // Salva os dados do pedido no estado local
                    this.checkoutSuccessOrderId = data.order_id;
                    this.showSuccessModal = true;

                    // Limpa o carrinho local
                    this.items = [];
                    this.customerName = '';
                    this.customerPhone = '';
                    this.customerEmail = '';
                    this.address = '';
                    this.paymentMethod = '';
                    this.couponCode = '';
                    this.couponType = '';
                    this.couponValue = 0;
                    this.discountApplied = 0;
                    this.couponSuccess = '';
                    this.couponError = '';
                    this.save();
                    this.isOpen = false;
                } else {
                    this.showToast('⚠️ Erro ao processar pedido');
                }
            })
            .catch(err => {
                this.showToast('❌ ' + err.message);
                console.error(err);
            });
        },

        closeSuccessModal() {
            this.showSuccessModal = false;
            this.checkoutSuccessOrderId = null;
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
            this.couponCode = '';
            this.couponType = '';
            this.couponValue = 0;
            this.discountApplied = 0;
            this.couponSuccess = '';
            this.couponError = '';
            this.save();
            this.showToast('Carrinho limpo');
        },

        // Modal de produto e opcionais
        openProductModal(product, optionsAttr, imagesAttr) {
            let parsedOptions = [];
            if (optionsAttr) {
                try {
                    parsedOptions = JSON.parse(optionsAttr);
                } catch(e) {
                    console.error("Erro ao processar opcionais do produto:", e);
                }
            }
            let parsedImages = [];
            if (imagesAttr) {
                try {
                    parsedImages = JSON.parse(imagesAttr);
                } catch(e) {
                    console.error("Erro ao processar imagens do produto:", e);
                }
            }
            if (parsedImages.length === 0 && product.image) {
                parsedImages = [product.image];
            }
            this.selectedProduct = { ...product, options: parsedOptions, images: parsedImages };
            this.activeImageIdx = 0;
            this.modalQty = 1;
            this.modalNote = '';
            this.selectedChoices = {};
            this.productModalOpen = true;
        },

        nextImage() {
            if (this.selectedProduct && this.selectedProduct.images) {
                this.activeImageIdx = (this.activeImageIdx + 1) % this.selectedProduct.images.length;
            }
        },

        prevImage() {
            if (this.selectedProduct && this.selectedProduct.images) {
                this.activeImageIdx = (this.activeImageIdx - 1 + this.selectedProduct.images.length) % this.selectedProduct.images.length;
            }
        },

        closeProductModal() {
            this.productModalOpen = false;
            setTimeout(() => {
                this.selectedProduct = null;
                this.modalNote = '';
                this.selectedChoices = {};
            }, 300);
        },

        increaseModalQty() {
            if (this.selectedProduct && this.selectedProduct.stockQty !== null && this.selectedProduct.stockQty !== undefined) {
                if (this.modalQty >= this.selectedProduct.stockQty) {
                    this.showToast(`⚠️ Apenas ${this.selectedProduct.stockQty} unidades em estoque`);
                    return;
                }
            }
            this.modalQty++;
        },

        decreaseModalQty() {
            if (this.modalQty > 1) {
                this.modalQty--;
            }
        },

        toggleChoice(opt, choice) {
            if (opt.multi) {
                if (!this.selectedChoices[opt.name]) {
                    this.selectedChoices[opt.name] = [];
                }
                const choicesList = this.selectedChoices[opt.name];
                const idx = choicesList.findIndex(c => c.name === choice.name);
                if (idx > -1) {
                    choicesList.splice(idx, 1);
                } else {
                    choicesList.push({ name: choice.name, price_adjust: parseFloat(choice.price_adjust || 0) });
                }
            } else {
                if (this.selectedChoices[opt.name] && this.selectedChoices[opt.name].name === choice.name) {
                    if (!opt.required) {
                        delete this.selectedChoices[opt.name];
                    }
                } else {
                    this.selectedChoices[opt.name] = { name: choice.name, price_adjust: parseFloat(choice.price_adjust || 0) };
                }
            }
        },

        isChoiceSelected(optName, choiceName) {
            const val = this.selectedChoices[optName];
            if (!val) return false;
            if (Array.isArray(val)) {
                return val.some(c => c.name === choiceName);
            }
            return val.name === choiceName;
        },

        getModalItemPrice() {
            if (!this.selectedProduct) return 0;
            let price = this.selectedProduct.price;
            Object.keys(this.selectedChoices).forEach(key => {
                const val = this.selectedChoices[key];
                if (Array.isArray(val)) {
                    val.forEach(c => {
                        price += (c.price_adjust || 0);
                    });
                } else if (val) {
                    price += (val.price_adjust || 0);
                }
            });
            return price;
        },

        getOptionsText() {
            let parts = [];
            Object.keys(this.selectedChoices).forEach(key => {
                const val = this.selectedChoices[key];
                if (Array.isArray(val)) {
                    const names = val.map(c => c.name).join(', ');
                    if (names) parts.push(`${key}: ${names}`);
                } else if (val) {
                    parts.push(`${key}: ${val.name}`);
                }
            });
            return parts.join(' | ');
        },

        addModalProductToCart() {
            if (!this.selectedProduct) return;

            // Validação de opcionais obrigatórios
            let missingRequired = [];
            (this.selectedProduct.options || []).forEach(opt => {
                if (opt.required && !this.selectedChoices[opt.name]) {
                    missingRequired.push(opt.name);
                }
            });
            if (missingRequired.length > 0) {
                this.showToast('⚠️ Selecione: ' + missingRequired.join(', '));
                return;
            }
            
            const product = this.selectedProduct;
            const itemPrice = this.getModalItemPrice();
            const optionsText = this.getOptionsText();
            const optionsJson = JSON.stringify(this.selectedChoices);

            // Tenta encontrar item idêntico com as mesmas escolhas/obs no carrinho
            const existing = this.items.find(i => 
                i.id === product.id && 
                i.note === this.modalNote.trim() && 
                i.options_json === optionsJson
            );

            if (existing) {
                if (product.stockQty !== null && product.stockQty !== undefined) {
                    if (existing.qty + this.modalQty > product.stockQty) {
                        this.showToast(`⚠️ Apenas ${product.stockQty} unidades em estoque`);
                        return;
                    }
                }
                existing.qty += this.modalQty;
            } else {
                this.items.push({
                    id: product.id,
                    name: product.name,
                    price: itemPrice,
                    image: product.image || '',
                    qty: this.modalQty,
                    note: this.modalNote.trim(),
                    options: { ...this.selectedChoices },
                    options_json: optionsJson,
                    options_text: optionsText,
                    stockQty: product.stockQty
                });
            }
            this.save();
            
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

