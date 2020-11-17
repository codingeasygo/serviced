package serviced

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"path/filepath"
	"strings"
)

//Console is service manager cli
type Console struct {
	conn    net.Conn
	TempDir string
	Waiter  chan error
}

//NewConsole will return new console
func NewConsole() (console *Console) {
	console = &Console{
		Waiter: make(chan error, 8),
	}
	return
}

//Bootstrap will dial to console by console.serviced.txt file
func (c *Console) Bootstrap() (err error) {
	addrFile := filepath.Join(c.TempDir, "console.serviced.txt")
	addrBytes, err := ioutil.ReadFile(addrFile)
	if err != nil {
		err = fmt.Errorf("readd console address from %v fail with %v", addrFile, err)
		return
	}
	err = c.Dial(string(addrBytes))
	return
}

//Dial will connect the console
func (c *Console) Dial(remote string) (err error) {
	c.conn, err = net.Dial("tcp", remote)
	if err != nil {
		err = fmt.Errorf("connect console by %v fail with %v", remote, err)
		return
	}
	return
}

//Close will close the console connection
func (c *Console) Close() (err error) {
	err = c.conn.Close()
	return
}

//Add will add group service to manager
func (c *Console) Add(groupFile string) (err error) {
	_, err = fmt.Fprintf(c.conn, "%v\n", toJSON([]string{"add", groupFile}))
	if err == nil {
		err = <-c.Waiter
	}
	return
}

//Remove will add group service to manager
func (c *Console) Remove(group string) (err error) {
	_, err = fmt.Fprintf(c.conn, "%v\n", toJSON([]string{"remove", group}))
	if err == nil {
		err = <-c.Waiter
	}
	return
}

//Start will start all service in group
func (c *Console) Start(group string) (err error) {
	_, err = fmt.Fprintf(c.conn, "%v\n", toJSON([]string{"start", group}))
	if err == nil {
		err = <-c.Waiter
	}
	return
}

//Stop will stop all service in group
func (c *Console) Stop(group string) (err error) {
	_, err = fmt.Fprintf(c.conn, "%v\n", toJSON([]string{"stop", group}))
	if err == nil {
		err = <-c.Waiter
	}
	return
}

//List will list all service info in group
func (c *Console) List(group string) (err error) {
	_, err = fmt.Fprintf(c.conn, "%v\n", toJSON([]string{"list", group}))
	if err == nil {
		err = <-c.Waiter
	}
	return
}

//CopyTo will copy connection to writer
func (c *Console) CopyTo(out io.Writer) (err error) {
	var buffer []byte
	reader := bufio.NewReader(c.conn)
	for {
		buffer, err = reader.ReadBytes('\n')
		if err != nil {
			break
		}
		info := string(buffer)
		info = strings.TrimSpace(info)
		if strings.HasPrefix(info, "==ERR:") {
			c.Waiter <- fmt.Errorf("%v", strings.TrimPrefix(info, "==ERR:"))
		} else if strings.HasPrefix(info, "==OK:") {
			c.Waiter <- nil
		} else {
			fmt.Fprintf(out, "%v\n", info)
		}
	}
	return
}
