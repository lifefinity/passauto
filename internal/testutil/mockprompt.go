//go:build ignore

package main

import (
	"bufio"
	"fmt"
	"os"
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Print("Password: ")
	if scanner.Scan() {
		pw := scanner.Text()
		if pw == "correct-password" {
			fmt.Println("Access granted")
		} else {
			fmt.Println("Access denied")
			os.Exit(1)
		}
	}

	fmt.Print("Continue? (yes/no): ")
	if scanner.Scan() {
		answer := scanner.Text()
		if answer == "yes" {
			fmt.Println("Done!")
		} else {
			fmt.Println("Aborted")
			os.Exit(1)
		}
	}
}
