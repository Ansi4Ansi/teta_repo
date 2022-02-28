package ftp

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
)

type FTP struct {
	host    string
	port    int
	user    string
	passwd  string
	pasv    int
	cmd     string
	Code    int
	Message string
	Debug   bool
	conn    net.Conn
}

func (ftp *FTP) newdataconn() (net.Conn, error) {
	ftp.Pasv()
	if ftp.pasv > 0 {
		dataconn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", ftp.host, ftp.pasv))

		return dataconn, err
	} else {
		return nil, errors.New("no new data link ")
	}
}

func (ftp *FTP) Pwd() {
	ftp.Request("PWD")
}

func (ftp *FTP) Cwd(path string) {
	ftp.Request("CWD " + path)
}

func (ftp *FTP) Mkd(path string) {
	ftp.Request("MKD " + path)
}

func (ftp *FTP) Size(path string) (size int) {
	ftp.Request("SIZE " + path)
	size, _ = strconv.Atoi(ftp.Message)
	return
}

func (ftp *FTP) List() (string, error) {
	list_cont := ""
	dataconn, err := ftp.newdataconn()
	if err != nil {
		return list_cont, err
	}
	ftp.Request("LIST")
	if ftp.Code != 150 {
		dataconn.Close()
		return list_cont, errors.New(ftp.Message)
	}
	for {
		buf := make([]byte, 1024)
		n, err := dataconn.Read(buf)
		if err == io.EOF {
			break
		} else if err != nil {
			dataconn.Close()
			return list_cont, err
		}
		list_cont = list_cont + string(buf[:n])
	}

	dataconn.Close()
	ftp.pasv = 0
	ftp.Response()
	return list_cont, nil
}

func (ftp *FTP) Stor(file string) error {
	File, err := os.Open(file)
	if err != nil {
		return err
	}
	dataconn, err := ftp.newdataconn()
	if err != nil {
		return err
	}
	ftp.Request("STOR " + file)
	if ftp.Code != 150 {
		dataconn.Close()
		return errors.New(ftp.Message)
	}

	io.Copy(dataconn, File)
	File.Close()
	dataconn.Close()
	ftp.pasv = 0
	return ftp.Response()
}

func (ftp *FTP) Retr(srcfile, dstfile string) error {
	dataconn, err := ftp.newdataconn()
	if err != nil {
		return err
	}
	ftp.Request("RETR " + srcfile)
	if ftp.Code != 150 {
		dataconn.Close()
		return errors.New(ftp.Message)
	}
	File, e := os.Create(dstfile)
	if e != nil {
		dataconn.Close()
		return e
	}
	io.Copy(File, dataconn)
	File.Close()
	dataconn.Close()
	ftp.pasv = 0
	return ftp.Response()
}

func (ftp *FTP) Quit() {
	ftp.Request("QUIT")
	if ftp.conn != nil {
		ftp.conn.Close()
	}
}
func (ftp *FTP) Connect(host string, port int) error {
	ftp.conn = nil
	addr := fmt.Sprintf("%s:%d", host, port)
	con, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	ftp.conn = con
	ftp.host = host
	ftp.port = port
	ftp.pasv = 0
	return ftp.Response()
}

func (ftp *FTP) Login(user, passwd string) {
	ftp.Request("USER " + user)
	ftp.Request("PASS " + passwd)
	ftp.user = user
	ftp.passwd = passwd
}

func (ftp *FTP) Response() error {
	ret := make([]byte, 1024)
	if ftp.conn == nil {

		return errors.New("no connection.")
	}
	n, err := ftp.conn.Read(ret)
	if err != nil {
		return err
	}
	msg := string(ret[:n])
	ftp.Code, _ = strconv.Atoi(msg[:3])
	ftp.Message = msg[4 : len(msg)-2]

	return nil
}

func (ftp *FTP) Request(cmd string) error {
	if ftp.conn == nil {

		return errors.New("no connection.")
	}
	_, err := ftp.conn.Write([]byte(cmd + "\r\n"))
	if err != nil {
		return err
	}
	ftp.cmd = cmd

	return ftp.Response()
}

func (ftp *FTP) Pasv() {
	ftp.Request("PASV")
	if ftp.Code == 227 {
		start, end := strings.Index(ftp.Message, "("), strings.Index(ftp.Message, ")")
		if start == -1 || end == -1 {

			ftp.pasv = 0
		}
		s := strings.Split(ftp.Message[start:end], ",")
		l1, _ := strconv.Atoi(s[len(s)-2])
		l2, _ := strconv.Atoi(s[len(s)-1])
		ftp.pasv = l1*256 + l2
	} else {
		ftp.pasv = 0
	}
}
