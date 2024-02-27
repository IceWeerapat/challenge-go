package main

import (
	"bufio"
	"challenge/cipher"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/omise/omise-go"
	"github.com/omise/omise-go/operations"
)

type Donation struct {
	Name           string
	AmountSubunits int64
	CCNumber       string
	CVV            string
	ExpMonth       int
	ExpYear        int
}

type ChargeRequest struct {
	Amount   int    `json:"amount"`
	Currency string `json:"currency"`
	Card     string `json:"card"`
}

type Summary struct {
	TotalReceived       int64
	SuccessfullyDonated int64
	FaultyDonation      int64
	Donors              []Donation
}

func main() {

	decryptedFilePath := "./data/" + os.Args[1]

	encryptedFilePath := "./data/fng.1000.csv.rot128"

	encryptedFile, err := os.Open(encryptedFilePath)
	if err != nil {
		fmt.Printf("Failed to open encrypted file: %v\n", err)
		os.Exit(1)
	}
	defer encryptedFile.Close()

	rotReader, err := cipher.NewRot128Reader(encryptedFile)
	if err != nil {
		fmt.Printf("Failed to create Rot128Reader: %v\n", err)
		os.Exit(1)
	}

	decryptedFile, err := os.Create(decryptedFilePath)
	if err != nil {
		fmt.Printf("Failed to create decrypted file: %v\n", err)
		os.Exit(1)
	}
	defer decryptedFile.Close()

	writer := bufio.NewWriter(decryptedFile)
	defer writer.Flush()

	_, err = io.Copy(writer, rotReader)
	if err != nil {
		fmt.Printf("Failed to decrypt file: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Decryption complete.")

	donations, err := parseCSV("./data/fng.1000.csv")
	if err != nil {
		fmt.Printf("Failed to parse CSV: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("performing donations")
	var summary Summary
	for i := 1; i < len(donations); i++ {
		donation := donations[i]
		err := createCharge(donation)
		if err != nil {
			fmt.Printf("Failed to create charge for %s: %v\n", donation.Name, err)
			summary.FaultyDonation += donation.AmountSubunits
			continue
		}
		summary.SuccessfullyDonated += donation.AmountSubunits
		summary.TotalReceived += donation.AmountSubunits
		summary.Donors = append(summary.Donors, donation)
	}

	printSummary(summary)
}

func parseCSV(filePath string) ([]Donation, error) {

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var donations []Donation
	reader := csv.NewReader(file)
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return donations, err
		}

		amountSubunits, _ := strconv.ParseInt(record[1], 10, 64)
		expMonth, _ := strconv.Atoi(record[4])
		expYear, _ := strconv.Atoi(record[5])

		donations = append(donations, Donation{
			Name:           record[0],
			AmountSubunits: amountSubunits,
			CCNumber:       record[2],
			CVV:            record[3],
			ExpMonth:       expMonth,
			ExpYear:        expYear,
		})
	}

	return donations, nil
}

func createCharge(donation Donation) error {

	client, err := omise.NewClient(os.Getenv("OMISE_PUBLIC_KEY"), os.Getenv("OMISE_SECRET_KEY"))
	if err != nil {
		return fmt.Errorf("failed to create Omise client: %v", err)
	}

	token, createToken := &omise.Token{}, &operations.CreateToken{
		Name:            donation.Name,
		Number:          donation.CCNumber,
		ExpirationMonth: time.Month(donation.ExpMonth),
		ExpirationYear:  donation.ExpYear,
		SecurityCode:    donation.CVV,
	}
	if err := client.Do(token, createToken); err != nil {
		return fmt.Errorf("failed to create token: %v", err)
	}

	charge, createCharge := &omise.Charge{}, &operations.CreateCharge{
		Amount:   donation.AmountSubunits,
		Currency: "thb",
		Card:     token.ID,
	}

	if err := client.Do(charge, createCharge); err != nil {
		return fmt.Errorf("failed to create charge: %v", err)
	}

	return nil
}

func printSummary(summary Summary) {
	fmt.Printf("total received: THB  %.2f\n", float64(summary.TotalReceived)/100)
	fmt.Printf("successfully donated: THB  %.2f\n", float64(summary.SuccessfullyDonated)/100)
	fmt.Printf("faulty donation: THB   %.2f\n", float64(summary.FaultyDonation)/100)

	if len(summary.Donors) > 0 {
		average := float64(summary.SuccessfullyDonated) / float64(len(summary.Donors))
		fmt.Printf("average per person: THB      %.2f\n", average/100)

		sort.Slice(summary.Donors, func(i, j int) bool {
			return summary.Donors[i].AmountSubunits > summary.Donors[j].AmountSubunits
		})

		fmt.Println("top donors:")
		for i, donor := range summary.Donors {
			if i >= 3 {
				break
			}
			fmt.Printf("%s\n", donor.Name)
		}
	}
}
