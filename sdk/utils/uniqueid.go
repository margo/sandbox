package utils

import "github.com/google/uuid"

func GenerateAppId() string {
	return generateUUID()
}

func GenerateInstanceId() string {
	return generateUUID()
}

func GenerateDeviceId() string {
	return generateUUID()
}

func generateUUID() string {
	return uuid.New().String()
}
