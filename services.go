	package main

	import (
		
		"io"
		"database/sql"
		"encoding/xml"
		"fmt"
		"log"
		"net/http"
		"net/smtp"
		"strconv"
    	"strings"
		"sync"
		"time"
		"github.com/beevik/etree"
		"github.com/shopspring/decimal"
	)

	type payment struct {
		LoanID        int
		AccountID     int
		DueDate       time.Time
		Amount        decimal.Decimal
		PenaltyAmount decimal.Decimal
		Paid          bool
	}

	const cbrSOAPURL = "http://www.cbr.ru/DailyInfoWebServ/DailyInfo.asmx"

	type ValCurs struct {
		XMLName xml.Name `xml:"soap:Envelope"`
		Body    struct {
			GetCursOnDateResponse struct {
				GetCursOnDateResult string `xml:"GetCursOnDateResult"`
			} `xml:"GetCursOnDateResponse"`
		} `xml:"soap:Body"`
	}
	
	var cachedKeyRate struct {
		rate decimal.Decimal
		time time.Time
	}
	var keyRateMutex sync.Mutex

	// Получаем ключевую ставку из ЦБ
	func GetCBRKeyRate(date string) (decimal.Decimal, error) {
    keyRateMutex.Lock()
    defer keyRateMutex.Unlock()

    if !cachedKeyRate.rate.IsZero() && time.Since(cachedKeyRate.time) < time.Hour {
        log.Println("Используем кэшированную ключевую ставку")
        return cachedKeyRate.rate, nil
    }

    soapBody := fmt.Sprintf(`
<soap12:Envelope xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
                 xmlns:xsd="http://www.w3.org/2001/XMLSchema"
                 xmlns:soap12="http://www.w3.org/2003/05/soap-envelope">
  <soap12:Body>
    <KeyRate xmlns="http://web.cbr.ru/">
      <OnDate>%s</OnDate>
    </KeyRate>
  </soap12:Body>
</soap12:Envelope>`, date)

    req, err := http.NewRequest("POST", cbrSOAPURL, strings.NewReader(soapBody))
    if err != nil {
        return decimal.Zero, fmt.Errorf("не удалось создать запрос: %w", err)
    }

    req.Header.Set("Content-Type", "application/soap+xml; charset=utf-8")
    req.Header.Set("SOAPAction", "http://web.cbr.ru/KeyRate")

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return decimal.Zero, fmt.Errorf("не удалось отправить запрос: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return decimal.Zero, fmt.Errorf("неожиданный HTTP статус: %s", resp.Status)
    }

    bodyBytes, err := io.ReadAll(resp.Body)
    if err != nil {
        return decimal.Zero, fmt.Errorf("не удалось прочитать тело ответа: %w", err)
    }

    log.Printf("Ответ ЦБ РФ:\n%s\n", string(bodyBytes))

    doc := etree.NewDocument()
    if err := doc.ReadFromBytes(bodyBytes); err != nil {
        return decimal.Zero, fmt.Errorf("не удалось прочитать XML: %w", err)
    }
    
    var body *etree.Element
    for _, el := range doc.Root().ChildElements() {
        if strings.HasSuffix(el.Tag, "Body") {
            body = el
            break
        }
    }
    if body == nil {
        return decimal.Zero, fmt.Errorf("элемент Body не найден")
    }

    var keyRateResponse *etree.Element
    for _, el := range body.ChildElements() {
        if strings.HasSuffix(el.Tag, "KeyRateResponse") {
            keyRateResponse = el
            break
        }
    }
    if keyRateResponse == nil {
        return decimal.Zero, fmt.Errorf("элемент KeyRateResponse не найден")
    }

    keyRateResult := keyRateResponse.FindElement("./KeyRateResult")
    if keyRateResult == nil {
        return decimal.Zero, fmt.Errorf("элемент KeyRateResult не найден")
    }

    keyRateStr := strings.TrimSpace(keyRateResult.Text())
    if keyRateStr == "" {
        return decimal.Zero, fmt.Errorf("пустое значение ключевой ставки")
    }

    rateFloat, err := strconv.ParseFloat(keyRateStr, 64)
    if err != nil {
        return decimal.Zero, fmt.Errorf("не удалось преобразовать ключевую ставку в число: %w", err)
    }

    rate := decimal.NewFromFloat(rateFloat)
    cachedKeyRate.rate = rate
    cachedKeyRate.time = time.Now()

    return rate, nil
}

	// Конфигурация SMTP
	var smtpConfig = struct {
		Host     string
		Port     int
		Username string
		Password string
		From     string
	}{
		Host:     "smtp.example.com",
		Port:     587,
		Username: "your_email@example.com",
		Password: "your_password",
		From:     "bankapp@example.com",
	}

	// ProcessPayments обрабатывает автоматические списания и штрафы
	func ProcessPayments(db *sql.DB) {
		log.Println("Запуск обработки автоматических платежей и штрафов...")

		// Получаем все активные кредиты с платежами на сегодня или ранее
		query := `
    SELECT loan_id, account_id, payment_due_date, payment_amount, penalty_amount, paid
    FROM loan_payments
    WHERE payment_due_date <= $1 AND paid = FALSE
`

		fmt.Printf("%q\n", query)
		fmt.Printf("%v\n", []byte(query))	

		rows, err := db.Query(query, time.Now())

		if err != nil {
			log.Printf("Ошибка при получении платежей: %v", err)
			return
		}
		defer rows.Close()

		var payments []payment
		for rows.Next() {
			var p payment
			if err := rows.Scan(&p.LoanID, &p.AccountID, &p.DueDate, &p.Amount, &p.PenaltyAmount, &p.Paid); err != nil {
				log.Printf("Ошибка при сканировании строки: %v", err)
				continue
			}
			payments = append(payments, p)
		}

		for _, p := range payments {			
			if err := processPayment(db, p); err != nil {
				log.Printf("Ошибка при обработке платежа: %v", err)
			}
		}
	}

	func processPayment(db *sql.DB, p payment) error {		
		_, err := db.Exec(`
			UPDATE loan_payments
			SET paid = TRUE
			WHERE loan_id = $1
		`, p.LoanID)

		if err != nil {
			return fmt.Errorf("не удалось обновить статус платежа: %w", err)
		}

		log.Printf("Платеж по кредиту %d успешно обработан", p.LoanID)		
		return nil
	}


	// SendEmailNotification отправляет уведомление по электронной почте
	func SendEmailNotification(to, subject, body string) error {
		if smtpConfig.Host == "smtp.example.com" {
			log.Printf("SMTP не настроен. Пропускаем отправку письма на %s: Тема: %s", to, subject)
			return nil
		}

		auth := smtp.PlainAuth("", smtpConfig.Username, smtpConfig.Password, smtpConfig.Host)

		msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s\r\n",
			smtpConfig.From, to, subject, body)

		addr := fmt.Sprintf("%s:%d", smtpConfig.Host, smtpConfig.Port)

		err := smtp.SendMail(addr, auth, smtpConfig.From, []string{to}, []byte(msg))
		if err != nil {
			log.Printf("Ошибка при отправке письма на %s: %v", to, err)
			return fmt.Errorf("не удалось отправить письмо: %w", err)
		}

		log.Printf("Письмо успешно отправлено на %s", to)
		return nil
	}