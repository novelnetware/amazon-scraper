package utils

import (
	"log"
	"regexp"
	"strconv"
	"strings"
)

// priceRegex is a powerful regex to find the first valid price number in a string.
// It handles integers (1,079), decimals (119.00), and commas.
var priceRegex = regexp.MustCompile(`[\d,]+(?:\.\d+)?`)

// ParsePrice cleans a price string and converts it to a float64.
// This new version is more robust and handles complex strings like "List Price: AED 219.41".
func ParsePrice(priceStr string) float64 {
	if priceStr == "" {
		return 0.0
	}

	// 1. Find the first number-like pattern in the string.
	foundPrice := priceRegex.FindString(priceStr)
	if foundPrice == "" {
		return 0.0
	}

	// 2. Remove commas to make it a valid number for parsing.
	cleanedStr := strings.ReplaceAll(foundPrice, ",", "")

	// 3. Convert to float64.
	price, err := strconv.ParseFloat(cleanedStr, 64)
	if err != nil {
		log.Printf("ParsePrice: Failed to parse '%s' from original string '%s': %v", cleanedStr, priceStr, err)
		return 0.0
	}

	return price
}
