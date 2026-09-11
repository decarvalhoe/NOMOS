// Package main is a security-gate fixture: it deliberately calls a vulnerable
// symbol of an old golang.org/x/text so that govulncheck reports GO-2021-0113
// as CALLED. It is never built into a product artifact.
package main

import (
	"fmt"

	"golang.org/x/text/language"
)

func main() {
	tags, _, err := language.ParseAcceptLanguage("fr-CH, fr;q=0.9, en;q=0.8")
	fmt.Println(tags, err)
}
