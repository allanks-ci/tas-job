package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/boltdb/bolt"
	"github.com/gorilla/mux"
)

type Job struct {
	Id          int    `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type TenantInfo struct {
	ShortCode string `json:"shortCode"`
}

var fatalLog = log.New(os.Stdout, "FATAL: ", log.LstdFlags)
var infoLog = log.New(os.Stdout, "INFO: ", log.LstdFlags)

var db *bolt.DB

func getTenantBucket(tenant string) []byte {
	return []byte(fmt.Sprintf("%s-Jobs", tenant))
}

func newTenant(rw http.ResponseWriter, req *http.Request) {
	decoder := json.NewDecoder(req.Body)
	var info TenantInfo
	infoLog.Printf("NewTenant json error: %v", decoder.Decode(&info))
	infoLog.Printf("NewTenant bolt error: %v", db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucket(getTenantBucket(info.ShortCode))
		return err
	}))
}

func deleteTenant(rw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	infoLog.Printf("DeleteTenant bolt error: %v", db.Update(func(tx *bolt.Tx) error {
		err := tx.DeleteBucket(getTenantBucket(vars["tenant"]))
		return err
	}))
}

func create(rw http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodPost {
		updateJob(rw, req)
	} else {
		vars := mux.Vars(req)
		t, err := template.ParseFiles("static/create.html")
		infoLog.Printf("Create template error: %v", err)
		if vars["job"] == "0" {
			t.Execute(rw, Job{})
		} else {
			jid, err := strconv.Atoi(vars["job"])
			infoLog.Printf("UpdateJob strconv error: %v", err)
			decoder := json.NewDecoder(getJobFromBolt(jid, req.Header.Get("tazzy-tenant")))
			var job Job
			infoLog.Printf("UpdateJob json error: %v", decoder.Decode(job))
			t.Execute(rw, job)
		}
	}
}

func updateJob(rw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	err := req.ParseForm()
	if err != nil {
		return
	}
	tenant := req.Header.Get("tazzy-tenant")
	jid, err := strconv.Atoi(vars["job"])
	infoLog.Printf("UpdateJob strconv error: %v", err)
	job := Job{
		Id:          jid,
		Title:       req.FormValue("Title"),
		Description: req.FormValue("Description"),
	}
	infoLog.Printf("UpdateJob bolt error: %v", db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(getTenantBucket(tenant))

		// Check if this is a new job
		if job.Id == 0 {
			id, _ := b.NextSequence()
			job.Id = int(id)
		}
		data, err := json.Marshal(&job)
		if err == nil {
			return b.Put(itob(job.Id), data)
		} else {
			return err
		}
	}))
	http.Redirect(rw, req, fmt.Sprintf("/job/%v", job.Id), 301)
	// postHTTP(tenant, getURL("devs/tas/jobSets/uploads"), getJobList(tenant))
}

func itob(v int) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(v))
	return b
}

func remove(rw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	jid, err := strconv.Atoi(vars["job"])
	infoLog.Printf("UpdateJob strconv error: %v", err)
	infoLog.Printf("Remove bolt error: %v", db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(getTenantBucket(req.Header.Get("tazzy-tenant")))
		return b.Delete(itob(jid))
	}))
	http.Redirect(rw, req, "/", 301)
}

func getJobs(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Set("Content-Type", "application/json")
	rw.Write(getJobList(req.Header.Get("tazzy-tenant")).Bytes())
}

func getJobList(tenant string) *bytes.Buffer {
	buffer := bytes.NewBuffer([]byte{})
	db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(getTenantBucket(tenant)).Cursor()
		buffer.WriteString("[")
		k, v := c.First()
		if k != nil {
			buffer.Write(v)
			for k, v := c.Next(); k != nil; k, v = c.Next() {
				buffer.WriteString(",")
				buffer.Write(v)
			}
		}
		buffer.WriteString("]")
		return nil
	})
	return buffer
}

func getJobById(rw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	jid, err := strconv.Atoi(vars["job"])
	infoLog.Printf("GetJobById strconv error: %v", err)
	rw.Write(getJobFromBolt(jid, req.Header.Get("tazzy-tenant")).Bytes())
}

func getJobFromBolt(jobId int, tenant string) *bytes.Buffer {
	buffer := bytes.NewBuffer([]byte{})
	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(getTenantBucket(tenant))
		buffer.Write(b.Get(itob(jobId)))
		return nil
	})
	return buffer
}

func basePage(rw http.ResponseWriter, req *http.Request) {
	buf := getJobList(req.Header.Get("tazzy-tenant"))
	var jobs []Job
	decoder := json.NewDecoder(buf)
	infoLog.Printf("BasePage json error: %v", decoder.Decode(&jobs))
	t, err := template.ParseFiles("static/index.html")
	infoLog.Printf("BasePage template error: %v", err)
	if jobs == nil {
		t.Execute(rw, []Job{})
	} else {
		t.Execute(rw, jobs)
	}
}

func main() {
	var err error
	db, err = bolt.Open("/db/tas-job.db", 0644, nil)
	if err != nil {
		fatalLog.Fatal(err)
	}
	defer db.Close()

	r := mux.NewRouter()
	r.HandleFunc("/", basePage)
	r.HandleFunc("/job/{job}", create)
	r.HandleFunc("/remove/{job}", remove)
	r.HandleFunc("/tas/core/tenants", newTenant)
	r.HandleFunc("/tas/core/tenants/{tenant}", deleteTenant)
	r.HandleFunc("/tas/devs/tas/jobs", getJobs)
	r.HandleFunc("/tas/devs/tas/jobs/byID/{job}", getJobById)
	r.PathPrefix("/").Handler(http.FileServer(http.Dir("./static/")))
	fatalLog.Fatal(http.ListenAndServe(":8080", r))
}

func postHTTP(tenant, url string, data io.Reader) ([]byte, error) {
	req, _ := http.NewRequest("POST", url, data)
	req.Header.Set("Content-Type", "application/json")
	return doHTTP(req, tenant)
}

func doHTTP(req *http.Request, tenant string) ([]byte, error) {
	req.Header.Set("tazzy-secret", os.Getenv("IO_TAZZY_SECRET"))
	req.Header.Set("tazzy-tenant", os.Getenv("APP_SHORTCODE"))
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}

func getURL(api string) string {
	return fmt.Sprintf("%s/%s", os.Getenv("IO_TAZZY_URL"), api)
}
