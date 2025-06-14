package main

import (
    "context"
    "database/sql"

    "net/http"
    "os"
    "time"
    
    log "github.com/sirupsen/logrus"

    "github.com/gorilla/mux"
    _ "github.com/lib/pq"
)

var db *sql.DB

type contextKey string

const userContextKey contextKey = "user"

func initDB() {
    var err error
    connStr := "host=localhost port=5432 user=postgres password=admin dbname=BankApp sslmode=disable" // Укажите свои параметры подключения
    db, err = sql.Open("postgres", connStr)
    if err != nil {
        log.Fatalf("Не удалось подключиться к базе данных: %v", err)
    }

    if err := db.Ping(); err != nil {
        log.Fatalf("Не удалось проверить соединение с базой данных: %v", err)
    }

    log.Println("Успешно подключено к базе данных.")
}

func JWTMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        tokenStr := r.Header.Get("Authorization")
        if tokenStr == "" {
            http.Error(w, "Отсутствует токен", http.StatusUnauthorized)
            return
        }

        claims, err := ValidateJWT(tokenStr)
        if err != nil {
            http.Error(w, "Недействительный токен", http.StatusUnauthorized)
            return
        }

        ctx := context.WithValue(r.Context(), userContextKey, claims.UserID)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

func main() {
    log.SetOutput(os.Stdout)
    log.SetFormatter(&log.TextFormatter{
        FullTimestamp: true,
    })
    log.SetLevel(log.InfoLevel)

    log.Println("Запуск Simple Bank API...")

    initDB()         
    defer db.Close() 

    // Запуск шедулера для автоматической обработки платежей
    go func() {
    ticker := time.NewTicker(12 * time.Hour)
    defer ticker.Stop()
    for {
        <-ticker.C
        ProcessPayments(db)
    }
}()

    // Получаем курсы валют с ЦБ РФ
    date := time.Now().Format("2025-06-14") 
    keyRate, err := GetCBRKeyRate(date)
    if err != nil {
        log.Printf("Ошибка при получении курса валюты: %v", err)
    } else {
        log.Printf("Текущий курс валюты: %s", keyRate)
    }

    r := mux.NewRouter()

    // Открытые маршруты
    r.HandleFunc("/register", RegisterUserHandler).Methods("POST")
    r.HandleFunc("/login", LoginUserHandler).Methods("POST")

    // Защищённые маршруты
    secured := r.PathPrefix("/api").Subrouter()
    secured.Use(JWTMiddleware)

    secured.HandleFunc("/accounts", CreateAccountHandler).Methods("POST")
    secured.HandleFunc("/users/{userId}/accounts", GetUserAccountsHandler).Methods("GET")
    secured.HandleFunc("/cards", GenerateCardHandler).Methods("POST")
    secured.HandleFunc("/accounts/{accountId}/cards", GetAccountCardsHandler).Methods("GET")
    secured.HandleFunc("/payments/card", PayWithCardHandler).Methods("POST")
    secured.HandleFunc("/transfers", TransferHandler).Methods("POST")
    secured.HandleFunc("/deposits", DepositHandler).Methods("POST")
    secured.HandleFunc("/loans", ApplyLoanHandler).Methods("POST")
    secured.HandleFunc("/loans/{loanId}/schedule", GetLoanScheduleHandler).Methods("GET")
    secured.HandleFunc("/analytics/transactions/{accountId}", GetTransactionsHandler).Methods("GET")
    secured.HandleFunc("/analytics/summary/{userId}", GetFinancialSummaryHandler).Methods("GET")
    secured.HandleFunc("/analytics/forecast", GetFinancialForecastHandler).Methods("GET")

    port := "8080"
    log.Infof("Сервер запускается на порту %s", port)

    loggedRouter := loggingMiddleware(r)

    err = http.ListenAndServe(":"+port, loggedRouter)
    if err != nil {
        log.Fatalf("Не удалось запустить сервер: %v", err)
    }
}

func loggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        log.Infof("--> %s %s %s", r.Method, r.RequestURI, r.Proto)
        next.ServeHTTP(w, r)
        log.Infof("<-- %s %s (%v)", r.Method, r.RequestURI, time.Since(start))
    })
}