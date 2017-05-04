package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	dockerclient "github.com/fsouza/go-dockerclient"
	"os/exec"
)

func enableSharedMounts() error {
	cmd := exec.Command("nsenter", "--mount=/media/host/proc/1/ns/mnt", "--", "mount", "--make-shared", "/")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		fmt.Printf("Failed to enable shared mounts. Err: %v Stderr: %v\n", err, stderr)
		return err
	}

	fmt.Println("Enabled shared mounts succesfully")
	return nil
}

func upgrade(args []string) error {
	tag := "latest"
	image := "portworx/px-enterprise"

	docker, err := dockerclient.NewClient("unix:///var/run/docker.sock")
	if err != nil {
		fmt.Println("Could not connect to Docker... is Docker running on this host? ",
			err.Error())
		return err
	}

	err = docker.Ping()
	if err != nil {
		fmt.Println("Could not connect to Docker... is Docker running on this host? ",
			err.Error())
		return err
	}

	fmt.Println("Downloading Portworx...")

	origStdout := os.Stdout
	rPipe, wPipe, err := os.Pipe()
	if err != nil {
		fmt.Printf("Failed to get os Pipe. Err: %v", err)
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
		Repository:        image,
		Tag:               tag,
		OutputStream:      os.Stdout,
		InactivityTimeout: 180 * time.Second,
	}

	regUser := os.Getenv("REGISTRY_USER")
	regPass := os.Getenv("REGISTRY_PASS")
	if err = docker.PullImage(po, dockerclient.AuthConfiguration{
		Username: regUser,
		Password: regPass,
	}); err != nil {
		fmt.Println("Could not connect to Docker... is Docker running on this host?")
		return err
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
			"/var/lib/kubelet:/var/lib/kubelet:shared",
			"/var/cores:/var/cores",
			"/usr/src:/usr/src",
			"/lib/modules:/lib/modules",
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
		return err
	}

	err = docker.StartContainer(con.ID, &hostConfig)
	if err != nil {
		fmt.Println("Warning, could not start the Portworx container: ",
			err.Error())
		return err
	}

	fmt.Println("Install Done.  Portworx monitor running.")
	return nil
}

func main() {
	enableSharedMounts()

	err := upgrade(os.Args[1:])
	if err != nil {
		fmt.Println("Failed to start px container. Err: ", err)
		return
	}

	select {}
}
