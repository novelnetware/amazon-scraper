package utils

import "testing"

func TestParsePrice(t *testing.T) {
	// ما از یک جدول تست برای بررسی چندین حالت مختلف استفاده می‌کنیم
	testCases := []struct {
		name     string  // نام تست
		input    string  // ورودی تابع
		expected float64 // خروجی مورد انتظار
	}{
		{"Standard Price", "AED 1,079.00", 1079.00},
		{"Price with Comma", "AED 2,550.50", 2550.50},
		{"Price without Comma", "AED 350.75", 350.75},
		{"Integer Price", "AED 99", 99.0},
		{"Empty String", "", 0.0},
		{"Invalid String", "No Price", 0.0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// تابع اصلی را با ورودی تست فراخوانی می‌کنیم
			result := ParsePrice(tc.input)

			// نتیجه را با خروجی مورد انتظار مقایسه می‌کنیم
			if result != tc.expected {
				t.Errorf("ParsePrice(%q) = %f; want %f", tc.input, result, tc.expected)
			}
		})
	}
}
