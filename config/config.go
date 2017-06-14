package config

import (
	"errors"
	"fmt"
	"github.com/ghodss/yaml"
	"io/ioutil"
	"reflect"
	"sync"
)

const (
	IntType    = "int"
	DoubleType = "double"
)

// Config contains configuration for the agent.
type Config struct {
	Daemon    *Daemon    `json:"daemon"`
	Metrics   *Metrics   `json:"metrics"`
	Endpoints []Endpoint `json:"endpoints"`
}

// Metrics contains the metric definitions that the agent expects to receive.
type Metrics struct {
	// The number of seconds that metrics should be aggregated prior to forwarding
	BufferSeconds int64

	// The list of reportable metrics
	Definitions []MetricDefinition `json:"definitions"`

	// Private cache of definitions by name for faster lookup.
	initOnce          sync.Once
	definitionsByName map[string]*MetricDefinition
}

// MetricDefinition describes a single reportable metric's name and type.
type MetricDefinition struct {
	Name string
	Type string
}

// Daemon contains configuration related to the daemon process (e.g., the port it listens on).
type Daemon struct {
	Port int32 `json:"port"`
}

// Endpoint describes a single remote endpoint used for sending aggregated metrics.
type Endpoint struct {
	Name           string                  `json:"name"`
	Disk           *DiskEndpoint           `json:"disk"`
	ServiceControl *ServiceControlEndpoint `json:"servicecontrol"`
	PubSub         *PubSubEndpoint         `json:"pubsub"`
}

type DiskEndpoint struct {
	Path          string `json:"path"`
	ExpireSeconds int64  `json:"expireSeconds"`
}

type ServiceControlEndpoint struct {
	KeyFile string `json:"keyfile"`
}

type PubSubEndpoint struct {
	KeyFile string `json:"keyfile"`
	Topic   string `json:"topic"`
}

func Load(path string) (*Config, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(data)
}

func Parse(data []byte) (*Config, error) {
	c := &Config{}
	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, err
	}
	return c, nil
}

// GetMetricDefinition returns the MetricDefinition with the given name, or nil if it does not
// exist.
func (c *Metrics) GetMetricDefinition(name string) *MetricDefinition {
	c.initOnce.Do(func() {
		c.definitionsByName = make(map[string]*MetricDefinition)
		for i := range c.Definitions {
			def := &c.Definitions[i]
			c.definitionsByName[def.Name] = def
		}
	})
	return c.definitionsByName[name]
}

// Validation

type Validatable interface {
	Validate() error
}

func (c *Config) Validate() error {
	if c.Daemon == nil {
		return errors.New("missing daemon section")
	}
	if err := c.Daemon.Validate(); err != nil {
		return err
	}
	if c.Metrics == nil {
		return errors.New("missing metrics section")
	}
	if err := c.Metrics.Validate(); err != nil {
		return err
	}
	if len(c.Endpoints) == 0 {
		return errors.New("no endpoints defined")
	}
	usedNames := make(map[string]bool)
	for _, e := range c.Endpoints {
		if usedNames[e.Name] {
			return errors.New(fmt.Sprintf("endpoint %v: multiple endpoints with the same name", e.Name))
		}
		if err := e.Validate(); err != nil {
			return err
		}
		usedNames[e.Name] = true
	}
	return nil
}

// Validate checks validity of metric configuration. Specifically, it must not contain duplicate
// metric definitions, and metric definitions must specify valid type names.
func (c *Metrics) Validate() error {
	usedNames := make(map[string]bool)
	for _, def := range c.Definitions {
		if usedNames[def.Name] {
			return errors.New(fmt.Sprintf("metric %v: duplicate name: %v", def.Name, def.Name))
		}
		usedNames[def.Name] = true

		if def.Type != IntType && def.Type != DoubleType {
			return errors.New(fmt.Sprintf("metric %s: invalid type: %v", def.Name, def.Type))
		}
	}
	return nil
}

func (d *Daemon) Validate() error {
	if d.Port <= 1024 || d.Port > 65535 {
		return errors.New("daemon: port must be greater than 1024 and less than 65536")
	}
	return nil
}

func (e *Endpoint) Validate() error {
	if e.Name == "" {
		return errors.New("endpoint: missing name")
	}
	// TODO(volkman): determine other Name requirements (no '/'?)

	types := 0
	for _, v := range []Validatable{e.Disk, e.PubSub, e.ServiceControl} {
		if reflect.ValueOf(v).IsNil() {
			continue
		}
		if err := v.Validate(); err != nil {
			return err
		}
		types++
	}

	if types == 0 {
		return errors.New(fmt.Sprintf("endpoint %v: missing type configuration", e.Name))
	}

	if types > 1 {
		return errors.New(fmt.Sprintf("endpoint %v: multiple type configurations", e.Name))
	}

	return nil
}

func (e *DiskEndpoint) Validate() error {
	if e.ExpireSeconds < 0 {
		return errors.New("disk: expireSeconds must not be negative")
	}
	if e.Path == "" {
		return errors.New("disk: missing path")
	}
	return nil
}

func (e *PubSubEndpoint) Validate() error {
	// TODO(volkman): implement
	return nil
}

func (e *ServiceControlEndpoint) Validate() error {
	// TODO(volkman): implement
	return nil
}