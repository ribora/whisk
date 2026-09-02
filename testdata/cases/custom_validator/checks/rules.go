package checks

// IsPasscode is a user-defined validator picked up via storage-path
// scanning. The PascalCase'd tag name "passcode" maps to IsPasscode.
func IsPasscode(s string) bool {
	return len(s) == 6
}
