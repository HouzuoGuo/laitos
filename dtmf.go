package main

import (
	"bytes"
	"log"
	"strings"
)

var dtmfDecodeTable = map[string]string{
	` `:   ` `,
	`111`: `!`, `112`: `@`, `113`: `#`, `114`: `$`, `115`: `%`, `116`: `^`, `117`: `&`, `118`: `*`, `119`: `(`,
	`121`: "`", `122`: `~`, `123`: `)`, `124`: `-`, `125`: `_`, `126`: `=`, `127`: `+`, `128`: `[`, `129`: `{`,
	`131`: `]`, `132`: `}`, `133`: `\`, `134`: `|`, `135`: `;`, `136`: `:`, `137`: `'`, `138`: `"`, `139`: `,`,
	`141`: `<`, `142`: `.`, `143`: `>`, `144`: `/`, `145`: `?`,
	`1`: `0`, `11`: `1`, `12`: `2`, `13`: `3`, `14`: `4`, `15`: `5`, `16`: `6`, `17`: `7`, `18`: `8`, `19`: `9`,
	`2`: "a", `22`: `b`, `222`: `c`,
	`3`: "d", `33`: `e`, `333`: `f`,
	`4`: "g", `44`: `h`, `444`: `i`,
	`5`: "j", `55`: `k`, `555`: `l`,
	`6`: "m", `66`: `n`, `666`: `o`,
	`7`: "p", `77`: `q`, `777`: `r`, `7777`: `s`,
	`8`: "t", `88`: `u`, `888`: `v`,
	`9`: "w", `99`: `x`, `999`: `y`, `9999`: `z`,
} // Decode sequence of DTMF digits into letters and symbols

// Decode a sequence of character string sent via DTMF. Input is a sequence of key names (0-9 and *)
func DTMFDecode(digits string) string {
	digits = strings.TrimSpace(digits)
	if len(digits) == 0 {
		return ""
	}
	/*
		The rationale is following:
		- The number-pad should be able to enter nearly all characters (symbols, numerals, and letters).
		- Each character is entered either by a single number or a sequence of numbers.
		- Asterisk toggles between upper case and lower case. In the beginning letters are in lower case.
		- Number 0 either terminates a character sequence, or generate spaces if character sequence is already terminated.
		- A new character sequence begins automatically if previous character sequence is terminated or this number does not
		  continue the number sequence (e.g. terminate when 4 follows "333")
		- Symbols and numerals always require explicit termination of their sequence by a number 0.
	*/
	words := make([]string, 0, 256)
	word := make([]rune, 0, 8)
	for _, char := range digits {
		switch char {
		case '1':
			if len(word) > 0 && word[0] != '1' {
				// The running word isn't a symbol/numeral sequence, terminate it and start a new sequence.
				words = append(words, string(word))
				word = make([]rune, 0, 8)
			}
			word = append(word, char)
		case '2', '3', '4', '5', '6', '7', '8', '9':
			if len(word) > 0 && word[len(word)-1] != char && word[0] != '1' {
				// Not a consecutive digit, store the previous word.
				words = append(words, string(word))
				word = make([]rune, 0, 8)
			}
			word = append(word, char)
		case '0':
			if len(word) == 0 {
				// Consecutive 0s after previously terminated word are going to appear as spaces
				words = append(words, " ")
			} else {
				// Terminate stored word
				words = append(words, string(word))
				word = make([]rune, 0, 8)
			}
		case '*':
			// Terminate stored word and store an asterisk (shift case)
			if len(word) > 0 {
				words = append(words, string(word))
				word = make([]rune, 0, 8)
			}
			words = append(words, "*")
		default:
			// Simply discard
		}
	}
	// Store the very last word
	if len(word) > 0 {
		words = append(words, string(word))
	}
	// Translate word sequences into message string
	var message bytes.Buffer
	var shift bool
	for _, seq := range words {
		if seq == "*" {
			shift = !shift
		} else {
			decoded, found := dtmfDecodeTable[seq]
			if !found {
				log.Printf("DTMF decoding table failed to decode '%s'", seq)
				continue
			}
			if shift {
				decoded = strings.ToUpper(decoded)
			}
			message.WriteString(decoded)
		}
	}
	return message.String()
}
