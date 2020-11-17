package serviced

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	log "github.com/sirupsen/logrus"
)

const (
	//StateRunning is the service running state
	StateRunning = 100
	//StateStopped is the service stopped state
	StateStopped = 300
)

//Running is running struct
type Running struct {
	State   int
	Cmd     *exec.Cmd
	Group   *Group
	Service *Service
	Err     error
	Waiter  sync.WaitGroup
}

//Manager is service manager
type Manager struct {
	Config
	TempDir string
	running map[string]*Running
	locker  sync.RWMutex
	console net.Listener
}

//NewManager will return new manager
func NewManager() (manager *Manager) {
	manager = &Manager{
		running: map[string]*Running{},
		locker:  sync.RWMutex{},
	}
	return
}

//Bootstrap will load configure and start console listener
func (m *Manager) Bootstrap() (err error) {
	log.Infof("bootstrap all service by config %v", m.Filename)
	err = m.Load()
	if err != nil {
		log.Errorf("load configure from %v fail with %v", m.Filename, err)
		return
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Errorf("start console listen fail with %v", err)
		return
	}
	addrFile := filepath.Join(m.TempDir, "console.serviced.txt")
	err = ioutil.WriteFile(addrFile, []byte(listener.Addr().String()), os.ModePerm)
	if err != nil {
		log.Errorf("write console listen to %v fail with %v", addrFile, err)
		listener.Close()
		return
	}
	log.Infof("starting console on %v, save to %v", listener.Addr(), addrFile)
	m.console = listener
	go m.procConsole(m.console)
	return
}

func (m *Manager) procConsole(listener net.Listener) (err error) {
	var raw net.Conn
	for {
		raw, err = listener.Accept()
		if err != nil {
			break
		}
		go m.procConn(raw)
	}
	log.Warnf("console is stopped by %v", err)
	return
}

func (m *Manager) procConn(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	for {
		cmd, err := reader.ReadBytes('\n')
		if err != nil {
			break
		}
		var parts []string
		err = json.Unmarshal(cmd, &parts)
		if err != nil || len(parts) < 2 {
			log.Warnf("parse client command fail with %v by %v", err, string(cmd))
			break
		}
		switch parts[0] {
		case "start":
			switch parts[1] {
			case "all":
				fmt.Fprintf(conn, "all service is starting\n")
				err = m.StartAll(conn)
			default:
				fmt.Fprintf(conn, "%v service is starting\n", parts[1])
				err = m.StartGroup(conn, parts[1])
			}
		case "stop":
			switch parts[1] {
			case "all":
				fmt.Fprintf(conn, "all service is stopping\n")
				err = m.StopAll()
			default:
				fmt.Fprintf(conn, "%v service is stopping\n", parts[1])
				err = m.StopGroup(parts[1])
			}
		case "add":
			var group Group
			group, err = m.Add(parts[1], 1)
			if err == nil {
				fmt.Fprintf(conn, "add group %v success with %v service\n", group.Name, len(group.Services))
			} else {
				fmt.Fprintf(conn, "add group %v fail with %v\n", parts[1], err)
			}
		case "remove":
			var group Group
			group, err = m.Remove(parts[1])
			if err == nil {
				fmt.Fprintf(conn, "remove group %v success with %v service\n", group.Name, len(group.Services))
			} else {
				fmt.Fprintf(conn, "remove group %v fail with %v\n", parts[1], err)
			}
		case "list":
			switch parts[1] {
			case "all":
				m.Print(conn, "*")
			default:
				m.Print(conn, parts[1])
			}

		}
		if err != nil {
			fmt.Fprintf(conn, "==ERR:%v\n", err)
		} else {
			fmt.Fprintf(conn, "==OK:\n")
		}
	}
}

//StopConsole will stop console listener
func (m *Manager) StopConsole() {
	m.console.Close()
	m.console = nil
}

//StartAll will start all service
func (m *Manager) StartAll(info io.Writer) (err error) {
	for _, group := range m.Groups {
		g := group
		startErr := m.startGroup(info, &g)
		if err == nil {
			err = startErr
		}
	}
	return
}

//StartGroup will start group by name
func (m *Manager) StartGroup(info io.Writer, name string) (err error) {
	group := m.Find(name)
	if group == nil {
		err = fmt.Errorf("group %v is not exist", name)
		return
	}
	err = m.startGroup(info, group)
	return
}

func (m *Manager) startGroup(info io.Writer, group *Group) (err error) {
	for _, service := range group.Services {
		s := service
		log.Infof("%v/%v is starting", group.Name, s.Name)
		fmt.Fprintf(info, "%v/%v is starting\n", group.Name, s.Name)
		err = m.StartService(group, &s)
		if err == nil {
			log.Infof("%v/%v is started", group.Name, s.Name)
			fmt.Fprintf(info, "%v/%v is started\n", group.Name, s.Name)
		} else {
			log.Infof("%v/%v is fail with %v", group.Name, s.Name, err)
			fmt.Fprintf(info, "%v/%v is fail with %v\n", group.Name, s.Name, err)
		}
	}
	if err != nil {
		err = fmt.Errorf("some service start fail")
	}
	return
}

//StartService will start one service
func (m *Manager) StartService(group *Group, service *Service) (err error) {
	key := fmt.Sprintf("%v/%v", group.Name, service.Name)
	m.locker.Lock()
	if m.running[key] != nil {
		err = fmt.Errorf("%v is running", key)
		m.locker.Unlock()
		return
	}
	m.locker.Unlock()
	confDir := filepath.Dir(group.Filename)
	values := map[string]interface{}{
		"CONF_DIR": confDir,
	}
	cmdDir := envReplaceEmpty(values, service.Dir, false)
	if !filepath.IsAbs(cmdDir) {
		cmdDir = filepath.Join(confDir, cmdDir)
	}
	cmdPath := envReplaceEmpty(values, service.Path, false)
	if !filepath.IsAbs(cmdPath) {
		cmdPath = filepath.Join(confDir, cmdPath)
	}
	cmdArgs := []string{}
	for _, arg := range service.Args {
		cmdArgs = append(cmdArgs, envReplaceEmpty(values, arg, false))
	}
	cmdEnv := []string{}
	for _, env := range service.Env {
		cmdEnv = append(cmdEnv, envReplaceEmpty(values, env, false))
	}
	var stdoutFile, stderrFile *os.File
	if len(service.Stdout) > 0 {
		stdout := envReplaceEmpty(values, service.Stdout, false)
		if !filepath.IsAbs(stdout) {
			stdout = filepath.Join(cmdDir, stdout)
		}
		stdoutFile, err = os.OpenFile(stdout, os.O_APPEND|os.O_CREATE|os.O_WRONLY, os.ModePerm)
		if err != nil {
			return err
		}
	}
	if len(service.Stderr) > 0 && service.Stderr == service.Stdout {
		stderrFile = stdoutFile
	} else if len(service.Stderr) > 0 {
		stderr := envReplaceEmpty(values, service.Stderr, false)
		if !filepath.IsAbs(stderr) {
			stderr = filepath.Join(cmdDir, stderr)
		}
		stderrFile, err = os.OpenFile(stderr, os.O_APPEND|os.O_CREATE|os.O_WRONLY, os.ModePerm)
		if err != nil {
			if stdoutFile != nil {
				stdoutFile.Close()
			}
			return err
		}
	}
	closeStd := func() {
		if stdoutFile != nil {
			stdoutFile.Close()
		}
		if stderrFile != nil && stderrFile != stdoutFile {
			stderrFile.Close()
		}
	}
	cmd := exec.Cmd{
		Path:   cmdPath,
		Args:   cmdArgs,
		Env:    cmdEnv,
		Dir:    cmdDir,
		Stdout: stdoutFile,
		Stderr: stderrFile,
	}
	running := &Running{
		Cmd:     &cmd,
		Group:   group,
		Service: service,
		Waiter:  sync.WaitGroup{},
	}
	err = cmd.Start()
	if err == nil {
		running.State = StateRunning
		running.Waiter.Add(1)
		m.locker.Lock()
		m.running[key] = running
		m.locker.Unlock()
		go func() {
			running.Err = cmd.Wait()
			log.Infof("%v/%v is stopped by %v", group.Name, service.Name, running.Err)
			running.State = StateStopped
			closeStd()
			m.locker.Lock()
			delete(m.running, key)
			m.locker.Unlock()
			running.Waiter.Done()
		}()
	} else {
		closeStd()
	}
	return
}

//StopAll will stop all service
func (m *Manager) StopAll() (err error) {
	m.StopGroup("*")
	return
}

//StopGroup will stop all service in group
func (m *Manager) StopGroup(group string) (err error) {
	stopping := []*Running{}
	m.locker.Lock()
	for _, running := range m.running {
		if group == "*" || running.Group.Name == group {
			stopping = append(stopping, running)
		}
	}
	m.locker.Unlock()
	for _, running := range stopping {
		log.Infof("%v/%v is stopping", running.Group.Name, running.Service.Name)
		err = m.StopService(running.Group.Name, running.Service.Name)
		log.Infof("%v/%v is stopped", running.Group.Name, running.Service.Name)
	}
	return
}

//StopService will stop single service in group
func (m *Manager) StopService(group, name string) (err error) {
	key := fmt.Sprintf("%v/%v", group, name)
	m.locker.Lock()
	running := m.running[key]
	if running == nil {
		err = fmt.Errorf("%v is not running", key)
		m.locker.Unlock()
		return
	}
	m.locker.Unlock()
	running.Cmd.Process.Kill()
	running.Waiter.Wait()
	return
}

//Print will show running
func (m *Manager) Print(info io.Writer, group string) {
	m.locker.Lock()
	defer m.locker.Unlock()
	fmt.Fprintf(info, "%v\t\t%v\t\t%v\t\t%v\t\t%v\n", "STATE", "NAME", "GROUP", "PATH", "DIR")
	for _, running := range m.running {
		if group != "*" && running.Group.Name != group {
			continue
		}
		fmt.Fprintf(info, "%v\t\t%v\t\t%v\t\t%v\t\t%v\n", "running", running.Service.Name, running.Group.Name, running.Service.Path, running.Cmd.Dir)
	}
	for _, g := range m.Groups {
		if group != "*" && g.Name != group {
			continue
		}
		for _, service := range g.Services {
			if _, ok := m.running[g.Name+"/"+service.Name]; ok {
				continue
			}
			dir := filepath.Dir(g.Filename)
			cmdDir := service.Dir
			if !filepath.IsAbs(cmdDir) {
				cmdDir = filepath.Join(dir, cmdDir)
			}
			fmt.Fprintf(info, "%v\t\t%v\t\t%v\t\t%v\t\t%v\n", "stopped", service.Name, g.Name, service.Path, cmdDir)
		}
	}
}
