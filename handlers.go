package main

import (
	"crypto/hmac"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/shopspring/decimal"
	"golang.org/x/crypto/bcrypt"
)

func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
    response, err := json.Marshal(payload)
    if err != nil {
        log.Printf("Error marshalling JSON: %v", err)
        w.WriteHeader(http.StatusInternalServerError)
        w.Write([]byte(`{"error": "Internal server error"}`))
        return
    }
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    w.Write(response)
}

func respondError(w http.ResponseWriter, code int, message string) {
    log.Printf("HTTP Error %d: %s", code, message)
    respondJSON(w, code, map[string]string{"error": message})
}

func RegisterUserHandler(w http.ResponseWriter, r *http.Request) {
    defer r.Body.Close()
    var req RegisterRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondError(w, http.StatusBadRequest, "Invalid request payload")
        return
    }

    if req.Username == "" || req.Email == "" || req.Password == "" {
        respondError(w, http.StatusBadRequest, "Username, email, and password are required")
        return
    }

    hashedPassword, err := HashPassword(req.Password)
    if err != nil {
        respondError(w, http.StatusInternalServerError, "Failed to hash password")
        return
    }

    user := User{
        ID:           GenerateID(),
        Username:     req.Username,
        Email:        req.Email,
        PasswordHash: hashedPassword,
        CreatedAt:    time.Now(),
    }

    if err := AddUser(user); err != nil {
        respondError(w, http.StatusConflict, err.Error())
        return
    }

    go func() {
        subject := "Welcome to Simple Bank!"
        body := fmt.Sprintf("Hello %s,\n\nThank you for registering at Simple Bank.", user.Username)
        err := SendEmailNotification(user.Email, subject, body)
        if err != nil {
            log.Printf("Failed to send registration email to %s: %v", user.Email, err)
        }
    }()

    log.Printf("User registered: %s (ID: %s)", user.Username, user.ID)
    user.PasswordHash = ""
    respondJSON(w, http.StatusCreated, user)
}

func LoginUserHandler(w http.ResponseWriter, r *http.Request) {
    defer r.Body.Close()
    var req LoginRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondError(w, http.StatusBadRequest, "Invalid request payload")
        return
    }

    user, ok := GetUserByUsername(req.Username)
    if !ok {
        respondError(w, http.StatusUnauthorized, "Invalid username or password")
        return
    }

    if !CheckPasswordHash(req.Password, user.PasswordHash) {
        respondError(w, http.StatusUnauthorized, "Invalid username or password")
        return
    }

    log.Printf("User  logged in: %s", user.Username)
    token, err := GenerateJWT(user.ID) 
    if err != nil {
        respondError(w, http.StatusInternalServerError, "Failed to generate token")
        return
    }

    respondJSON(w, http.StatusOK, map[string]string{
        "message": "Login successful",
        "token":   token,
        "user_id": user.ID,
    })
}

func CreateAccountHandler(w http.ResponseWriter, r *http.Request) {
    defer r.Body.Close()
    var req CreateAccountRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondError(w, http.StatusBadRequest, "Invalid request payload")
        return
    }

    userID, ok := r.Context().Value("user").(string) 
    if !ok {
        respondError(w, http.StatusUnauthorized, "User  not found in context")
        return
    }

    account := Account{
        ID:        GenerateID(),
        UserID:    userID, 
        Number:    GenerateAccountNumber(),
        Balance:   decimal.Zero,
        CreatedAt: time.Now(),
    }

    if err := AddAccount(account); err != nil {
        respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create account: %v", err))
        return
    }

    log.Printf("Account created: %s for user %s", account.Number, account.UserID)
    respondJSON(w, http.StatusCreated, account)
}

func GetUserAccountsHandler(w http.ResponseWriter, r *http.Request) {
    userID, ok := r.Context().Value("user").(string) 
    if !ok {
        respondError(w, http.StatusUnauthorized, "User  not found in context")
        return
    }

    accounts := GetUserAccounts(userID)
    log.Printf("Fetched %d accounts for user %s", len(accounts), userID)
    respondJSON(w, http.StatusOK, accounts)
}

func GenerateCardHandler(w http.ResponseWriter, r *http.Request) {
    defer r.Body.Close()
    var req GenerateCardRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondError(w, http.StatusBadRequest, "Invalid request payload")
        return
    }

    userID, ok := r.Context().Value(userContextKey).(string)
    if !ok {
        respondError(w, http.StatusUnauthorized, "User not found in context")
        return
    }

    account, ok := GetAccount(req.AccountID)
    if !ok || account.UserID != userID {
        respondError(w, http.StatusBadRequest, "Account not found or access denied")
        return
    }

    month, year := GenerateExpiryDate()
    cvv := GenerateCVV()

    card := Card{
        ID:          GenerateID(),
        AccountID:   req.AccountID,
        ExpiryMonth: month,
        ExpiryYear:  year,
        CreatedAt:   time.Now(),
    }

    // Загрузка публичного ключа PGP 
    pubKeyPath := os.Getenv("PGP_PUBLIC_KEY_PATH")
    if pubKeyPath == "" {
        respondError(w, http.StatusInternalServerError, "PGP_PUBLIC_KEY_PATH not set")
        return
    }
    pubKey, err := LoadPublicKey(pubKeyPath)
    if err != nil {
        respondError(w, http.StatusInternalServerError, "Failed to load PGP public key")
        return
    }

    encryptedNumber, err := EncryptWithPGP(GenerateValidCardNumber(), pubKey)
    if err != nil {
        respondError(w, http.StatusInternalServerError, "Error encrypting card number")
        return
    }

    card.Number = encryptedNumber

    // Хешируем CVV через bcrypt
    cvvHash, err := bcrypt.GenerateFromPassword([]byte(cvv), bcrypt.DefaultCost)
    if err != nil {
        respondError(w, http.StatusInternalServerError, "Failed to hash CVV")
        return
    }
    card.CVV = string(cvvHash)

    if err := AddCard(card); err != nil {
        respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to generate card: %v", err))
        return
    }

    log.Printf("Card generated for account %s", card.AccountID)
    
    respondJSON(w, http.StatusCreated, map[string]interface{}{
        "card_id":     card.ID,
        "card_number": DecryptPGPForResponse(card.Number), // функция ниже
        "expiry_month": card.ExpiryMonth,
        "expiry_year":  card.ExpiryYear,
        "cvv":         cvv,
        "created_at":  card.CreatedAt,
    })
}

// Вспомогательная функция для дешифровки номера карты в ответе 
func DecryptPGPForResponse(encrypted string) string {
    privKeyPath := os.Getenv("PGP_PRIVATE_KEY_PATH")
    if privKeyPath == "" {
        return "**** **** **** ****"
    }
    privKey, err := LoadPrivateKey(privKeyPath)
    if err != nil {
        return "**** **** **** ****"
    }
    decrypted, err := DecryptWithPGP(encrypted, privKey)
    if err != nil {
        return "**** **** **** ****"
    }
    return decrypted
}

func GetAccountCardsHandler(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    accountID := vars["accountId"]

    userID, ok := r.Context().Value(userContextKey).(string)
    if !ok {
        respondError(w, http.StatusUnauthorized, "User not found in context")
        return
    }

    account, ok := GetAccount(accountID)
    if !ok || account.UserID != userID {
        respondError(w, http.StatusNotFound, "Account not found or access denied")
        return
    }

    cards := GetAccountCards(accountID)

    privKeyPath := os.Getenv("PGP_PRIVATE_KEY_PATH")
    if privKeyPath == "" {
        respondError(w, http.StatusInternalServerError, "PGP_PRIVATE_KEY_PATH not set")
        return
    }
    privKey, err := LoadPrivateKey(privKeyPath)
    if err != nil {
        respondError(w, http.StatusInternalServerError, "Failed to load PGP private key")
        return
    }

    type cardResponse struct {
        ID          string `json:"id"`
        NumberMasked string `json:"number_masked"`
        ExpiryMonth int    `json:"expiry_month"`
        ExpiryYear  int    `json:"expiry_year"`
        CreatedAt   time.Time `json:"created_at"`
    }

    respCards := make([]cardResponse, 0, len(cards))
    for _, c := range cards {
        decryptedNumber, err := DecryptWithPGP(c.Number, privKey)
        if err != nil {
            respondError(w, http.StatusInternalServerError, "Error decrypting card number")
            return
        }
        masked := maskCardNumber(decryptedNumber)
        respCards = append(respCards, cardResponse{
            ID:           c.ID,
            NumberMasked: masked,
            ExpiryMonth:  c.ExpiryMonth,
            ExpiryYear:   c.ExpiryYear,
            CreatedAt:    c.CreatedAt,
        })
    }

    respondJSON(w, http.StatusOK, respCards)
}

func maskCardNumber(number string) string {
    if len(number) < 4 {
        return "****"
    }
    last4 := number[len(number)-4:]
    return "**** **** **** " + last4
}

func PayWithCardHandler(w http.ResponseWriter, r *http.Request) {
    defer r.Body.Close()
    var req PaymentRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondError(w, http.StatusBadRequest, "Invalid request payload")
        return
    }

    if req.Amount.LessThanOrEqual(decimal.Zero) {
        respondError(w, http.StatusBadRequest, "Payment amount must be positive")
        return
    }
    
    card, ok := GetCardByID(req.CardNumber) 
    if !ok {
        respondError(w, http.StatusNotFound, "Card not found")
        return
    }

    // Проверяем CVV
    if err := bcrypt.CompareHashAndPassword([]byte(card.CVV), []byte(req.CVV)); err != nil {
    respondError(w, http.StatusUnauthorized, "Invalid CVV")
    return
    }

    now := time.Now()
    expiry := time.Date(card.ExpiryYear, time.Month(card.ExpiryMonth), 1, 23, 59, 59, 0, time.UTC).AddDate(0, 1, -1)
    if now.After(expiry) {
        respondError(w, http.StatusBadRequest, "Card expired")
        return
    }

    account, ok := GetAccount(card.AccountID)
    if !ok {
        respondError(w, http.StatusInternalServerError, "Associated account not found")
        return
    }

    if account.Balance.LessThan(req.Amount) {
        respondError(w, http.StatusPaymentRequired, "Insufficient funds")
        return
    }

    err := UpdateAccountBalance(account.ID, req.Amount.Neg())
    if err != nil {
        respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to process payment: %v", err))
        return
    }

    tx := Transaction{
        ID:              GenerateID(),
        FromAccountID:   account.ID,
        ToAccountID:     "",
        Amount:          req.Amount,
        Timestamp:       time.Now(),
        TransactionType: "payment",
        Description:     fmt.Sprintf("Payment to %s", req.Merchant),
    }
    AddTransaction(tx)

    log.Printf("Payment of %s processed from account %s", req.Amount.String(), account.ID)
    respondJSON(w, http.StatusOK, map[string]string{"message": "Payment successful"})
}

func GetCardByID(cardID string) (Card, bool) {
    storage.mu.RLock()
    defer storage.mu.RUnlock()
    card, ok := storage.cards[cardID]
    if !ok {
        return Card{}, false
    }
    // Проверяем HMAC
    expectedHMAC := computeCardHMAC(card)
    if !hmac.Equal([]byte(expectedHMAC), []byte(card.HMAC)) {
        return Card{}, false 
    }
    return card, true
}

func TransferHandler(w http.ResponseWriter, r *http.Request) {
    defer r.Body.Close()
    var req TransferRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondError(w, http.StatusBadRequest, "Invalid request payload")
        return
    }

    if req.FromAccountID == req.ToAccountID {
        respondError(w, http.StatusBadRequest, "Cannot transfer to the same account")
        return
    }
    if req.Amount.LessThanOrEqual(decimal.Zero) {
        respondError(w, http.StatusBadRequest, "Transfer amount must be positive")
        return
    }

    storage.mu.Lock()
    defer storage.mu.Unlock()

    fromAccount, okFrom := storage.accounts[req.FromAccountID]
    toAccount, okTo := storage.accounts[req.ToAccountID]

    if !okFrom {
        respondError(w, http.StatusNotFound, fmt.Sprintf("Source account %s not found", req.FromAccountID))
        return
    }
    if !okTo {
        respondError(w, http.StatusNotFound, fmt.Sprintf("Destination account %s not found", req.ToAccountID))
        return
    }

    if fromAccount.Balance.LessThan(req.Amount) {
        respondError(w, http.StatusPaymentRequired, "Insufficient funds in source account")
        return
    }

    fromAccount.Balance = fromAccount.Balance.Sub(req.Amount)
    toAccount.Balance = toAccount.Balance.Add(req.Amount)

    storage.accounts[req.FromAccountID] = fromAccount
    storage.accounts[req.ToAccountID] = toAccount

    tx := Transaction{
        ID:              GenerateID(),
        FromAccountID:   req.FromAccountID,
        ToAccountID:     req.ToAccountID,
        Amount:          req.Amount,
        Timestamp:       time.Now(),
        TransactionType: "transfer",
        Description:     fmt.Sprintf("Transfer from %s to %s", fromAccount.Number, toAccount.Number),
    }
    storage.transactions = append(storage.transactions, tx)

    log.Printf("Transfer of %s from %s to %s successful", req.Amount.String(), req.FromAccountID, req.ToAccountID)
    respondJSON(w, http.StatusOK, map[string]string{"message": "Transfer successful"})
}

func DepositHandler(w http.ResponseWriter, r *http.Request) {
    defer r.Body.Close()
    var req DepositRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondError(w, http.StatusBadRequest, "Invalid request payload")
        return
    }

    if req.Amount.LessThanOrEqual(decimal.Zero) {
        respondError(w, http.StatusBadRequest, "Deposit amount must be positive")
        return
    }

    err := UpdateAccountBalance(req.ToAccountID, req.Amount)
    if err != nil {
        if strings.Contains(err.Error(), "not found") {
            respondError(w, http.StatusNotFound, err.Error())
        } else {
            respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to process deposit: %v", err))
        }
        return
    }

    account, _ := GetAccount(req.ToAccountID)
    tx := Transaction{
        ID:              GenerateID(),
        FromAccountID:   "",
        ToAccountID:     req.ToAccountID,
        Amount:          req.Amount,
        Timestamp:       time.Now(),
        TransactionType: "deposit",
        Description:     fmt.Sprintf("Deposit to account %s", account.Number),
    }
    AddTransaction(tx)

    log.Printf("Deposit of %s to account %s successful", req.Amount.String(), req.ToAccountID)
    respondJSON(w, http.StatusOK, map[string]string{"message": "Deposit successful"})
}

func ApplyLoanHandler(w http.ResponseWriter, r *http.Request) {
    defer r.Body.Close()
    var req ApplyLoanRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        respondError(w, http.StatusBadRequest, "Invalid request payload")
        return
    }

    if req.Amount.LessThanOrEqual(decimal.Zero) || req.TermMonths <= 0 {
        respondError(w, http.StatusBadRequest, "Loan amount and term must be positive")
        return
    }

    userID, ok := r.Context().Value(userContextKey).(string) // Извлекаем UserID из контекста
    if !ok {
        respondError(w, http.StatusUnauthorized, "User  not found in context")
        return
    }

    storage.mu.RLock()
    _, userExists := storage.users[userID] // Измените здесь на userID
    _, accountExists := storage.accounts[req.AccountID]
    storage.mu.RUnlock()

    if !userExists {
        respondError(w, http.StatusNotFound, fmt.Sprintf("User  %s not found", userID))
        return
    }
    if !accountExists {
        respondError(w, http.StatusNotFound, fmt.Sprintf("Account %s not found", req.AccountID))
        return
    }

    currentDate := time.Now().Format("2006-01-02")

    baseRate, err := GetCBRKeyRate(currentDate)
    if err != nil {
        log.Printf("Warning: Failed to get key rate, using default 10%%: %v", err)
        baseRate = decimal.NewFromInt(10)
    }

    interestRate := baseRate.Add(decimal.NewFromInt(5))

    monthlyPayment := CalculateMonthlyPayment(req.Amount, interestRate, req.TermMonths)
    startDate := time.Now()
    schedule := GeneratePaymentSchedule(req.Amount, interestRate, req.TermMonths, startDate, monthlyPayment)

    loan := Loan{
        ID:              GenerateID(),
        UserID:          req.UserID,
        AccountID:       req.AccountID,
        Amount:          req.Amount,
        InterestRate:    interestRate,
        TermMonths:      req.TermMonths,
        StartDate:       startDate,
        PaymentSchedule: schedule,
        RemainingAmount: req.Amount,
    }

    if err := AddLoan(loan); err != nil {
        respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to save loan: %v", err))
        return
    }

    err = UpdateAccountBalance(req.AccountID, req.Amount)
    if err != nil {
        respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to disburse loan funds: %v", err))
        return
    }

    tx := Transaction{
        ID:              GenerateID(),
        FromAccountID:   "", //
        ToAccountID:     req.AccountID,
        Amount:          req.Amount,
        Timestamp:       time.Now(),
        TransactionType: "loan_disbursement",
        Description:     fmt.Sprintf("Loan disbursement (ID: %s)", loan.ID),
    }
    AddTransaction(tx)

    log.Printf("Loan %s approved for user %s, amount %s, rate %s%%, term %d months. Funds disbursed to account %s.",
        loan.ID, req.UserID, req.Amount.String(), interestRate.String(), req.TermMonths, req.AccountID)

    respondJSON(w, http.StatusCreated, loan)
}

func GetLoanScheduleHandler(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    loanID := vars["loanId"]

    loan, ok := GetLoan(loanID)
    if !ok {
        respondError(w, http.StatusNotFound, fmt.Sprintf("Loan %s not found", loanID))
        return
    }

    log.Printf("Fetched payment schedule for loan %s", loanID)
    respondJSON(w, http.StatusOK, loan.PaymentSchedule)
}

func GetTransactionsHandler(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    accountID := vars["accountId"]

    if _, ok := GetAccount(accountID); !ok {
        respondError(w, http.StatusNotFound, fmt.Sprintf("Account %s not found", accountID))
        return
    }

    transactions := GetAccountTransactions(accountID)

    sort.Slice(transactions, func(i, j int) bool {
        return transactions[i].Timestamp.After(transactions[j].Timestamp)
    })

    log.Printf("Fetched %d transactions for account %s", len(transactions), accountID)
    respondJSON(w, http.StatusOK, transactions)
}

func GetFinancialSummaryHandler(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    userID := vars["userId"]

    accounts := GetUserAccounts(userID)
    loans := GetUserLoans(userID)

    totalBalance := decimal.Zero
    for _, acc := range accounts {
        totalBalance = totalBalance.Add(acc.Balance)
    }

    totalLoanDebt := decimal.Zero
    activeLoans := 0
    for _, loan := range loans {
        totalLoanDebt = totalLoanDebt.Add(loan.RemainingAmount)
        if loan.RemainingAmount.GreaterThan(decimal.Zero) {
            activeLoans++
        }
    }

    summary := map[string]interface{}{
        "user_id":               userID,
        "total_account_balance": totalBalance,
        "number_of_accounts":    len(accounts),
        "total_loan_debt":       totalLoanDebt,
        "active_loans":          activeLoans,
    }

    log.Printf("Generated financial summary for user %s", userID)
    respondJSON(w, http.StatusOK, summary)
}

func GetFinancialForecastHandler(w http.ResponseWriter, r *http.Request) {
    userID, ok := r.Context().Value(userContextKey).(string)
    if !ok {
        respondError(w, http.StatusUnauthorized, "User not found in context")
        return
    }

    accounts := GetUserAccounts(userID)
    transactions := []Transaction{}
    storage.mu.RLock()
    for _, tx := range storage.transactions {
        if tx.FromAccountID != "" && storage.accounts[tx.FromAccountID].UserID == userID {
            transactions = append(transactions, tx)
        } else if tx.ToAccountID != "" && storage.accounts[tx.ToAccountID].UserID == userID {
            transactions = append(transactions, tx)
        }
    }
    storage.mu.RUnlock()

    totalBalance := decimal.Zero
    for _, acc := range accounts {
        totalBalance = totalBalance.Add(acc.Balance)
    }

    // Считаем доходы и расходы за последний месяц
    oneMonthAgo := time.Now().AddDate(0, -1, 0)
    totalIncome := decimal.Zero
    totalExpenses := decimal.Zero

    for _, tx := range transactions {
        if tx.Timestamp.Before(oneMonthAgo) {
            continue
        }
        if tx.ToAccountID != "" && storage.accounts[tx.ToAccountID].UserID == userID {
            totalIncome = totalIncome.Add(tx.Amount)
        }
        if tx.FromAccountID != "" && storage.accounts[tx.FromAccountID].UserID == userID {
            totalExpenses = totalExpenses.Add(tx.Amount)
        }
    }

    projectedBalance := totalBalance.Add(totalIncome).Sub(totalExpenses)

    response := map[string]interface{}{
        "current_balance":        totalBalance,
        "projected_balance":      projectedBalance,
        "total_income_last_month": totalIncome,
        "total_expenses_last_month": totalExpenses,
    }

    respondJSON(w, http.StatusOK, response)
}