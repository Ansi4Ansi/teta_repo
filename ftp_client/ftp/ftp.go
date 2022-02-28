package ftp

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"regexp"
	"strconv"
	"strings"
)

type Connection struct {
	conn         net.Conn
	//logger       Logger
	transferType transferType
}

// Logger can be used to log the raw messages on the FTP control connection.
//type Logger interface {
	// SentFTP is called after a message is sent to the FTP server on the control
	// connection. If an error occurred during the sent it is given to the Logger
	// as well.
	//SentFTP(msg []byte, err error)
	// ReceivedFTP is called after a message is received from FTP server on the control
	// connection. If an error occurred while receiving it is given to the Logger as well.
	//ReceivedFTP(response []byte, err error)
//}

// Connect establishes a connection to the given host on the given port.
// The standard FTP port is 21.
func Connect(host string, port uint16) (*Connection, error) {
	return ConnectLogging(host, port, nil)
}

// ConnectLogging establishes a connection to the given host on the given port.
// All messages sent and reveived over the control connection are additionally
// passed to the given Logger.
// The standard FTP port is 21.
func ConnectLogging(host string, port uint16) (*Connection, error) {
	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	return newConnection(conn)
}

// ConnectOn uses the given connection as an FTP control connection. This can be
// used for setting connection parameters like time-outs.
func ConnectOn(conn net.Conn) (*Connection, error) {
	return newConnection(conn, nil)
}

// ConnectLoggingOn uses the given connection as an FTP control connection. This
// can be used for setting connection parameters like time-outs. It also sets
// the logger.
func ConnectLoggingOn(conn net.Conn (*Connection, error) {
	return newConnection(conn, logger)
}

type transferType string

const (
	transferASCII  transferType = "ASCII"
	transferBinary              = "binary"
)

func newConnection(conn net.Conn) (*Connection, error) {
	c := &Connection{conn, logger, transferASCII}
	resp, code, err := c.receive()
	if err != nil {
		return nil, err
	}
	if code != serviceReadyForNewUser {
		return nil, errorMessage("connect", resp)
	}
	return c, nil
}

func errorMessage(command string, response []byte) error {
	return errors.New("FTP server responded to " + command +
		" with error: " + string(response))
}

// Close closes the underlying TCP connection to the FTP server. Call this
// function when done. Closing does not send a QUIT message to the server
// so make sure to do that before-hand.
func (c *Connection) Close() {
	c.conn.Close()
}

func (c *Connection) send(words ...string) error {
	msg := strings.Join(words, " ") + "\r\n"
	_, err := c.conn.Write([]byte(msg))
	if c.logger != nil {
		c.logger.SentFTP([]byte(msg), err)
	}
	return err
}

func (c *Connection) sendWithoutEmptyString(cmd, arg string) error {
	if arg == "" {
		return c.send(cmd)
	}
	return c.send(cmd, arg)
}

// if the returned error is not nil then the response and the code are not meaningful
func (c *Connection) receive() (response []byte, code responseCode, e error) {
	msg, err := readResponse(c.conn)
	if c.logger != nil {
		c.logger.ReceivedFTP(msg, err)
	}
	return msg, extractCode(msg), err
}

func readResponse(conn net.Conn) ([]byte, error) {
	all := new(bytes.Buffer)
	buffer := make([]byte, 1024)
	done := false
	var err error
	for !done {
		done, err = readResponseInto(conn, buffer, all)
	}
	return all.Bytes(), err
}

func readResponseInto(con net.Conn, buf []byte, dest *bytes.Buffer) (done bool, err error) {
	n, err := con.Read(buf)
	if err != nil {
		return true, err
	}
	dest.Write(buf[:n])
	if isCompleteResponse(dest.Bytes()) {
		return true, nil
	}
	return false, nil
}

func isCompleteResponse(msg []byte) bool {
	return isCompleteSingleLineResponse(msg) || isCompleteMultiLineResponse(msg)
}

func isCompleteSingleLineResponse(msg []byte) bool {
	return isSingleLineResponse(msg) && endsInNewLine(msg)
}

func isSingleLineResponse(msg []byte) bool {
	return len(msg) >= 4 && msg[3] == ' '
}

func endsInNewLine(msg []byte) bool {
	l := len(msg)
	if l < 2 {
		return false
	}
	return msg[l-2] == '\r' && msg[l-1] == '\n'
}

func isCompleteMultiLineResponse(msg []byte) bool {
	return isMultiLineResponse(msg) && lastLineEndsInSameCodeAsFirstLine(msg)
}

func isMultiLineResponse(msg []byte) bool {
	return len(msg) >= 4 && msg[3] == '-'
}

func lastLineEndsInSameCodeAsFirstLine(msg []byte) bool {
	// msg should end in \r\n so splitting at \r\n creates an empty string
	// at the end. This makes the actual (non-empty) last line of the msg
	// the second last split part.
	lines := strings.Split(string(msg), "\r\n")
	if len(lines) < 3 {
		return false
	}
	first := lines[0]
	last := lines[len(lines)-2]
	if len(first) < 3 || len(last) < 4 {
		return false
	}
	codePlusSpace := first[:3] + " "
	return last[:4] == codePlusSpace
}

func extractCode(msg []byte) responseCode {
	if len(msg) <= 3 {
		return responseCode(msg)
	}
	return responseCode(msg[:3])
}


func (c *Connection) Login(user, password string) error {
	err := c.send("USER", user)
	if err != nil {
		return err
	}
	resp, code, err := c.receive()
	if err != nil {
		return err
	}
	if code == userLoggedIn_Proceed {
		return nil
	}
	if code == userNameOK_NeedPassword {
		return c.execute(userLoggedIn_Proceed, "PASS", password)
	}
	return errorMessage("USER", resp)
}

func (c *Connection) execute(success responseCode, args ...string) error {
	_, err := c.executeGetResponse(success, args...)
	return err
}

func (c *Connection) executeGetResponse(expectedCode responseCode, args ...string) ([]byte, error) {
	err := c.send(args...)
	if err != nil {
		return nil, err
	}
	resp, code, err := c.receive()
	if err != nil {
		return nil, err
	}
	if code == expectedCode {
		return resp, nil
	}
	return nil, errorMessage(args[0], resp)
}


func (c *Connection) ChangeWorkingDirTo(path string) error {
	return c.execute(fileActionCompleted, "CWD", path)
}

func (c *Connection) Quit() error {
	return c.execute(serviceClosingControlConnection, "QUIT")
}

func (c *Connection) Delete(path string) error {
	return c.execute(fileActionCompleted, "DELE", path)
}

func (c *Connection) MakeDirectory(path string) (string, error) {
	resp, err := c.executeGetResponse(pathNameCreated, "MKD", path)
	if err != nil {
		return "", err
	}
	return getPathFromResponse(resp)
}

func (c *Connection) RemoveDirectory(path string) error {
	return c.execute(fileActionCompleted, "RMD", path)
}

func (c *Connection) NoOperation() error {
	return c.execute(commandOk, "NOOP")
}

func removeControlSymbols(resp []byte) string {
	noCodeOrNewLine := strings.TrimSuffix(string(resp[4:]), "\r\n")
	if isSingleLineResponse(resp) {
		return noCodeOrNewLine
	}
	lastLineStart := strings.LastIndex(noCodeOrNewLine, "\r\n")
	start := noCodeOrNewLine[:lastLineStart+2]
	end := noCodeOrNewLine[lastLineStart+6:]
	all := start + end
	return strings.TrimSuffix(all, "\r\n")
}

func (c *Connection) Status() (StatusType, string, error) {
	return c.StatusOf("")
}

func (c *Connection) StatusOf(path string) (StatusType, string, error) {
	err := c.sendWithoutEmptyString("STAT", path)
	if err != nil {
		return "", "", err
	}
	resp, code, err := c.receive()
	if err != nil {
		return "", "", err
	}
	if typ, ok := statusTypeOfCode(code); ok {
		return typ, removeControlSymbols(resp), nil
	}
	return "", "", errorMessage("STAT", resp)
}

func statusTypeOfCode(code responseCode) (typ StatusType, ok bool) {
	if code == systemStatusOrHelpReply {
		return GeneralStatus, true
	}
	if code == directoryStatus {
		return DirectoryStatus, true
	}
	if code == fileStatus {
		return FileStatus, true
	}
	return "", false
}

type StatusType string

const (
	GeneralStatus   StatusType = "status"
	FileStatus                 = "file status"
	DirectoryStatus            = "directory status"
)

func (c *Connection) System() (string, error) {
	resp, err := c.executeGetResponse(systemName, "SYST")
	if err != nil {
		return "", err
	}
	return removeControlSymbols(resp), nil
}

func (c *Connection) PrintWorkingDirectory() (string, error) {
	resp, err := c.executeGetResponse(pathNameCreated, "PWD")
	if err != nil {
		return "", err
	}
	return getPathFromResponse(resp)
}

func (c *Connection) Abort() error {
	resp, code, err := c.sendAndReceive("ABOR")
	if err != nil {
		return err
	}
	if code == noTransferInProgress || code == closingDataConnection {
		return nil
	}
	if code == connectionClosed_TransferAborter {
		resp, code, err = c.receive()
		if err != nil {
			return err
		}
		if code == closingDataConnection {
			return nil
		}
		return errorMessage("ABOR", resp)
	}
	return errorMessage("ABOR", resp)
}

func (c *Connection) sendAndReceive(words ...string) ([]byte, responseCode, error) {
	err := c.send(words...)
	if err != nil {
		return nil, "", err
	}
	return c.receive()
}

var pathMatcher = regexp.MustCompile("[0-9][0-9][0-9][ |-]\"(.+)\".*\r\n")

func getPathFromResponse(resp []byte) (string, error) {
	if !pathMatcher.Match(resp) {
		return "", errorMessage("path extraction", resp)
	}
	matches := pathMatcher.FindSubmatch(resp)
	return string(matches[1]), nil
}

func (c *Connection) setASCIITransfer() error {
	return c.setTransferTypeTo(transferASCII, "A")
}

func (c *Connection) setBinaryTransfer() error {
	return c.setTransferTypeTo(transferBinary, "I")
}

// the symbol A is for ASCII and I is for binary data
func (c *Connection) setTransferTypeTo(t transferType, symbol string) error {
	if c.transferType == t {
		return nil
	}
	err := c.execute(commandOk, "TYPE", symbol)
	if err == nil {
		c.transferType = t
	}
	return err
}

func parseNLST(data string) []string {
	onlyNewLines := strings.Replace(data, "\r\n", "\n", -1)
	lines := strings.Split(onlyNewLines, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func (c *Connection) readListCommandData(cmd, path string) (string, error) {
	err := c.setASCIITransfer()
	if err != nil {
		return "", err
	}
	dataConn, err := c.enterPassiveMode()
	if err != nil {
		return "", err
	}
	defer dataConn.Close()
	err = c.sendWithoutEmptyString(cmd, path)
	if err != nil {
		return "", err
	}
	resp, code, err := c.receive()
	if err != nil {
		return "", err
	}
	if !code.ok() {
		return "", errorMessage(cmd, resp)
	}
	data, err := ioutil.ReadAll(dataConn)
	if err != nil {
		return "", err
	}
	resp, code, err = c.receive()
	if err != nil {
		return "", err
	}
	if !code.ok() {
		return "", errorMessage(cmd, resp)
	}
	return string(data), nil
}

func (c *Connection) enterPassiveMode() (net.Conn, error) {
	resp, err := c.executeGetResponse(enteringPassiveMode, "PASV")
	if err != nil {
		return nil, err
	}
	addr, err := getAddressOfPasvResponse(resp)
	if err != nil {
		return nil, err
	}
	return net.Dial("tcp", addr)
}

var addrMatcher = regexp.MustCompile(
	".*\\(([0-9]+,[0-9]+,[0-9]+,[0-9]+),([0-9]+),([0-9]+)\\).*")

func getAddressOfPasvResponse(msg []byte) (string, error) {
	if !addrMatcher.Match(msg) {
		return "", errorMessage("address extraction", msg)
	}
	matches := addrMatcher.FindSubmatch(msg)
	ip := strings.Replace(string(matches[1]), ",", ".", -1)
	highPort, _ := strconv.Atoi(string(matches[2]))
	lowPort, _ := strconv.Atoi(string(matches[3]))
	port := strconv.Itoa(256*highPort + lowPort)
	return ip + ":" + port, nil
}

func (c *Connection) Download(path string, dest io.Writer) error {
	err := c.setBinaryTransfer()
	if err != nil {
		return err
	}
	dataConn, err := c.enterPassiveMode()
	if err != nil {
		return err
	}
	err = c.send("RETR", path)
	if err != nil {
		dataConn.Close()
		return err
	}
	resp, code, err := c.receive()
	if err != nil {
		dataConn.Close()
		return err
	}
	if !code.ok() {
		dataConn.Close()
		return errorMessage("RETR", resp)
	}
	_, err = io.Copy(dest, dataConn)
	if err != nil {
		dataConn.Close()
		return err
	}
	err = dataConn.Close()
	if err != nil {
		return err
	}
	resp, code, err = c.receive()
	if err != nil {
		return err
	}
	if !code.ok() {
		return errorMessage("RETR", resp)
	}
	return nil
}

func (c *Connection) Upload(source io.Reader, path string) error {
	return c.upload("STOR", path, source)
}

func (c *Connection) upload(cmd, path string, source io.Reader) error {
	err := c.setBinaryTransfer()
	if err != nil {
		return err
	}
	dataConn, err := c.enterPassiveMode()
	if err != nil {
		return err
	}
	err = c.sendWithoutEmptyString(cmd, path)
	if err != nil {
		dataConn.Close()
		return err
	}
	resp, code, err := c.receive()
	if err != nil {
		dataConn.Close()
		return err
	}
	if !code.ok() {
		dataConn.Close()
		return errorMessage(cmd, resp)
	}
	_, err = io.Copy(dataConn, source)
	if err != nil {
		dataConn.Close()
		return err
	}
	err = dataConn.Close()
	if err != nil {
		return err
	}
	resp, code, err = c.receive()
	if err != nil {
		return err
	}
	if !code.ok() {
		return errorMessage(cmd, resp)
	}
	return nil
}
