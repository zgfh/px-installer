package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

const (
	currentPxImage = "portworx/px-enterprise:1.2.8"
)

type Params struct {
	Kvdb       string
	Cluster    string
	DIface     string
	MIface     string
	Drives     string
	EtcdPasswd string
	EtcdCa     string
	EtcdCert   string
	EtcdKey    string
	Acltoken   string
	Token      string
	Env        string
	Coreos     string
	Openshift  string
	PxImage    string
}

func generate(templateFile, kvdb, cluster, dataIface, mgmtIface, drives, force, etcdPasswd,
	etcdCa, etcdCert, etcdKey, acltoken, token, env, coreos, openshift, pximage string) string {

	cwd, _ := os.Getwd()
	p := filepath.Join(cwd, templateFile)

	t, err := template.ParseFiles(p)
	if err != nil {
		log.Println(err)
		return ""
	}

	drives = strings.Trim(drives, " ")
	if len(drives) != 0 {
		var drivesParam string
		for _, d := range strings.Split(drives, ",") {
			drivesParam = drivesParam + " -s " + d
		}
		drives = drivesParam
	} else {
		if len(force) != 0 {
			drives = "-A -f"
		} else {
			drives = "-a -f"
		}
	}

	if len(env) != 0 {
		env = strings.Trim(env, " ")
		if len(env) != 0 {
			var envParam = "env:\n"
			for _, e := range strings.Split(env, ",") {
				entry := strings.SplitN(e, "=", 2)
				if len (entry) == 2 {
					key := entry[0]
					val := entry[1]
					envParam = envParam + "           - name: " + key + "\n"
					envParam = envParam + "             value: " + val + "\n"
				}
			}
			env = envParam
		}
	}

	if pximage == "" {
		pximage = currentPxImage
	}
	params := Params{
		Cluster:    cluster,
		Kvdb:       kvdb,
		DIface:     dataIface,
		MIface:     mgmtIface,
		Drives:     drives,
		EtcdPasswd: etcdPasswd,
		EtcdCa:     etcdCa,
		EtcdCert:   etcdCert,
		EtcdKey:    etcdKey,
		Acltoken:   acltoken,
		Token:      token,
		Env:        env,
		Coreos:     coreos,
		Openshift:  openshift,
		PxImage: pximage,
	}

	var result bytes.Buffer
	err = t.Execute(&result, params)
	if err != nil {
		log.Println(err)
	}

	s := result.String()

	return s
}

func main() {
	http.HandleFunc("/kube1.5", func(w http.ResponseWriter, r *http.Request) {
		kvdb := r.URL.Query().Get("kvdb")
		cluster := r.URL.Query().Get("cluster")
		dataIface := r.URL.Query().Get("diface")
		mgmtIface := r.URL.Query().Get("miface")
		drives := r.URL.Query().Get("drives")
		zeroStorage := r.URL.Query().Get("zeroStorage")
		force := r.URL.Query().Get("force")
		etcdPasswd := r.URL.Query().Get("etcdPasswd")
		etcdCa := r.URL.Query().Get("etcdCa")
		etcdCert := r.URL.Query().Get("etcdCert")
		etcdKey := r.URL.Query().Get("etcdKey")
		acltoken := r.URL.Query().Get("acltoken")
		token := r.URL.Query().Get("token")
		env := r.URL.Query().Get("env")
		coreos := r.URL.Query().Get("coreos")
		openshift := r.URL.Query().Get("openshift")
		pximage := r.URL.Query().Get("pximage")

		if len(zeroStorage) != 0 {
			fmt.Fprintf(w, generate("k8s-flexvol-master-worker-response.gtpl", kvdb, cluster, dataIface, mgmtIface,
				drives, force, etcdPasswd, etcdCa, etcdCert, etcdKey, acltoken, token, env, coreos, openshift, pximage))
		} else {
			fmt.Fprintf(w, generate("k8s-flexvol-pxd-spec-response.gtpl", kvdb, cluster, dataIface, mgmtIface,
				drives, force, etcdPasswd, etcdCa, etcdCert, etcdKey, acltoken, token, env, coreos, openshift, pximage))
		}
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		kvdb := r.URL.Query().Get("kvdb")
		cluster := r.URL.Query().Get("cluster")
		dataIface := r.URL.Query().Get("diface")
		mgmtIface := r.URL.Query().Get("miface")
		drives := r.URL.Query().Get("drives")
		zeroStorage := r.URL.Query().Get("zeroStorage")
		force := r.URL.Query().Get("force")
		etcdPasswd := r.URL.Query().Get("etcdPasswd")
		etcdCa := r.URL.Query().Get("etcdCa")
		etcdCert := r.URL.Query().Get("etcdCert")
		etcdKey := r.URL.Query().Get("etcdKey")
		acltoken := r.URL.Query().Get("acltoken")
		token := r.URL.Query().Get("token")
		env := r.URL.Query().Get("env")
		coreos := r.URL.Query().Get("coreos")
		pximage := r.URL.Query().Get("pximage")

		if len(zeroStorage) != 0 {
			fmt.Fprintf(w, generate("k8s-master-worker-response.gtpl", kvdb, cluster, dataIface, mgmtIface,
				drives, force, etcdPasswd, etcdCa, etcdCert, etcdKey, acltoken, token, env, coreos, "", pximage))
		} else {
			fmt.Fprintf(w, generate("k8s-pxd-spec-response.gtpl", kvdb, cluster, dataIface, mgmtIface,
				drives, force, etcdPasswd, etcdCa, etcdCert, etcdKey, acltoken, token, env, coreos, "", pximage))
		}
	})

	log.Fatal(http.ListenAndServe(":8080", nil))
}
