package utils

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetMyContainerID(t *testing.T) {
	proc_self_cgroup_docker_v12 := `10:blkio:/system.slice/docker-d70eeb70bebdfa02dcbbda0e9aee666f38c0cba2e5664f1bc4eb0eb560932e7a.scope
9:freezer:/system.slice/docker-d70eeb70bebdfa02dcbbda0e9aee666f38c0cba2e5664f1bc4eb0eb560932e7a.scope
8:memory:/system.slice/docker-d70eeb70bebdfa02dcbbda0e9aee666f38c0cba2e5664f1bc4eb0eb560932e7a.scope
7:net_cls:/system.slice/docker-d70eeb70bebdfa02dcbbda0e9aee666f38c0cba2e5664f1bc4eb0eb560932e7a.scope
6:devices:/system.slice/docker-d70eeb70bebdfa02dcbbda0e9aee666f38c0cba2e5664f1bc4eb0eb560932e7a.scope
5:cpuset:/system.slice/docker-d70eeb70bebdfa02dcbbda0e9aee666f38c0cba2e5664f1bc4eb0eb560932e7a.scope
4:hugetlb:/system.slice/docker-d70eeb70bebdfa02dcbbda0e9aee666f38c0cba2e5664f1bc4eb0eb560932e7a.scope
3:perf_event:/system.slice/docker-d70eeb70bebdfa02dcbbda0e9aee666f38c0cba2e5664f1bc4eb0eb560932e7a.scope
2:cpuacct,cpu:/system.slice/docker-d70eeb70bebdfa02dcbbda0e9aee666f38c0cba2e5664f1bc4eb0eb560932e7a.scope
1:name=systemd:/system.slice/docker-d70eeb70bebdfa02dcbbda0e9aee666f38c0cba2e5664f1bc4eb0eb560932e7a.scope`

	tripl := containerIDre.FindAllStringSubmatch(proc_self_cgroup_docker_v12, -1)
	assert.Equal(t, 1, len(tripl))
	assert.Equal(t, 2, len(tripl[0]))
	assert.Equal(t, "d70eeb70bebdfa02dcbbda0e9aee666f38c0cba2e5664f1bc4eb0eb560932e7a", tripl[0][1])

	proc_self_cgroup_docker_v17_06_2_ce := `11:blkio:/kubepods/besteffort/pod002e671e-8c78-11e7-b787-525400ada096/d1b8179dbb75053ffcdc2a066f46f73d16dd5cce75a41c22f6363ba8102aa667
10:devices:/kubepods/besteffort/pod002e671e-8c78-11e7-b787-525400ada096/d1b8179dbb75053ffcdc2a066f46f73d16dd5cce75a41c22f6363ba8102aa667
9:cpuacct,cpu:/kubepods/besteffort/pod002e671e-8c78-11e7-b787-525400ada096/d1b8179dbb75053ffcdc2a066f46f73d16dd5cce75a41c22f6363ba8102aa667
8:net_prio,net_cls:/kubepods/besteffort/pod002e671e-8c78-11e7-b787-525400ada096/d1b8179dbb75053ffcdc2a066f46f73d16dd5cce75a41c22f6363ba8102aa667
7:freezer:/kubepods/besteffort/pod002e671e-8c78-11e7-b787-525400ada096/d1b8179dbb75053ffcdc2a066f46f73d16dd5cce75a41c22f6363ba8102aa667
6:pids:/kubepods/besteffort/pod002e671e-8c78-11e7-b787-525400ada096/d1b8179dbb75053ffcdc2a066f46f73d16dd5cce75a41c22f6363ba8102aa667
5:perf_event:/kubepods/besteffort/pod002e671e-8c78-11e7-b787-525400ada096/d1b8179dbb75053ffcdc2a066f46f73d16dd5cce75a41c22f6363ba8102aa667
4:cpuset:/kubepods/besteffort/pod002e671e-8c78-11e7-b787-525400ada096/d1b8179dbb75053ffcdc2a066f46f73d16dd5cce75a41c22f6363ba8102aa667
3:hugetlb:/kubepods/besteffort/pod002e671e-8c78-11e7-b787-525400ada096/d1b8179dbb75053ffcdc2a066f46f73d16dd5cce75a41c22f6363ba8102aa667
2:memory:/kubepods/besteffort/pod002e671e-8c78-11e7-b787-525400ada096/d1b8179dbb75053ffcdc2a066f46f73d16dd5cce75a41c22f6363ba8102aa667
1:name=systemd:/kubepods/besteffort/pod002e671e-8c78-11e7-b787-525400ada096/d1b8179dbb75053ffcdc2a066f46f73d16dd5cce75a41c22f6363ba8102aa667`

	tripl = containerIDre.FindAllStringSubmatch(proc_self_cgroup_docker_v17_06_2_ce, -1)
	assert.Equal(t, 1, len(tripl))
	assert.Equal(t, 2, len(tripl[0]))
	assert.Equal(t, "d1b8179dbb75053ffcdc2a066f46f73d16dd5cce75a41c22f6363ba8102aa667", tripl[0][1])

}
