package integration

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/ory/dockertest"
	"github.com/ory/dockertest/docker"
)

var (
	dockertestNewPool = newDefaultCreatePool
	once              sync.Once
	pool              dockertestPool
)

type dockertestPool interface {
	Purge(r *dockertest.Resource) error
	Retry(op func() error) error
	RunWithOptions(opts *dockertest.RunOptions, hcOpts ...func(*docker.HostConfig)) (*dockertest.Resource, error)
}

// DockerServiceInstance represents a running service running in docker.
type DockerServiceInstance struct {
	// stopAndCleanup is a function to call to stop and remove volumes of a service.
	stopAndCleanup func()
	// ContainerName is the name of the docker container running the service.
	ContainerName string
	// DockerHost is the (hostname + port) to use within docker to access the service.
	// it does not have a scheme
	DockerHost string
	// Host is the host (hostname + port) to use outside of docker to access the service,
	// it does not have a scheme
	Host string
}

type defaultDockerTestPool struct {
	*dockertest.Pool
}

func newDefaultCreatePool() (dockertestPool, error) {
	p, err := dockertest.NewPool("")
	return &defaultDockerTestPool{p}, err
}

func newDockerServiceInstance(res *dockertest.Resource, dockerHost, host string) *DockerServiceInstance {
	cleanup := func() {
		err := pool.Purge(res)
		if err != nil {
			log.Panicf("error purging container %s - %v", res.Container.Name, err)
		}
	}
	return &DockerServiceInstance{
		stopAndCleanup: cleanup,
		ContainerName:  res.Container.Name,
		DockerHost:     dockerHost,
		Host:           host,
	}
}

func createPool() {
	var err error
	pool, err = dockertestNewPool()
	if err != nil {
		log.Panicf("error connecting to docker - %v", err)
	}
}

// DockerService contains the various settings required to create a new docker service.
type DockerService struct {
	// DockerHostname determines how to access services from code. Empty means the code runs within docker.
	// when using docker-for-mac, "localhost" would be used when running outside of docker.
	DockerHostname string
	Image          string
	Version        string
	PublishedPort  string
	ContainerPort  string
	Env            []string
	Cmd            []string
	Entrypoint     []string
	HealthCheck    func(*DockerServiceInstance) error
	Instance       *DockerServiceInstance
}

// NewMongoService returns a mongo service.
func NewMongoService(withinDocker bool) *DockerService {
	dockerHostname := ""
	if !withinDocker {
		dockerHostname = "localhost"
	}
	return &DockerService{
		DockerHostname: dockerHostname,
		Image:          "mongo",
		Version:        "4.2",
		PublishedPort:  "27017",
		ContainerPort:  "27017",
		Env:            []string{},
		Cmd:            []string{},
		HealthCheck: func(svc *DockerServiceInstance) error {
			healthHost := svc.Host
			if withinDocker {
				healthHost = svc.DockerHost
			}

			conn, err := net.DialTimeout("tcp", healthHost, 10*time.Second)
			if conn != nil {
				_ = conn.Close()
			}
			return err
		},
	}
}

// Start starts the instance of the service
func (svc *DockerService) Start() (*DockerServiceInstance, error) {
	if svc.Instance != nil {
		return nil, fmt.Errorf("ignoring Start of %s, instance already started", svc.Image)
	}

	once.Do(createPool)
	if svc.Version == "" {
		svc.Version = "latest"
	}

	portBindings := map[docker.Port][]docker.PortBinding{
		docker.Port(fmt.Sprintf("%s/tcp", svc.ContainerPort)): {
			{HostPort: svc.PublishedPort},
		},
	}

	runOptions := &dockertest.RunOptions{
		Repository:   svc.Image,
		Tag:          svc.Version,
		PortBindings: portBindings,
		Env:          svc.Env,
		Cmd:          svc.Cmd,
	}
	if svc.Entrypoint != nil {
		runOptions.Entrypoint = svc.Entrypoint
	}
	// Pull the image, create a container based on the image, and run the container.
	resource, err := pool.RunWithOptions(runOptions)
	if err != nil {
		return nil, fmt.Errorf("error running collections container, %v", err)
	}

	hostname := resource.Container.NetworkSettings.IPAddress
	port := resource.GetPort(fmt.Sprintf("%s/tcp", svc.ContainerPort))
	inDockerAddr := net.JoinHostPort(hostname, port)
	addr := ""
	if svc.DockerHostname != "" {
		addr = net.JoinHostPort(svc.DockerHostname, port)
	}

	svc.Instance = newDockerServiceInstance(resource, inDockerAddr, addr)

	// exponential backoff-retry until service is ready to accept connections.
	err = pool.Retry(func() error {
		return svc.HealthCheck(svc.Instance)
	})
	if err != nil {
		return nil, fmt.Errorf("healthcheck failed for %s. %v", svc.Image, err)
	}

	return svc.Instance, nil
}

// Stop stops the currently running instance of the service
func (svc *DockerService) Stop() {
	if svc.Instance == nil {
		return
	}
	svc.Instance.stopAndCleanup()
	svc.Instance = nil
}
