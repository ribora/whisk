package validate

import "golang.org/x/text/language"

// BCP 47 language tag: "en", "en-US", "zh-Hant-CN".
func IsLocale(value string, args ...string) (bool, error) {
	if value == "" {
		return false, nil
	}
	_, err := language.Parse(value)
	if err != nil {
		return false, err
	}
	return true, nil
}
