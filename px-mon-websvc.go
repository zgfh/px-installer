package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"text/template"
)

type Params struct {
	Etcd    string
	Cluster string
	DIface  string
	MIface  string
	Drive   string
}

func generate(etcd, cluster, dataIface, mgmtIface, drive string) string {

	cwd, _ := os.Getwd()
	p := filepath.Join(cwd, "k8s-px-spec-response.gtpl")

	t, err := template.ParseFiles(p)
	if err != nil {
		log.Println(err)
		return ""
	}

	params := Params{
		Cluster: cluster,
		Etcd:    etcd,
		DIface:  dataIface,
		MIface:  mgmtIface,
		Drive:   drive,
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
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

		etcd := r.URL.Query().Get("etcd")
		cluster := r.URL.Query().Get("cluster")
		dataIface := r.URL.Query().Get("diface")
		mgmtIface := r.URL.Query().Get("miface")
		drive := r.URL.Query().Get("drive")
		fmt.Fprintf(w, generate(etcd, cluster, dataIface, mgmtIface, drive))
	})

	log.Fatal(http.ListenAndServe(":8080", nil))
}
