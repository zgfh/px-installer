package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	dockerclient "github.com/fsouza/go-dockerclient"
)

func upgrade(args []string) error {
	tag := "latest"
	image := "portworx/px-enterprise"

	docker, err := dockerclient.NewClient("unix:///var/run/docker.sock")
	if err != nil {
		fmt.Println("Could not connect to Docker... is Docker running on this host? ",
			err.Error())
		return nil
	}

	err = docker.Ping()
	if err != nil {
		fmt.Println("Could not connect to Docker... is Docker running on this host? ",
			err.Error())
		return nil
	}

	fmt.Println("Downloading Portworx...")

	origStdout := os.Stdout
	rPipe, wPipe, err := os.Pipe()
	if err != nil {
		return err
	}

	os.Stdout = wPipe

	dpullOut := make(chan string)
	go func() {
		var bufout bytes.Buffer
		io.Copy(&bufout, rPipe)
		dpullOut <- bufout.String()
	}()

	po := dockerclient.PullImageOptions{
		Repository:   image,
		Tag:          tag,
		OutputStream: os.Stdout,
	}

	regUser := os.Getenv("REGISTRY_USER")
	regPass := os.Getenv("REGISTRY_PASS")
	if err = docker.PullImage(po, dockerclient.AuthConfiguration{
		Username: regUser,
		Password: regPass,
	}); err != nil {
		fmt.Println("Could not connect to Docker... is Docker running on this host?")
		return nil
	}

	// Back to normal state.
	wPipe.Close()
	os.Stdout = origStdout
	dOutput := <-dpullOut
	pline := ""
	for _, line := range strings.Split(dOutput, "\n") {
		if len(line) > 0 && len(pline) > 0 {
			fmt.Printf("\r%s", strings.Repeat(" ", len(pline)))
		}
		if len(line) > 0 {
			fmt.Printf("\r%s", line)
			time.Sleep(800 * time.Millisecond)
		}
		pline = line
	}
	fmt.Print("\n")

	fmt.Println("Starting Portworx...")

	hostConfig := dockerclient.HostConfig{
		RestartPolicy: dockerclient.RestartPolicy{
			Name:              "always",
			MaximumRetryCount: 0,
		},
		NetworkMode: "host",
		Privileged:  true,
		IpcMode:     "host",
		Binds: []string{
			"/run/docker/plugins:/run/docker/plugins",
			"/var/lib/osd:/var/lib/osd:shared",
			"/dev:/dev",
			"/etc/pwx:/etc/pwx",
			"/opt/pwx/bin:/export_bin:shared",
			"/var/run/docker.sock:/var/run/docker.sock",
			"/var/cores:/var/cores",
			"/usr/src:/usr/src",
		},
	}

	config := dockerclient.Config{
		Image: image + ":" + tag,
		Cmd:   args,
	}

	co := dockerclient.CreateContainerOptions{
		Name:       "portworx",
		Config:     &config,
		HostConfig: &hostConfig}
	con, err := docker.CreateContainer(co)
	if err != nil {
		fmt.Println("Warning, could not create the Portworx container: ",
			err.Error())
		return nil
	}

	err = docker.StartContainer(con.ID, &hostConfig)
	if err != nil {
		fmt.Println("Warning, could not start the Portworx container: ",
			err.Error())
		return nil
	}

	fmt.Println("Install Done.")
	return nil
}

func main() {
	upgrade(os.Args[1:])
}
