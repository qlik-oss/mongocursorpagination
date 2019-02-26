package integration

import (
	"flag"
	"fmt"
	"log"
	"os"
	"testing"
)

var (
	dockerFlag  = flag.Bool("docker", false, "Set to true to use the Docker container's IP address. Set to false to use localhost.")
	purgePolicy = flag.String("purge-policy", "always", "Define when to purge test containers. (always, onsuccess, never)")
	mongoSvc    *DockerService
)

func startMongo() {
	instance, err := mongoSvc.Start()
	if err != nil {
		log.Fatalf("error starting mongo: %v", err)
	}

	if *dockerFlag {
		err = os.Setenv("MONGO_URI", "mongodb://"+instance.DockerHost)
	} else {
		err = os.Setenv("MONGO_URI", "mongodb://"+instance.Host)
	}
	if err != nil {
		log.Fatalf("error setting MONGO_URI env vars: %v", err)
	}
}

func stopMongo() {
	mongoSvc.Stop()
}

func TestMain(m *testing.M) {
	flag.Parse()

	mongoSvc = NewMongoService(*dockerFlag)
	startMongo()

	code := m.Run()

	if *purgePolicy == "always" || (*purgePolicy == "onsuccess" && code == 0) {
		stopMongo()
	} else {
		fmt.Println("=== Not Purging Containers ===")
		fmt.Printf("docker rm -fv %s\n", mongoSvc.Instance.ContainerName)
		fmt.Println("==============================")
	}

	os.Exit(code) // Note that os.Exit ignores deferred statements
}
