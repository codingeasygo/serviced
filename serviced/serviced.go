package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/codingeasygo/serviced"

	log "github.com/sirupsen/logrus"
)

func usage() {
	switch runtime.GOOS {
	case "windows":
		fmt.Printf("Usage: serviced <install|uninstall|stat|stop|list|add|remove>\n")
		fmt.Printf("\tinstall\t\t install windows service\n")
		fmt.Printf("\tuninstall\t\t remove windows service\n")
	default:
		fmt.Printf("Usage: serviced <srv|stat|stop|list|add|remove>\n")
	}
	fmt.Printf("\tstart\t\t start group service\n")
	fmt.Printf("\tstop\t\t stop group service\n")
	fmt.Printf("\tlist\t\t list group service\n")
	fmt.Printf("\tadd\t\t add group service\n")
	fmt.Printf("\tremove\t\t remove group service\n")
	fmt.Printf("\n")
}

func main() {
	switch runtime.GOOS {
	case "windows":
		windowService()
	default:
		if (os.Args[1]) == "srv" {
			runService()
		} else {
			runConsole()
		}
	}
}

var service *serviced.Manager

func runService() {
	log.SetFormatter(NewPlainFormatter())
	path, _ := exePath()
	dir := filepath.Dir(path)
	service = serviced.NewManager()
	service.Filename = filepath.Join(dir, "serviced.json")
	switch runtime.GOOS {
	case "windows":
		service.TempDir = dir
	default:
		service.TempDir = os.TempDir()
	}
	err := service.Bootstrap()
	if err != nil {
		os.Exit(1)
		return
	}
	service.StartAll(ioutil.Discard)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	<-stop
	stopService()
}

func stopService() {
	service.StopAll()
	service.StopConsole()
}

func runConsole() {
	if len(os.Args) < 3 {
		usage()
		return
	}
	path, _ := exePath()
	dir := filepath.Dir(path)
	c := serviced.NewConsole()
	switch runtime.GOOS {
	case "windows":
		c.TempDir = dir
	default:
		c.TempDir = os.TempDir()
	}
	err := c.Bootstrap()
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
		return
	}
	go c.CopyTo(os.Stdout)
	defer c.Close()
	switch os.Args[1] {
	case "add":
		path, _ := filepath.Abs(os.Args[2])
		c.Add(path)
	case "remove":
		c.Remove(os.Args[2])
	case "start":
		c.Start(os.Args[2])
	case "stop":
		c.Stop(os.Args[2])
	case "list":
		c.List(os.Args[2])
	default:
		usage()
		os.Exit(1)
	}
}

func exePath() (string, error) {
	var err error
	prog := os.Args[0]
	p, err := filepath.Abs(prog)
	if err != nil {
		return "", err
	}
	fi, err := os.Stat(p)
	if err == nil {
		if !fi.Mode().IsDir() {
			return p, nil
		}
		err = fmt.Errorf("%s is directory", p)
	}
	if filepath.Ext(p) == "" {
		var fi os.FileInfo

		p += ".exe"
		fi, err = os.Stat(p)
		if err == nil {
			if !fi.Mode().IsDir() {
				return p, nil
			}
			err = fmt.Errorf("%s is directory", p)
		}
	}
	return "", err
}

//PlainFormatter is logrus formatter
type PlainFormatter struct {
	TimestampFormat string
	LevelDesc       []string
}

//NewPlainFormatter will create new formater
func NewPlainFormatter() (formatter *PlainFormatter) {
	formatter = &PlainFormatter{
		TimestampFormat: "2006-01-02 15:04:05",
		LevelDesc:       []string{"PANC", "FATL", "ERRO", "WARN", "INFO", "DEBG"},
	}
	return
}

//Format will format the log entry
func (f *PlainFormatter) Format(entry *log.Entry) ([]byte, error) {
	timestamp := fmt.Sprintf(entry.Time.Format(f.TimestampFormat))
	return []byte(fmt.Sprintf("%s %s %s\n", timestamp, f.LevelDesc[entry.Level], entry.Message)), nil
}
