package utils

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/mount"
	"github.com/sirupsen/logrus"
)

const cgroupFileName = "/proc/self/cgroup"

var (
	// ErrContainerNotFound returned when no container can be found
	ErrContainerNotFound = errors.New("container not found")
	containerIDre        = regexp.MustCompilePOSIX(`[0-9]+:name=.*[/-]([0-9a-f]{64})`)
)

// SimpleContainerConfig is a simplified container configuration, which includes arguments,
// Mounts (in Docker-CLI format) and environment variables.
type SimpleContainerConfig struct {
	Args   []string
	Mounts []string
	Env    []string
	Labels map[string]string
}

type simpleOciProcess struct {
	Args []string `json:"args"`
	Env  []string `json:"env"`
	Data json.RawMessage
}
type simpleOciConfig struct {
	Process simpleOciProcess `json:"process"`
	Data    json.RawMessage
}

// ExtractEnvFromOciConfig extracts a given env variable out of the OCI's config.json
func ExtractEnvFromOciConfig(fname, envVar string) (string, error) {
	buf, err := ioutil.ReadFile(fname)
	if err != nil {
		return "", err
	}
	var c simpleOciConfig
	if err = json.Unmarshal(buf, &c); err != nil {
		return "", err
	}
	look4, found := envVar+"=", ""
	offs := len(look4)
	for _, e := range c.Process.Env {
		if strings.HasPrefix(e, look4) {
			found = e[offs:]
		}
	}
	return found, nil
}

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
func formatMounts(cconf types.ContainerJSON) []string {
	mounts := cconf.Mounts
	outList := make([]string, 0, 5)
	lookupCache := make(map[string]string)

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
			// NOTE: Docker 1.13.1 breaks their client API (see PWX-4278), so we have to handle "" as type.Propagation
			if prop := strings.TrimSpace(string(m.Propagation)); prop != "" {
				out.WriteRune(sep)
				out.WriteString(prop)
				sep = ','
			}
		}
		// add "ro" if required
		if !m.RW {
			out.WriteRune(sep)
			out.WriteString("ro")
		}

		// consult lookupCache for dupe/replace warnings
		newMount := out.String()
		if oldMount, has := lookupCache[m.Destination]; has {
			if oldMount == newMount {
				logrus.Warn("Duplicate mount-entry for '%s'", newMount)
			} else {
				logrus.Warn("Overriding mount-entry for '%s' - from %s to %s",
					m.Destination, oldMount, newMount)
			}
		}
		lookupCache[m.Destination] = newMount
		outList = append(outList, newMount)
	}

	// process extras: consult lookupCache for resolv.conf / hosts mounts - add if empty
	var extras = []struct {
		confSource  string
		destination string
		label       string
	}{
		{cconf.HostsPath, "/etc/hosts", "HostsPath"},
		{cconf.ResolvConfPath, "/etc/resolv.conf", "ResolvConfPath"},
	}
	for _, ex := range extras {
		if ex.confSource != "" {
			if old, has := lookupCache[ex.destination]; has {
				if !strings.HasPrefix(old, ex.confSource+":"+ex.destination) {
					// issue warning if already have a _different_ mount for the same dest (unlikely to happen)
					logrus.Warnf("Mount rule '%s' overrides %s='%s'", old, ex.label, ex.confSource)
				}
			} else {
				// no override via mounts, just add it as extra
				val := fmt.Sprintf("%s:%s:ro", ex.confSource, ex.destination)
				outList = append(outList, val)
			}
		}
	}

	return outList
}
