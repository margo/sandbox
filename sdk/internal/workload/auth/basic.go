package auth

func BasicAuthCredentials(username, password string) map[string]string {
	return map[string]string{
		"Authorization": "Basic " + encodeBasicAuth(username, password),
	}
}
func encodeBasicAuth(username, password string) string {
	// This function encodes the username and password in Base64 format
	// to create a Basic Authentication header.
	// In a real implementation, you would use a proper Base64 encoding function.
	return "encoded_" + username + ":" + password // Placeholder for actual Base64 encoding
}
