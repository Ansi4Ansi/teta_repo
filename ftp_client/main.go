package main

import (
	"bytes"
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

	err = conn.Login("user", "password")
	if err != nil {
		fmt.Println("login failed, error:", err)
		return
	}
	defer conn.Quit()

	source := bytes.NewBuffer([]byte("This is the file content."))
	err = conn.Upload(source, "/example_upload.txt")
	if err != nil {
		fmt.Println("unable to upload file, error:", err)
	}
}
