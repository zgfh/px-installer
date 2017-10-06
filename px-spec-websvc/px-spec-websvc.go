package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
)

const (
	currentPxImage           = "portworx/px-enterprise:1.2.10"
	currentPxLighthouseImage = "portworx/px-lighthouse:1.1.9"
)

var k8sVersionRegex, _ = regexp.Compile("v?(\\d+)\\.(\\d+)\\.(\\d+)")

type Params struct {
	Kvdb             string
	Cluster          string
	DIface           string
	MIface           string
	Drives           string
	EtcdPasswd       string
	EtcdCa           string
	EtcdCert         string
	EtcdKey          string
	Acltoken         string
	Token            string
	Env              string
	Coreos           string
	Openshift        string
	PxImage          string
	MasterLess       bool
	K8s8AndAbove     bool
	SecretType       string
	ClusterSecretKey string
}

type LighthouseParams struct {
	Kvdb            string
	EtcdPasswd      string
	EtcdCa          string
	EtcdCert        string
	EtcdKey         string
	EtcdAuth        string
	AdminEmail      string
	Company         string
	LighthouseImage string
}

func getK8sEnvParam(env string) string {
	if len(env) != 0 {
		env = strings.Trim(env, " ")
		if len(env) != 0 {
			var envParam = "env:\n"
			for _, e := range strings.Split(env, ",") {
				entry := strings.SplitN(e, "=", 2)
				if len(entry) == 2 {
					key := entry[0]
					val := entry[1]
					envParam = envParam + "           - name: " + key + "\n"
					envParam = envParam + "             value: " + val + "\n"
				}
			}
			env = envParam
		}
	}
	return env
}

func generate(templateFile, kvdb, cluster, dataIface, mgmtIface, drives, force, etcdPasswd, etcdCa, etcdCert, etcdKey,
	acltoken, token, env, coreos, openshift, pximage, master, k8sVersion, secretType, clusterSecretKey string) string {
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

	if pximage == "" {
		pximage = currentPxImage
	}

	masterless := true
	if len(master) != 0 && master == "true" {
		masterless = false
	}

	env = getK8sEnvParam(env)

	k8s8AndAbove := false
	if matches := k8sVersionRegex.FindStringSubmatch(k8sVersion); len(matches) == 4 {
		if matches[1] == "1" && matches[2] == "8" {
			k8s8AndAbove = true
		}
	}

	params := Params{
		Cluster:          cluster,
		Kvdb:             kvdb,
		DIface:           dataIface,
		MIface:           mgmtIface,
		Drives:           drives,
		EtcdPasswd:       etcdPasswd,
		EtcdCa:           etcdCa,
		EtcdCert:         etcdCert,
		EtcdKey:          etcdKey,
		Acltoken:         acltoken,
		Token:            token,
		Env:              env,
		Coreos:           coreos,
		Openshift:        openshift,
		PxImage:          pximage,
		MasterLess:       masterless,
		K8s8AndAbove:     k8s8AndAbove,
		SecretType:       secretType,
		ClusterSecretKey: clusterSecretKey,
	}

	var result bytes.Buffer
	err = t.Execute(&result, params)
	if err != nil {
		log.Println(err)
	}

	s := result.String()

	return s
}

func generateForLighthouse(w http.ResponseWriter, r *http.Request) {
	kvdb := r.URL.Query().Get("kvdb")
	etcdPasswd := r.URL.Query().Get("etcdPasswd")
	etcdCa := r.URL.Query().Get("etcdCa")
	etcdCert := r.URL.Query().Get("etcdCert")
	etcdKey := r.URL.Query().Get("etcdKey")
	etcdAuth := r.URL.Query().Get("etcdAuth")
	adminEmail := r.URL.Query().Get("adminEmail")
	company := r.URL.Query().Get("company")
	lighthouseImage := r.URL.Query().Get("lighthouseImage")

	templateFile := "k8s-portworx-lighthouse.gtpl"
	cwd, _ := os.Getwd()
	p := filepath.Join(cwd, templateFile)

	t, err := template.ParseFiles(p)
	if err != nil {
		log.Println(err)
		fmt.Fprintf(w, "Failed to parse file: %v", err)
		return
	}

	if lighthouseImage == "" {
		lighthouseImage = currentPxLighthouseImage
	}

	addEnvParam := func(key, value string) string {
		if value == "" {
			return ""
		}
		str := "- name: " + key + "\n"
		str = str + "          value: \"" + value + "\""
		return str
	}
	etcdPasswd = addEnvParam("PWX_KVDB_USER_PWD", etcdPasswd)
	etcdCa = addEnvParam("PWX_KVDB_CA_PATH", etcdCa)
	etcdCert = addEnvParam("PWX_KVDB_USER_CERT_PATH", etcdCert)
	etcdKey = addEnvParam("PWX_KVDB_USER_CERT_KEY_PATH", etcdKey)
	etcdAuth = addEnvParam("PWX_KVDB_AUTH", etcdAuth)
	if company == "" {
		company = "Portworx"
	}
	company = addEnvParam("PWX_PX_COMPANY_NAME", company)
	adminEmail = addEnvParam("PWX_PX_ADMIN_EMAIL", adminEmail)

	params := LighthouseParams{
		Kvdb:            kvdb,
		EtcdPasswd:      etcdPasswd,
		EtcdCa:          etcdCa,
		EtcdCert:        etcdCert,
		EtcdKey:         etcdKey,
		AdminEmail:      adminEmail,
		Company:         company,
		LighthouseImage: lighthouseImage,
		EtcdAuth:        etcdAuth,
	}

	var result bytes.Buffer
	err = t.Execute(&result, params)
	if err != nil {
		log.Println(err)
		fmt.Fprintf(w, "Unable to parse query params: %v", err)
	}

	s := result.String()
	fmt.Fprintf(w, s)
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
				drives, force, etcdPasswd, etcdCa, etcdCert, etcdKey, acltoken, token, env, coreos, openshift,
				pximage, "", "", "", ""))
		} else {
			fmt.Fprintf(w, generate("k8s-flexvol-pxd-spec-response.gtpl", kvdb, cluster, dataIface, mgmtIface,
				drives, force, etcdPasswd, etcdCa, etcdCert, etcdKey, acltoken, token, env, coreos, openshift,
				pximage, "", "", "", ""))
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
		master := r.URL.Query().Get("master")
		k8sVersion := r.URL.Query().Get("k8sVersion")
		secretType := r.URL.Query().Get("secretType")
		clusterSecretKey := r.URL.Query().Get("clusterSecretKey")

		if len(zeroStorage) != 0 && (len(master) == 0 || master == "true") {
			fmt.Fprintf(w, generate("k8s-master-worker-response.gtpl", kvdb, cluster, dataIface, mgmtIface,
				drives, force, etcdPasswd, etcdCa, etcdCert, etcdKey, acltoken, token, env, coreos, "",
				pximage, master, k8sVersion, secretType, clusterSecretKey))
		} else {
			fmt.Fprintf(w, generate("k8s-pxd-spec-response.gtpl", kvdb, cluster, dataIface, mgmtIface,
				drives, force, etcdPasswd, etcdCa, etcdCert, etcdKey, acltoken, token, env, coreos, "",
				pximage, master, k8sVersion, secretType, clusterSecretKey))
		}
	})

	http.HandleFunc("/lighthouse", generateForLighthouse)

	log.Fatal(http.ListenAndServe(":8080", nil))
}
