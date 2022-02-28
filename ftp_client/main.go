package main

import (
	"fmt"

	"github.com/Ansi4Ansi/teta_repo/ftp_client/ftp"
)

func main() {
	conn, err := ftp.Connect("localhost", 21)
	if err != nil {
		fmt.Println("unable to connect, error:", err)
		return
	}
	defer conn.Close()

	err = conn.Login("test_ftp", "test_password")
	if err != nil {
		fmt.Println("login failed, error:", err)
		return
	}
	defer conn.Quit()

}
