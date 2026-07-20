// Package money holds the platform's supported settlement currencies.
//
// The platform is a worldwide freight marketplace: each deal is priced in the
// currency the party naming the price chooses (no FX conversion — the two
// sides settle in one currency they both accept). Every price-bearing row
// (offers, customs offers, driver bids, warehouse offers, transport proposals)
// already carries a `currency` column; this package is the single source of
// truth for which codes are accepted and how they are normalized.
package money

import "strings"

// Supported lists the ISO-4217 codes the UI offers and the API accepts. It is
// intentionally a curated set of common freight-trade currencies rather than
// the full ISO list — extend it here when a new corridor needs one. Keys are
// upper-case alpha-3 codes.
var Supported = map[string]bool{
	"USD": true, // US dollar
	"EUR": true, // Euro
	"CNY": true, // Chinese yuan
	"RUB": true, // Russian ruble
	"KZT": true, // Kazakhstani tenge
	"GBP": true, // Pound sterling
	"TRY": true, // Turkish lira
	"AED": true, // UAE dirham
	"KRW": true, // South Korean won
	"JPY": true, // Japanese yen
	"INR": true, // Indian rupee
	"PLN": true, // Polish zloty
	"KGS": true, // Kyrgyzstani som
	"UZS": true, // Uzbekistani som
	"GEL": true, // Georgian lari
	"AZN": true, // Azerbaijani manat
	"BYN": true, // Belarusian ruble
	"BRL": true, // Brazilian real
	"IRR": true, // Iranian rial
	"PKR": true, // Pakistani rupee
	"MNT": true, // Mongolian tugrik
}

// Fallback is the platform default used when a request omits a currency. It
// must be a member of Supported.
const Fallback = "USD"

// Normalize upper-cases and trims a currency code and returns it only if it is
// supported; otherwise it returns "". An empty input yields "".
func Normalize(code string) string {
	c := strings.ToUpper(strings.TrimSpace(code))
	if c == "" || !Supported[c] {
		return ""
	}
	return c
}

// IsValid reports whether a (case-insensitive) code is a supported currency.
func IsValid(code string) bool {
	return Normalize(code) != ""
}
