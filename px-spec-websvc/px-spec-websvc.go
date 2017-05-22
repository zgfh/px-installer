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
}

func generate(templateFile, kvdb, cluster, dataIface, mgmtIface, drives, zeroStorage, force, etcdPasswd,
	etcdCa, etcdCert, etcdKey, acltoken, token, env string) string {

	cwd, _ := os.Getwd()
	p := filepath.Join(cwd, templateFile)

	t, err := template.ParseFiles(p)
	if err != nil {
		log.Println(err)
		return ""
	}

	if len(zeroStorage) != 0 && zeroStorage == "true" {
		drives = "-z"
	} else {
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
	}

	if len(env) != 0 {
		env = strings.Trim(env, " ")
		if len(env) != 0 {
			var envParam string
			for _, e := range strings.Split(env, ",") {
				envParam = envParam + " -e " + e
			}
			env = envParam
		}
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
	http.HandleFunc("/swarm", func(w http.ResponseWriter, r *http.Request) {
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

		fmt.Fprintf(w, generate("swarm-px-service-spec-response.gtpl", kvdb, cluster, dataIface,
			mgmtIface, drives, zeroStorage, force, etcdPasswd, etcdCa, etcdCert, etcdKey, acltoken, token,
			env))
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

		fmt.Fprintf(w, generate("k8s-pxd-spec-response.gtpl", kvdb, cluster, dataIface, mgmtIface,
			drives, zeroStorage, force, etcdPasswd, etcdCa, etcdCert, etcdKey, acltoken, token, env))
	})

	log.Fatal(http.ListenAndServe(":8080", nil))
}
