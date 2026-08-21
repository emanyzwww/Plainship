package main

import "github.com/emanyzwww/papership-client/cmd/client/cmd"

func main() {
	err := cmd.NewRootCmd().Execute()
	if err != nil {
		return
	}
}
