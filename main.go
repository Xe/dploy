package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Xe/dploy/lib/backplane"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
)

var (
	appImage          = flag.String("image", "", "docker image to use for service")
	replicaCount      = flag.Uint64("replica-count", 1, "number of replicas to spawn")
	versionID         = flag.String("tag", "latest", "image tag to use (software version)")
	serviceName       = flag.String("service-name", "", "name of the service")
	backplaneToken    = flag.String("backplane-token", "", "backplane token, or BACKPLANE_TOKEN from env")
	endpoint          = flag.String("endpoint", "", "endpoint to route application traffic to")
	routeID           = flag.String("route", "", "existing route ID to shape to if it exists already")
	shapePause        = flag.Duration("shape-pause", 30*time.Second, "how long to wait between each step of backend shaping")
	dontCreateService = flag.Bool("dont-create-service", false, "don't create the service")
)

func main() {
	flag.Parse()

	if *backplaneToken == "" {
		*backplaneToken = os.Getenv("BACKPLANE_TOKEN")
	}

	bp, err := backplane.New(*backplaneToken)
	if err != nil {
		log.Fatal(err)
	}
	_ = bp

	defaultHeaders := map[string]string{"User-Agent": "dploy-devel"}
	cli, err := client.NewClient(client.DefaultDockerHost, client.DefaultVersion, nil, defaultHeaders)
	if err != nil {
		log.Fatal(err)
	}

	if !*dontCreateService {
		id, err := createService(cli, bp)
		if err != nil {
			log.Fatal(err)
		}

		log.Println("Created service " + id)
	} else {
		log.Println("Skipping service creation")
	}

	if *routeID == "" {
		*routeID, err = createRoute(cli, bp)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		log.Println("Skipping route creation, using " + *routeID)
	}

	log.Println("Waiting for services to be primed...")

	waitForContainers(bp)

	log.Printf("Service %s at version %s is now all ready for traffic", *serviceName, *versionID)

	log.Println("Performing 0 downtime shape (3 steps)")
	log.Println("In case of emergency, press ^C") // TODO(Xe): Implement this, have it roll back changes

	q, err := bp.Query()
	if err != nil {
		log.Fatal(err)
	}

	// Identify old routes
	var myEndpoint backplane.Endpoint
	for _, e := range q.Endpoints {
		if *endpoint == e.Pattern {
			myEndpoint = e
			break
		}
	}

	var oldRoute backplane.Route
	for _, route := range myEndpoint.Routes {
		if route.Weight == 100 {
			oldRoute = route
			break
		}
	}

	log.Println("Shaping 1/3 (75 old / 25 new)")

	err = shape(bp, oldRoute.ID, *routeID, 75, 25)
	if err != nil {
		log.Fatal(err)
	}

	time.Sleep(*shapePause)

	log.Println("Shaping 2/3 (50 old / 50 new)")

	err = shape(bp, oldRoute.ID, *routeID, 50, 50)
	if err != nil {
		log.Fatal(err)
	}

	time.Sleep(*shapePause)

	log.Println("Shaping 3/3 (25 old / 75 new)")

	err = shape(bp, oldRoute.ID, *routeID, 25, 75)
	if err != nil {
		log.Fatal(err)
	}

	time.Sleep(*shapePause)

	err = shape(bp, oldRoute.ID, *routeID, 0, 100)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("100% of traffic has been shaped over to " + *routeID)
	log.Println("Deploy complete")
}

/*
	usr, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}

	n, err := netrc.Parse(filepath.Join(usr.HomeDir, ".netrc"))
	if err != nil {
		t.Fatal(err)
	}

	req.SetBasicAauth(n.Machine(backplaneHost).Get("login"), "")
*/

func createService(c *client.Client, bp *backplane.Client) (string, error) {
	token, err := bp.GenToken()
	if err != nil {
		return "", err
	}

	svc := swarm.ServiceSpec{
		Annotations: swarm.Annotations{
			Name: *serviceName + "-" + *versionID,
			Labels: map[string]string{
				"Xe.dploy/service":  *serviceName,
				"Xe.dploy/endpoint": *endpoint,
				"Xe.dploy/version":  *versionID,
			},
		},

		TaskTemplate: swarm.TaskSpec{
			ContainerSpec: swarm.ContainerSpec{
				Image: *appImage,
				Env: []string{
					"BACKPLANE_TOKEN=" + token,
					"BACKPLANE_LABELS=" + fmt.Sprintf(
						"service:%s version:%s endpoint:%s",
						*serviceName, *versionID, *endpoint,
					),
				},
			},
		},

		Mode: swarm.ServiceMode{
			Replicated: &swarm.ReplicatedService{
				Replicas: replicaCount,
			},
		},
	}

	resp, err := c.ServiceCreate(context.Background(), svc, types.ServiceCreateOptions{})
	if err != nil {
		return "", err
	}

	return resp.ID, nil
}

func createRoute(c *client.Client, bp *backplane.Client) (string, error) {
	route, err := bp.Route(*endpoint, map[string]string{
		"service":  *serviceName,
		"endpoint": *endpoint,
		"version":  *versionID,
	})
	if err != nil {
		return "", err
	}

	log.Printf("Created %s for %s", route.ID, *endpoint)
	return route.ID, nil
}

func shape(bp *backplane.Client, oldroute, newroute string, oldweight, newweight int) error {
	err := bp.Shape(*endpoint, map[string]int{
		oldroute: oldweight,
		newroute: newweight,
	})
	if err != nil {
		return err
	}

	return nil
}

func waitForContainers(bp *backplane.Client) {
	t := time.Tick(time.Second)

outer:
	for {
		<-t

		q, err := bp.Query()
		if err != nil {
			log.Fatal(err)
		}

		for _, e := range q.Endpoints {
			if *endpoint != e.Pattern {
				continue
			}

			for _, route := range e.Routes {
				if route.ID == *routeID {
					if len(route.Backends) == int(*replicaCount) {
						break outer
					}
				}
			}
		}
	}
}
