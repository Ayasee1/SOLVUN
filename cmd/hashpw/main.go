// Command hashpw prints a bcrypt hash for ADMIN_PASSWORD_HASH.
//
// Usage:
//
//	go run ./cmd/hashpw            # prompts on stdin
//	go run ./cmd/hashpw 'my pass'  # password as an argument
//
// Prefer the stdin form: arguments are visible in the process list and shell
// history.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	cost := flag.Int("cost", bcrypt.DefaultCost, "bcrypt cost")
	flag.Parse()

	password := strings.TrimSpace(strings.Join(flag.Args(), " "))

	if password == "" {
		fmt.Fprint(os.Stderr, "Password: ")
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			fatal("could not read the password: %v", err)
		}
		password = strings.TrimSpace(line)
	}

	if len(password) < 8 {
		fatal("password must be at least 8 characters")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), *cost)
	if err != nil {
		fatal("could not hash the password: %v", err)
	}

	fmt.Println(string(hash))
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
