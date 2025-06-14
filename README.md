# Банковское приложение на Go

## Описание

Это REST API для управления банковскими операциями: регистрация и аутентификация пользователей с JWT, создание счетов, управление банковскими картами с безопасным хранением данных, переводы, кредиты с графиком платежей, аналитика, интеграция с Центробанком РФ и SMTP.

---

## Запуск сервиса

1. Убедитесь, что у вас установлен Go (версия 1.18+).
2. Клонируйте репозиторий и перейдите в каталог проекта.
3. Создайте файл `.env` с переменными окружения:
   ```env
   JWT_SECRET_KEY=your_jwt_secret
   PGP_PUBLIC_KEY_PATH=path/to/pgp_public_key.asc
   PGP_PRIVATE_KEY_PATH=path/to/pgp_private_key.asc
   SMTP_HOST=smtp.example.com
   SMTP_PORT=587
   SMTP_USERNAME=your_email@example.com
   SMTP_PASSWORD=your_password

4. Настройте подключение к базе данных в initDB() (строка подключения connStr).
5. Запустите сервис:
```
go run main.go
```
По умолчанию сервер слушает на http://localhost:8080.

## Как пользоваться сервисом

1. **Регистрация пользователя**:
- `POST /register`
  ```json
  {
    "username": "user1",
    "email": "user1@example.com",
    "password": "yourPassword123"
  }     
2. **Вход (аутентификация)**:
- `POST /login`
  ```json
  {
  "username": "user1",
  "password": "yourPassword123"
  }      
3. **Использование JWT**
- **Для всех защищённых эндпоинтов передавайте в заголовке:**
  ```
  Authorization: Bearer <JWT_TOKEN>       
4. **Создание банковского счета**:
- `POST /api/accounts`
  ```json
  {  }     
5. **Получение счетов пользователя** 
-`GET /api/users/{userId}/accounts`
- Ответ: список счетов пользователя.  
6. **Генерация банковской карты для счета**
- `POST /api/cards`
 ```json
  {
  "account_id": "<account_id>"
  }   
  ```   
7. **Получение карт счета**
-`GET /api/accounts/{accountId}/cards`
- Ответ: список карт с замаскированными номерами. 
8. **Оплата по карте**
- `POST /api/payments/card`
  ```json
  {
  "card_number": "<card_id>",
  "cvv": "123",
  "amount": "100.50",
  "merchant": "Store XYZ"
  }
9. **Перевод между счетами**
- `POST /api/transfers`
  ```json
  {
  "from_account_id": "<account_id_1>",
  "to_account_id": "<account_id_2>",
  "amount": "50.00"
  }
10. **Пополнение счета**
- `POST /api/deposits`
  ```json
  {
  "to_account_id": "<account_id>",
  "amount": "200.00"
  }
11. **Оформление кредита**
- `POST /api/loans`
  ```json
  {
  "user_id": "<user_id>",
  "account_id": "<account_id>",
  "amount": "10000.00",
  "term_months": 12
  }
12. **Получение графика платежей по кредиту**
- `GET /api/loans/{loanId}/schedule`
- Ответ: массив платежей с датами и суммами.
13. **Получение транзакций счета**
- `GET /api/analytics/transactions/{accountId}`
14. **Финансовая сводка пользователя**
- `GET /api/analytics/summary/{userId}`
- Ответ: баланс, количество счетов, сумма долгов по кредитам.
15. **Финансовый прогноз**
- `GET /api/analytics/forecast`
- Использует JWT для определения пользователя.
- Ответ: текущий баланс, прогнозируемый баланс, доходы и расходы за последний месяц.

## Используемые внешние библиотеки

1. **github.com/google/uuid** - для генерации UUID (идентификаторов пользователей, счетов, карт)
2. **github.com/gorilla/mux** — маршрутизатор HTTP-запросов (роутер) с поддержкой переменных в URL и middleware
3. **github.com/shopspring/decimal** — работа с денежными значениями с точной арифметикой
4. **golang.org/x/crypto** — криптография: bcrypt, PGP и др.
5. **github.com/ProtonMail/go-crypto** — PGP-шифрование/дешифрование данных
6. **github.com/golang-jwt/jwt/v5** — JWT аутентификация
7. **github.com/joho/godotenv** - для загрузки конфигурации из .env файла
8. **github.com/lib/pq** — PostgreSQL драйвер для Go
9. **github.com/sirupsen/logrus** — продвинутое логирование с уровнями и форматированием
10. **github.com/beevik/etree** — XML парсер (для интеграции с ЦБ РФ)