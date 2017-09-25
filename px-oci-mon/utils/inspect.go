package utils

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/mount"
)

// SimpleContainerConfig is a simplified container configuration, which includes arguments,
// Mounts (in Docker-CLI format) and environment variables.
type SimpleContainerConfig struct {
	Args   []string
	Mounts []string
	Env    []string
	Labels map[string]string
}

const cgroupFileName = "/proc/self/cgroup"

var (
	ErrContainerNotFound = errors.New("container not found")
	containerIDre        = regexp.MustCompilePOSIX(`[0-9]+:name=.*[/-]([0-9a-f]{64})`)
)

// GetMyContainerID extracts the Container ID from its cgroups entry.
func GetMyContainerID() (string, error) {
	f, err := os.Open(cgroupFileName)
	if err != nil {
		return "", fmt.Errorf("Unable to open %s: %s", cgroupFileName, err)
	}
	defer f.Close()

	content, err := ioutil.ReadAll(f)
	if err != nil {
		return "", fmt.Errorf("Unable to read %s: %s", cgroupFileName, err)
	}

	tripl := containerIDre.FindAllSubmatch(content, -1)
	if len(tripl) <= 0 || len(tripl[0]) != 2 {
		return "", ErrContainerNotFound
	}
	return string(tripl[0][1]), nil
}

// formatMounts is a helper-function which converts `types.MountPoint` structs into the Docker-CLI representation
// (ie. `source:dest[:shared,ro]`)
func formatMounts(mounts []types.MountPoint) []string {
	outList := make([]string, 0, 1)

	for _, m := range mounts {
		var out bytes.Buffer
		sep := ':'
		out.WriteString(m.Source)
		out.WriteRune(sep)
		out.WriteString(m.Destination)

		// fix up options
		switch m.Propagation {
		case mount.PropagationPrivate, mount.PropagationRPrivate:
			// ignore
		default:
			out.WriteRune(sep)
			out.WriteString(string(m.Propagation))
			sep = ','
		}
		if !m.RW {
			out.WriteRune(sep)
			out.WriteString("ro")
		}
		outList = append(outList, out.String())
	}
	return outList
}

