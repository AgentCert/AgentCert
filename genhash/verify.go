package main

import (
	"fmt"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	hash := "$2a$08$Qs37QPtj3qoYKqHVj7XwmO4NpPuBh6Zpe8YP0umTO0g3dXxIlaDPC"
	password := "litmus"
	
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		fmt.Println("Password does NOT match:", err)
	} else {
		fmt.Println("Password matches!")
	}
}
