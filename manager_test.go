package serviced

import (
	"fmt"
	"net"
	"os"
	"testing"
	"time"
)

func TestManager(t *testing.T) {
	var err error
	m := NewManager()
	m.TempDir = os.TempDir()
	m.Filename = "test-config.json"
	err = m.Bootstrap()
	if err != nil {
		t.Error(err)
		return
	}
	c := NewConsole()
	c.TempDir = os.TempDir()
	err = c.Bootstrap()
	if err != nil {
		t.Error(err)
		return
	}
	go c.CopyTo(os.Stdout)
	//add
	c.Add("test-service.json")
	c.List("test")
	c.Add("test-service.json")
	//start
	c.Start("all")
	c.Start("test")
	c.Start("abc")
	//list
	c.List("all")
	c.List("test")
	c.List("abc")
	//stop
	c.Stop("all")
	c.Stop("test")
	//remove
	c.Remove("test")
	c.Remove("abc")
	//error
	fmt.Fprintf(c.conn, "%v\n", toJSON([]string{"remove"}))
	//
	time.Sleep(100 * time.Millisecond)
	c.Close()
	raw, _ := net.Dial("tcp", m.console.Addr().String())
	time.Sleep(10 * time.Millisecond)
	raw.Close()
	time.Sleep(300 * time.Millisecond)
	m.StopConsole()
	if m.StopService("test", "t0") == nil {
		t.Error("error")
		return
	}
}
