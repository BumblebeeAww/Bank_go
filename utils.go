package main

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func GenerateID() string {
	return uuid.NewString()
}

func GenerateAccountNumber() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(9000000000))
	return fmt.Sprintf("40817810%010d", n.Int64()+1000000000)
}

func GenerateCardNumber() string {
	n1, _ := rand.Int(rand.Reader, big.NewInt(9000))
	n2, _ := rand.Int(rand.Reader, big.NewInt(10000))
	n3, _ := rand.Int(rand.Reader, big.NewInt(10000))
	n4, _ := rand.Int(rand.Reader, big.NewInt(10000))
	return fmt.Sprintf("4%03d%04d%04d%04d", n1.Int64()+100, n2.Int64(), n3.Int64(), n4.Int64())
}

func GenerateCVV() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(900))
	return fmt.Sprintf("%03d", n.Int64()+100)
}

func GenerateExpiryDate() (int, int) {
	now := time.Now()
	year := now.Year() + 4
	month := int(now.Month())
	return month, year
}

func CalculateMonthlyPayment(loanAmount decimal.Decimal, annualRate decimal.Decimal, termMonths int) decimal.Decimal {
	if termMonths <= 0 {
		return decimal.Zero
	}
	monthlyRate := annualRate.Div(decimal.NewFromInt(12)).Div(decimal.NewFromInt(100))

	if monthlyRate.IsZero() {
		return loanAmount.Div(decimal.NewFromInt(int64(termMonths)))
	}

	onePlusRate := decimal.NewFromInt(1).Add(monthlyRate)
	powOnePlusRate := onePlusRate.Pow(decimal.NewFromInt(int64(termMonths)))

	numerator := monthlyRate.Mul(powOnePlusRate)
	denominator := powOnePlusRate.Sub(decimal.NewFromInt(1))

	if denominator.IsZero() {
		return decimal.Zero
	}

	monthlyPayment := loanAmount.Mul(numerator.Div(denominator))

	return monthlyPayment.RoundBank(2)
}

func GeneratePaymentSchedule(loanAmount decimal.Decimal, annualRate decimal.Decimal, termMonths int, startDate time.Time, monthlyPayment decimal.Decimal) []Payment {
	schedule := make([]Payment, 0, termMonths)
	remainingPrincipal := loanAmount
	monthlyRate := annualRate.Div(decimal.NewFromInt(12)).Div(decimal.NewFromInt(100))

	for i := 0; i < termMonths; i++ {
		dueDate := startDate.AddDate(0, i+1, 0)

		interestPart := remainingPrincipal.Mul(monthlyRate).RoundBank(2)
		principalPart := monthlyPayment.Sub(interestPart)

		if i == termMonths-1 || remainingPrincipal.Sub(principalPart).LessThanOrEqual(decimal.Zero) {
			principalPart = remainingPrincipal
			monthlyPayment = principalPart.Add(interestPart).RoundBank(2)
		}

		payment := Payment{
			DueDate:       dueDate,
			Amount:        monthlyPayment,
			InterestPart:  interestPart,
			PrincipalPart: principalPart,
			Paid:          false,
		}
		schedule = append(schedule, payment)

		remainingPrincipal = remainingPrincipal.Sub(principalPart)
		if remainingPrincipal.LessThanOrEqual(decimal.Zero) {
			break
		}
	}
	return schedule
}

// Проверка номера карты по алгоритму Луна
func ValidateCardNumberLuhn(cardNumber string) bool {
    sum := 0
    alt := false
    for i := len(cardNumber) - 1; i >= 0; i-- {
        digit := int(cardNumber[i] - '0')
        if alt {
            digit *= 2
            if digit > 9 {
                digit -= 9
            }
        }
        sum += digit
        alt = !alt
    }
    return sum%10 == 0
}

// Вычисление контрольной цифры для номера карты
func calculateLuhnCheckDigit(number string) int {
    sum := 0
    alt := true
    for i := len(number) - 1; i >= 0; i-- {
        digit := int(number[i] - '0')
        if alt {
            digit *= 2
            if digit > 9 {
                digit -= 9
            }
        }
        sum += digit
        alt = !alt
    }
    mod := sum % 10
    if mod == 0 {
        return 0
    }
    return 10 - mod
}

// Генерация валидного номера карты с проверкой по Луна
func GenerateValidCardNumber() string {
    prefix := "4" // например, Visa начинается с 4
    length := 16
    for {
        number := prefix
        for len(number) < length-1 {
            n, _ := rand.Int(rand.Reader, big.NewInt(10))
            number += fmt.Sprintf("%d", n.Int64())
        }
        checkDigit := calculateLuhnCheckDigit(number)
        cardNumber := number + fmt.Sprintf("%d", checkDigit)
        if ValidateCardNumberLuhn(cardNumber) {
            return cardNumber
        }
    }
}