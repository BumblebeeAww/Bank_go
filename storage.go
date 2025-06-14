package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"sync"

	"github.com/shopspring/decimal"
)

type InMemoryStorage struct {
	users        map[string]User     // key: UserID
	accounts     map[string]Account  // key: AccountID
	cards        map[string]Card     // key: CardID
	loans        map[string]Loan     // key: LoanID
	transactions []Transaction       // Просто список всех транзакций
	userIndex    map[string]string   // key: Username -> UserID (для быстрой проверки уникальности)
	emailIndex   map[string]string   // key: Email -> UserID
	accountIndex map[string][]string // key: UserID -> []AccountID
	cardIndex    map[string][]string // key: AccountID -> []CardID
	loanIndex    map[string][]string // key: UserID -> []LoanID
	mu           sync.RWMutex        // Mutex для защиты доступа к данным
}

var storage *InMemoryStorage

func InitStorage() {
	storage = &InMemoryStorage{
		users:        make(map[string]User),
		accounts:     make(map[string]Account),
		cards:        make(map[string]Card),
		loans:        make(map[string]Loan),
		transactions: make([]Transaction, 0),
		userIndex:    make(map[string]string),
		emailIndex:   make(map[string]string),
		accountIndex: make(map[string][]string),
		cardIndex:    make(map[string][]string),
		loanIndex:    make(map[string][]string),
	}
}

func AddUser(user User) error {
	storage.mu.Lock()
	defer storage.mu.Unlock()

	if _, exists := storage.userIndex[user.Username]; exists {
		return fmt.Errorf("username '%s' already taken", user.Username)
	}
	if _, exists := storage.emailIndex[user.Email]; exists {
		return fmt.Errorf("email '%s' already registered", user.Email)
	}

	storage.users[user.ID] = user
	storage.userIndex[user.Username] = user.ID
	storage.emailIndex[user.Email] = user.ID
	return nil
}

func GetUserByUsername(username string) (User, bool) {
	storage.mu.RLock()
	defer storage.mu.RUnlock()
	userID, ok := storage.userIndex[username]
	if !ok {
		return User{}, false
	}
	user, ok := storage.users[userID]
	return user, ok
}

func AddAccount(account Account) error {
	storage.mu.Lock()
	defer storage.mu.Unlock()
	if _, exists := storage.users[account.UserID]; !exists {
		return fmt.Errorf("user with ID %s not found", account.UserID)
	}
	storage.accounts[account.ID] = account
	storage.accountIndex[account.UserID] = append(storage.accountIndex[account.UserID], account.ID)
	return nil
}

func GetAccount(accountID string) (Account, bool) {
	storage.mu.RLock()
	defer storage.mu.RUnlock()
	acc, ok := storage.accounts[accountID]
	return acc, ok
}

func GetUserAccounts(userID string) []Account {
	storage.mu.RLock()
	defer storage.mu.RUnlock()
	accountIDs := storage.accountIndex[userID]
	accounts := make([]Account, 0, len(accountIDs))
	for _, id := range accountIDs {
		if acc, ok := storage.accounts[id]; ok {
			accounts = append(accounts, acc)
		}
	}
	return accounts
}

func UpdateAccountBalance(accountID string, amount decimal.Decimal) error {
	storage.mu.Lock()
	defer storage.mu.Unlock()

	acc, ok := storage.accounts[accountID]
	if !ok {
		return fmt.Errorf("account %s not found", accountID)
	}

	newBalance := acc.Balance.Add(amount)
	if newBalance.IsNegative() {
	}

	acc.Balance = newBalance
	storage.accounts[accountID] = acc
	return nil
}

func AddTransaction(tx Transaction) {
	storage.mu.Lock()
	defer storage.mu.Unlock()
	storage.transactions = append(storage.transactions, tx)
}

func GetAccountTransactions(accountID string) []Transaction {
	storage.mu.RLock()
	defer storage.mu.RUnlock()
	var accountTxs []Transaction
	for _, tx := range storage.transactions {
		if tx.FromAccountID == accountID || tx.ToAccountID == accountID {
			accountTxs = append(accountTxs, tx)
		}
	}
	return accountTxs
}


// Секретный ключ для HMAC — лучше брать из конфигурации/окружения
var secretHMACKey = []byte("your-secret-hmac-key")

// Вспомогательная функция для генерации HMAC карты
func computeCardHMAC(card Card) string {
    // Конкатенируем важные поля карты
    data := card.Number + card.CVV + fmt.Sprintf("%02d%04d", card.ExpiryMonth, card.ExpiryYear)
    return GenerateHMAC(data, secretHMACKey)
}

func GenerateHMAC(data string, key []byte) string {
    mac := hmac.New(sha256.New, key)
    mac.Write([]byte(data))
    return fmt.Sprintf("%x", mac.Sum(nil))
}

func AddCard(card Card) error {
    storage.mu.Lock()
    defer storage.mu.Unlock()

    if _, exists := storage.accounts[card.AccountID]; !exists {
        return fmt.Errorf("account %s not found", card.AccountID)
    }

    // --- Вот здесь добавляем генерацию HMAC ---
    card.HMAC = computeCardHMAC(card)

    // Сохраняем карту с HMAC
    storage.cards[card.ID] = card
    storage.cardIndex[card.AccountID] = append(storage.cardIndex[card.AccountID], card.ID)

    return nil
}

func GetAccountCards(accountID string) []Card {
	storage.mu.RLock()
	defer storage.mu.RUnlock()
	cardIDs := storage.cardIndex[accountID]
	cards := make([]Card, 0, len(cardIDs))
	for _, id := range cardIDs {
		if card, ok := storage.cards[id]; ok {
			cards = append(cards, card)
		}
	}
	return cards
}

func GetCardByNumber(number string) (Card, bool) {
	storage.mu.RLock()
	defer storage.mu.RUnlock()
	for _, card := range storage.cards {
		if card.Number == number {
			return card, true
		}
	}
	return Card{}, false
}

func AddLoan(loan Loan) error {
	storage.mu.Lock()
	defer storage.mu.Unlock()
	if _, exists := storage.users[loan.UserID]; !exists {
		return fmt.Errorf("user %s not found", loan.UserID)
	}
	if _, exists := storage.accounts[loan.AccountID]; !exists {
		return fmt.Errorf("account %s not found", loan.AccountID)
	}
	storage.loans[loan.ID] = loan
	storage.loanIndex[loan.UserID] = append(storage.loanIndex[loan.UserID], loan.ID)
	return nil
}

func GetUserLoans(userID string) []Loan {
	storage.mu.RLock()
	defer storage.mu.RUnlock()
	loanIDs := storage.loanIndex[userID]
	loans := make([]Loan, 0, len(loanIDs))
	for _, id := range loanIDs {
		if loan, ok := storage.loans[id]; ok {
			loans = append(loans, loan)
		}
	}
	return loans
}

func GetLoan(loanID string) (Loan, bool) {
	storage.mu.RLock()
	defer storage.mu.RUnlock()
	loan, ok := storage.loans[loanID]
	return loan, ok
}
