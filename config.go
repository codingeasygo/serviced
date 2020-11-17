package serviced

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
)

func unmarshal(filename string, v interface{}) (err error) {
	jsonBytes, err := ioutil.ReadFile(filename)
	if err == nil {
		err = json.Unmarshal(jsonBytes, v)
	}
	return
}

func marshal(filename string, v interface{}) (err error) {
	jsonBytes, err := json.MarshalIndent(v, "", "  ")
	if err == nil {
		err = ioutil.WriteFile(filename, jsonBytes, os.ModePerm)
	}
	return
}

//Service is struct to record service configure
type Service struct {
	Name   string   `json:"name"`
	Path   string   `json:"path"`
	Args   []string `json:"args"`
	Env    []string `json:"env"`
	Stdout string   `json:"stdout"`
	Stderr string   `json:"stderr"`
	Dir    string   `json:"dir"`
}

//Group is struct to record the service group configure
type Group struct {
	Name     string    `json:"name"`
	Services []Service `json:"services"`
	Filename string    `json:"-"`
	Enable   int       `json:"-"`
}

//Config is current running configure
type Config struct {
	Filename string           `json:"-"`
	Includes map[string]int   `json:"includes"`
	Groups   map[string]Group `json:"-"`
}

func (c *Config) copy() (config *Config) {
	config = &Config{
		Filename: c.Filename,
		Includes: map[string]int{},
		Groups:   map[string]Group{},
	}
	for k, v := range c.Includes {
		config.Includes[k] = v
	}
	for k, v := range c.Groups {
		config.Groups[k] = v
	}
	return
}

func (c *Config) init() {
	if c.Includes == nil {
		c.Includes = map[string]int{}
	}
	if c.Groups == nil {
		c.Groups = map[string]Group{}
	}
}

//Load will return configure from data and parset to configure.
func (c *Config) Load() (err error) {
	c.init()
	err = unmarshal(c.Filename, c)
	if os.IsNotExist(err) {
		err = c.Save()
		return
	} else if err != nil {
		return
	}
	for file, enable := range c.Includes {
		group := &Group{}
		err = unmarshal(file, group)
		if err != nil {
			return
		}
		if len(group.Name) < 1 {
			log.Warnf("load group from %v fail with group name is empty", file)
			continue
		}
		if len(group.Services) < 1 {
			log.Warnf("load group from %v fail with service list is empty", file)
			continue
		}
		group.Filename = file
		group.Enable = enable
		c.Groups[group.Name] = *group
		log.Infof("load group from %v with %v service", file, len(group.Services))
	}
	return
}

//Save will save current configure to file
func (c *Config) Save() (err error) {
	c.init()
	log.Infof("save config to %v", c.Filename)
	err = marshal(c.Filename, c)
	if err == nil {
		log.Infof("save config to %v success", c.Filename)
	} else {
		log.Errorf("save config to %v fail with %v", c.Filename, err)
	}
	return
}

//Reload will reload group configure by name
func (c *Config) Reload(name string) (err error) {
	c.init()
	group, ok := c.Groups[name]
	if !ok {
		err = fmt.Errorf("%v is not exists", name)
		return
	}
	newGroup := &Group{}
	err = unmarshal(group.Filename, newGroup)
	if err == nil {
		newGroup.Filename = group.Filename
		newGroup.Enable = group.Enable
		c.Groups[name] = *newGroup
	}
	return
}

//Find will find the group by name
func (c *Config) Find(name string) (group *Group) {
	c.init()
	g, ok := c.Groups[name]
	if !ok {
		return nil
	}
	return &g
}

//Add will add group service
func (c *Config) Add(filename string, enable int) (group Group, err error) {
	c.init()
	filename, err = filepath.Abs(filename)
	if err != nil {
		return
	}
	group = Group{}
	err = unmarshal(filename, &group)
	if err != nil {
		return
	}
	group.Filename = filename
	group.Enable = enable
	old, ok := c.Groups[group.Name]
	if ok {
		err = fmt.Errorf("group %v is exists from %v", group.Name, old.Filename)
		return
	}
	if len(group.Services) < 1 {
		err = fmt.Errorf("group %v services is empty from %v", group.Name, group.Filename)
		return
	}
	for index, service := range group.Services {
		if len(service.Name) < 1 || len(service.Path) < 1 {
			err = fmt.Errorf("group %v %v service name/path is required", group.Name, index)
			return
		}
	}
	copy := c.copy()
	copy.Includes[filename] = enable
	err = copy.Save()
	if err == nil {
		c.Includes[filename] = enable
		c.Groups[group.Name] = group
	}
	return
}

//Remove will remote group service
func (c *Config) Remove(name string) (group Group, err error) {
	c.init()
	group, ok := c.Groups[name]
	if !ok {
		err = fmt.Errorf("group %v is not exists", name)
		return
	}
	copy := c.copy()
	delete(copy.Includes, group.Filename)
	err = copy.Save()
	if err == nil {
		c.Includes = copy.Includes
		delete(c.Groups, name)
	}
	return
}

// //AddService will add service
// func (c *Config) AddService(service Service) (err error) {
// 	if len(service.Name) < 1 || len(service.Start) < 1 {
// 		err = fmt.Errorf("name/service is required")
// 		return
// 	}
// 	if s, e := os.Stat(service.Start); e != nil || !s.IsDir() {
// 		err = fmt.Errorf("start %v is not exists or is folder", service.Start)
// 		return
// 	}
// 	having := c.FindService(service.Name)
// 	if having != nil {
// 		err = fmt.Errorf("%v is exists", service.Name)
// 		return
// 	}
// 	for _, after := range service.After {
// 		having = c.FindService(after)
// 		if having == nil {
// 			err = fmt.Errorf("%v is not exists", after)
// 			return
// 		}
// 	}
// 	config := *c //copy
// 	config.Services = append(config.Services, service)
// 	err = config.Save()
// 	if err == nil {
// 		c.Services = append(c.Services, service)
// 	}
// 	return
// }

// //UpdateService will update service
// func (c *Config) UpdateService(service Service) (err error) {
// 	if len(service.Name) < 1 || len(service.Start) < 1 {
// 		err = fmt.Errorf("name/service is required")
// 		return
// 	}
// 	if s, e := os.Stat(service.Start); e != nil || !s.IsDir() {
// 		err = fmt.Errorf("start %v is not exists or is folder", service.Start)
// 		return
// 	}
// 	having := c.FindService(service.Name)
// 	if having == nil {
// 		err = fmt.Errorf("%v is not exists", service.Name)
// 		return
// 	}

// 	return
// }
