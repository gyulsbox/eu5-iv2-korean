// Package korean holds the mechanical Korean-language cleanups that both the
// translation stage and the build stage need. It lives on its own because
// build must apply it after substituting a template's placeholders, and
// translate must apply it before storing one.
package korean

import (
	"strings"
	"unicode/utf8"
)

// Korean particle selection depends on whether the preceding syllable ends in
// a consonant. Where a particle follows a runtime placeholder that is genuinely
// unknowable, so the game's own Korean writes the paired form: 을(를).
//
// A translator told that rule tends to over-apply it and write the paired form
// after ordinary words too, where the consonant is perfectly well known. That
// reads as sloppy machine translation. It is also completely mechanical to fix,
// which is what this file does.

// particlePair is one alternation: the form used after a final consonant and
// the form used after a vowel.
type particlePair struct {
	afterConsonant string
	afterVowel     string
}

// pairs lists the alternations, longest first so that 으로/로 is matched before
// any shorter pair could claim its prefix.
var pairs = []particlePair{
	{"으로", "로"},
	{"을", "를"},
	{"은", "는"},
	{"이", "가"},
	{"과", "와"},
}

// Canonical renders a pair the way the game's Korean writes it when the
// following value is unknown.
func (p particlePair) Canonical() string {
	switch p.afterConsonant {
	// 와(과) and 으로(로) are the conventional orders for these two; the rest
	// lead with the post-consonant form.
	case "과":
		return "와(과)"
	case "으로":
		return "으로(로)"
	default:
		return p.afterConsonant + "(" + p.afterVowel + ")"
	}
}

// hasFinalConsonant reports whether a rune is a Hangul syllable ending in a
// consonant, plus whether it was a Hangul syllable at all.
func hasFinalConsonant(r rune) (final bool, hangul bool) {
	if r < 0xAC00 || r > 0xD7A3 {
		return false, false
	}
	return (r-0xAC00)%28 != 0, true
}

// endsInRieul reports whether a syllable's final consonant is ㄹ, which takes
// 로 rather than 으로.
func endsInRieul(r rune) bool {
	if r < 0xAC00 || r > 0xD7A3 {
		return false
	}
	return (r-0xAC00)%28 == 8
}

// digitReadings maps a digit to the Hangul syllable it is read as, so that a
// particle after a numeral agrees with how the numeral is spoken.
var digitReadings = map[rune]rune{
	'0': '영', '1': '일', '2': '이', '3': '삼', '4': '사',
	'5': '오', '6': '육', '7': '칠', '8': '팔', '9': '구',
}

// FixParticles corrects paired particle forms in a translated string.
//
// After a placeholder the paired form is right and is only normalised to the
// conventional spelling. After a Hangul syllable or a digit the correct single
// particle is known, so the pair is collapsed to it. After anything else -
// Latin text, punctuation - the pair is left alone, since guessing there would
// be worse than the paired form.
func FixParticles(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	i := 0
	for i < len(s) {
		matched := false
		for _, p := range pairs {
			for _, form := range []string{
				p.afterConsonant + "(" + p.afterVowel + ")",
				p.afterVowel + "(" + p.afterConsonant + ")",
				// Models also emit degenerate pairs like 과(과).
				p.afterConsonant + "(" + p.afterConsonant + ")",
				p.afterVowel + "(" + p.afterVowel + ")",
			} {
				if !strings.HasPrefix(s[i:], form) {
					continue
				}
				b.WriteString(resolve(b.String(), p))
				i += len(form)
				matched = true
				break
			}
			if matched {
				break
			}
		}
		if matched {
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		b.WriteRune(r)
		i += size
	}
	return b.String()
}

// resolve picks the right rendering of a pair given what precedes it.
func resolve(before string, p particlePair) string {
	prev, size := utf8.DecodeLastRuneInString(before)
	if size == 0 {
		return p.Canonical()
	}

	// Directly after a placeholder the substituted value is unknown, which is
	// the one case the paired form exists for.
	if prev == '⟧' {
		return p.Canonical()
	}

	if reading, ok := digitReadings[prev]; ok {
		prev = reading
	}

	final, hangul := hasFinalConsonant(prev)
	if !hangul {
		// Latin text or punctuation: no basis to choose, so keep the pair.
		return p.Canonical()
	}

	if p.afterConsonant == "으로" && endsInRieul(prev) {
		// ㄹ is the exception: 서울로, not 서울으로.
		return p.afterVowel
	}
	if final {
		return p.afterConsonant
	}
	return p.afterVowel
}

// Polish applies every mechanical cleanup to one Korean string.
func Polish(s string) string {
	return FixParticles(s)
}
