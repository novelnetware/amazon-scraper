package utils

import (
	"regexp"
	"strings"
)

// UniqueStrings یک اسلایس از رشته‌ها را دریافت کرده و یک اسلایس جدید
// بدون اعضای تکراری برمی‌گرداند.
func UniqueStrings(slice []string) []string {
	keys := make(map[string]bool)
	uniqueSlice := []string{}
	for _, entry := range slice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			uniqueSlice = append(uniqueSlice, entry)
		}
	}
	return uniqueSlice
}

// slugRegex matches any character that is NOT a Persian letter, an English letter, a number, or a hyphen.
var slugRegex = regexp.MustCompile(`[^\p{L}\p{N}-]+`)

// CreateSlug generates a WordPress-friendly slug from a title.
func CreateSlug(title string) string {
	// 1. Replace spaces with hyphens
	slug := strings.ReplaceAll(title, " ", "-")

	// 2. Remove all invalid characters using the regex
	slug = slugRegex.ReplaceAllString(slug, "")

	// 3. Convert to lowercase
	slug = strings.ToLower(slug)

	return slug
}
