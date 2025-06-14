package main

import (
	"time"

	"github.com/shopspring/decimal" 
)

type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"` 
	CreatedAt    time.Time `json:"created_at"`
}

type Account struct {
	ID        string          `json:"id"`
	UserID    string          `json:"user_id"`
	Number    string          `json:"number"` 
	Balance   decimal.Decimal `json:"balance"`
	CreatedAt time.Time       `json:"created_at"`
}

type Card struct {
    ID          string    `json:"id"`
    AccountID   string    `json:"account_id"`
    Number      string    `json:"number"`      // Зашифрованный номер карты (PGP)
    ExpiryMonth int       `json:"expiry_month"`
    ExpiryYear  int       `json:"expiry_year"`
    CVV         string    `json:"cvv"`         // Хэшированный CVV (bcrypt)
    CreatedAt   time.Time `json:"created_at"`
    HMAC        string    `json:"hmac"`        // HMAC для проверки целостности
}

type Transaction struct {
	ID              string          `json:"id"`
	FromAccountID   string          `json:"from_account_id,omitempty"` 
	ToAccountID     string          `json:"to_account_id,omitempty"`   
	Amount          decimal.Decimal `json:"amount"`
	Timestamp       time.Time       `json:"timestamp"`
	TransactionType string          `json:"transaction_type"`
	Description     string          `json:"description,omitempty"`
}

type Loan struct {
	ID              string          `json:"id"`
	UserID          string          `json:"user_id"`
	AccountID       string          `json:"account_id"` 
	Amount          decimal.Decimal `json:"amount"`
	InterestRate    decimal.Decimal `json:"interest_rate"`
	TermMonths      int             `json:"term_months"`
	StartDate       time.Time       `json:"start_date"`
	PaymentSchedule []Payment       `json:"payment_schedule"`
	RemainingAmount decimal.Decimal `json:"remaining_amount"`
}

type Payment struct {
	DueDate       time.Time       `json:"due_date"`
	Amount        decimal.Decimal `json:"amount"`
	PrincipalPart decimal.Decimal `json:"principal_part"`
	InterestPart  decimal.Decimal `json:"interest_part"`
	Paid          bool            `json:"paid"`
}


type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type CreateAccountRequest struct {
	UserID string `json:"user_id"` 
}

type GenerateCardRequest struct {
	AccountID string `json:"account_id"`
}

type PaymentRequest struct {
    CardNumber string          `json:"card_number"`
    CVV        string          `json:"cvv"`
    Amount     decimal.Decimal `json:"amount"`
    Merchant   string          `json:"merchant"`
}

type TransferRequest struct {
	FromAccountID string          `json:"from_account_id"`
	ToAccountID   string          `json:"to_account_id"`
	Amount        decimal.Decimal `json:"amount"`
}

type DepositRequest struct {
	ToAccountID string          `json:"to_account_id"`
	Amount      decimal.Decimal `json:"amount"`
}

type ApplyLoanRequest struct {
	UserID     string          `json:"user_id"` 
	AccountID  string          `json:"account_id"`
	Amount     decimal.Decimal `json:"amount"`
	TermMonths int             `json:"term_months"`
}

type ScheduledPayment struct {
    ID        int64     `db:"id"`
    AccountID int64     `db:"account_id"`
    Amount    float64   `db:"amount"`
    DueDate   time.Time `db:"due_date"`
    Paid      bool      `db:"paid"`
}


